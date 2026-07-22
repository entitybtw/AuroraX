package responsecache

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/cespare/xxhash/v2"
	"github.com/labstack/echo/v5"

	"aurora/configuration"
	"aurora/internal/audit_logging"
	"aurora/internal/authorization_scope"
	"aurora/internal/core"
	"aurora/internal/embedding"
)

type semanticCacheMiddleware struct {
	embedder         embedding.Embedder
	store            VecStore
	cfg              config.SemanticCacheConfig
	embedderIdentity string
	wg               sync.WaitGroup
	hitRecorder      func(*echo.Context, []byte, string)
}

func newSemanticCacheMiddleware(emb embedding.Embedder, store VecStore, cfg config.SemanticCacheConfig, hitRecorder func(*echo.Context, []byte, string)) *semanticCacheMiddleware {
	return &semanticCacheMiddleware{
		embedder:         emb,
		store:            store,
		cfg:              cfg,
		embedderIdentity: emb.Identity(),
		hitRecorder:      hitRecorder,
	}
}

func (m *semanticCacheMiddleware) Handle(c *echo.Context, body []byte, next func() error) error {
	if m == nil || m.store == nil {
		return next()
	}

	path := c.Request().URL.Path
	if !cacheablePaths[path] || c.Request().Method != http.MethodPost {
		return next()
	}

	if shouldSkipSemanticCache(c.Request()) {
		return next()
	}

	ctx := c.Request().Context()
	plan := core.GetWorkflow(ctx)

	embedText, msgCount := extractEmbedText(body, m.cfg.ExcludeSystemPrompt)
	if embedText == "" {
		return next()
	}

	threshold := m.cfg.SimilarityThreshold
	if v := headerFloat64(c.Request(), "X-Cache-Semantic-Threshold"); v > 0 {
		threshold = v
	}
	promptThreshold := m.cfg.PromptSimilarityThreshold
	if promptThreshold <= 0 {
		promptThreshold = defaultPromptSimilarityThreshold
	}
	if v := headerFloat64(c.Request(), "X-Cache-Prompt-Similarity"); v > 0 {
		promptThreshold = v
	}
	semTTL := resolveSemanticTTL(c.Request(), m.cfg)

	if m.cfg.MaxConversationMessages != nil && *m.cfg.MaxConversationMessages > 0 && msgCount > *m.cfg.MaxConversationMessages {
		return next()
	}

	msgFp, fpOK := conversationInvariantFingerprint(body, m.cfg.ExcludeSystemPrompt)
	if !fpOK {
		return next()
	}
	baseParams := computeParamsHashForContext(ctx, body, path, plan, core.GetGuardrailsHash(ctx), m.embedderIdentity)
	paramsHash := sha256HexOf(baseParams + "\x00" + msgFp)

	vec, err := m.embedder.Embed(ctx, embedText)
	if err != nil {
		slog.Warn("semantic cache: embed failed, bypassing", "err", err)
		return next()
	}

	results, err := m.store.Search(ctx, vec, paramsHash, 1)
	if err != nil {
		attachDebugHeaders(c, debugMeta{Layer: "none", SemanticParamsKey: paramsHash, SemanticThreshold: threshold, MissReason: "search_failed"})
		slog.Warn("semantic cache: search failed, bypassing", "err", err)
		return next()
	}

	semanticCacheKey := sha256HexOf(embedText + "\x00" + paramsHash)
	var promptScore float64
	if len(results) > 0 {
		promptScore = promptTextSimilarity(embedText, results[0].PromptText)
	}
	if len(results) > 0 && float64(results[0].Score) >= threshold && promptScore >= promptThreshold {
		attachDebugHeaders(c, debugMeta{Layer: CacheTypeSemantic, SemanticKey: semanticCacheKey, SemanticParamsKey: paramsHash, SemanticThreshold: threshold, SemanticScore: results[0].Score, PromptSimilarityThreshold: promptThreshold, PromptSimilarityScore: promptScore})
		replayErr := writeCachedResponse(c, path, body, results[0].Response, CacheTypeSemantic)
		if replayErr == nil {
			auditlog.EnrichEntryWithCacheType(c, CacheTypeSemantic)
			if m.hitRecorder != nil {
				m.hitRecorder(c, results[0].Response, CacheTypeSemantic)
			}
			slog.Info("semantic cache hit",
				"path", path,
				"score", results[0].Score,
				"request_id", c.Request().Header.Get("X-Request-ID"),
			)
			return nil
		}
		slog.Warn("semantic cache replay failed", "path", path, "err", replayErr)
	}
	missReason := "not_found"
	if len(results) > 0 {
		missReason = "below_threshold"
		if float64(results[0].Score) >= threshold && promptScore < promptThreshold {
			missReason = "prompt_changed"
		}
	}
	attachDebugHeaders(c, debugMeta{Layer: "none", SemanticKey: semanticCacheKey, SemanticParamsKey: paramsHash, SemanticThreshold: threshold, MissReason: missReason, PromptSimilarityThreshold: promptThreshold, PromptSimilarityScore: promptScore})

	data, ok, err := captureResponseForCache(
		c,
		path,
		"semantic cache: failed to capture cacheable response body",
		next,
	)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	ttl := time.Duration(semTTL) * time.Second
	if v := headerDuration(c.Request(), "X-Cache-TTL"); v > 0 {
		ttl = v
	}

	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		storeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := m.store.Insert(storeCtx, semanticCacheKey, vec, data, paramsHash, embedText, ttl); err != nil {
			slog.Warn("semantic cache: store failed", "key", semanticCacheKey, "err", err)
		}
	}()

	return nil
}

func (m *semanticCacheMiddleware) close() error {
	m.wg.Wait()
	if m.embedder != nil {
		_ = m.embedder.Close()
	}
	if m.store != nil {
		return m.store.Close()
	}
	return nil
}

type embedMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

func extractEmbedText(body []byte, excludeSystem bool) (text string, nonSystemCount int) {
	var envelope struct {
		Messages []embedMessage  `json:"messages"`
		Input    json.RawMessage `json:"input"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return "", 0
	}
	if len(envelope.Messages) > 0 {
		return extractText(envelope.Messages, excludeSystem)
	}
	if len(envelope.Input) == 0 {
		return "", 0
	}
	var s string
	if json.Unmarshal(envelope.Input, &s) == nil {
		if s == "" {
			return "", 0
		}
		return s, 1
	}
	var inputMsgs []embedMessage
	if json.Unmarshal(envelope.Input, &inputMsgs) != nil || len(inputMsgs) == 0 {
		return "", 0
	}
	return extractText(inputMsgs, excludeSystem)
}

func extractText(messages []embedMessage, excludeSystem bool) (text string, count int) {
	var lastUser string
	for _, m := range messages {
		if m.Role == "system" && excludeSystem {
			continue
		}
		count++
		if m.Role == "user" {
			lastUser = extractContent(m.Content)
		}
	}
	return lastUser, count
}

func conversationInvariantFingerprint(body []byte, excludeSystem bool) (fingerprint string, ok bool) {
	var envelope struct {
		Messages []json.RawMessage `json:"messages"`
		Input    json.RawMessage   `json:"input"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return "", false
	}
	msgs, fpOK := resolveMessages(envelope.Messages, envelope.Input)
	if !fpOK {
		return "", false
	}
	if len(msgs) == 0 {
		return "", true
	}

	type msgPart struct {
		role        string
		content     json.RawMessage
		unparseable bool
		rawMsg      json.RawMessage
	}
	var included []msgPart
	for _, rawMsg := range msgs {
		var m struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		}
		if err := json.Unmarshal(rawMsg, &m); err != nil {
			var roleOnly struct {
				Role string `json:"role"`
			}
			_ = json.Unmarshal(rawMsg, &roleOnly)
			if roleOnly.Role == "system" && excludeSystem {
				continue
			}
			included = append(included, msgPart{role: roleOnly.Role, unparseable: true, rawMsg: rawMsg})
			continue
		}
		if m.Role == "system" && excludeSystem {
			continue
		}
		included = append(included, msgPart{role: m.Role, content: m.Content})
	}

	lastUser := -1
	for i := len(included) - 1; i >= 0; i-- {
		if included[i].role == "user" {
			lastUser = i
			break
		}
	}

	h := sha256.New()
	for i, p := range included {
		h.Write([]byte(p.role))
		h.Write([]byte{0})
		if p.unparseable {
			sum := sha256.Sum256(p.rawMsg)
			h.Write(sum[:])
			h.Write([]byte{0})
			continue
		}
		if i == lastUser && lastUser >= 0 {
			hashNonTextContent(h, p.content)
		} else if len(p.content) > 0 {
			h.Write(p.content)
		}
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil)), true
}

func resolveMessages(messages []json.RawMessage, input json.RawMessage) (msgs []json.RawMessage, ok bool) {
	if len(messages) > 0 {
		return messages, true
	}
	if len(input) == 0 {
		return nil, false
	}
	var s string
	if json.Unmarshal(input, &s) == nil {
		return nil, true
	}
	var arr []json.RawMessage
	if json.Unmarshal(input, &arr) != nil {
		return nil, false
	}
	if len(arr) == 0 {
		return nil, false
	}
	return arr, true
}

func hashNonTextContent(h hash.Hash, content json.RawMessage) {
	if len(bytes.TrimSpace(content)) == 0 {
		return
	}
	var s string
	if json.Unmarshal(content, &s) == nil {
		return
	}
	var parts []json.RawMessage
	if json.Unmarshal(content, &parts) != nil {
		_, _ = h.Write(content)
		return
	}
	for _, p := range parts {
		var obj map[string]json.RawMessage
		if json.Unmarshal(p, &obj) != nil {
			_, _ = h.Write(p)
			_, _ = h.Write([]byte{0})
			continue
		}
		tBytes, hasType := obj["type"]
		if !hasType {
			_, _ = h.Write(p)
			_, _ = h.Write([]byte{0})
			continue
		}
		var typeStr string
		_ = json.Unmarshal(tBytes, &typeStr)
		if typeStr == "text" {
			continue
		}
		_, _ = h.Write(p)
		_, _ = h.Write([]byte{0})
	}
}

const defaultPromptSimilarityThreshold = 0.72

func promptTextSimilarity(a, b string) float64 {
	aTokens := tokenSet(a)
	bTokens := tokenSet(b)
	if len(aTokens) == 0 || len(bTokens) == 0 {
		return 0
	}
	intersection := 0
	for token := range aTokens {
		if _, ok := bTokens[token]; ok {
			intersection++
		}
	}
	return float64(intersection) / float64(max(len(aTokens), len(bTokens)))
}

func tokenSet(text string) map[string]struct{} {
	fields := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	set := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		if len(field) < 2 && !isAllDigits(field) {
			continue
		}
		set[field] = struct{}{}
	}
	return set
}

func isAllDigits(text string) bool {
	if text == "" {
		return false
	}
	for _, r := range text {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func extractContent(content any) string {
	switch v := content.(type) {
	case string:
		return v
	case []any:
		var parts []string
		for _, item := range v {
			if m, ok := item.(map[string]any); ok {
				if t, ok := m["type"].(string); ok && t == "text" {
					if text, ok := m["text"].(string); ok {
						parts = append(parts, text)
					}
				}
			}
		}
		return strings.Join(parts, " ")
	default:
		return fmt.Sprintf("%v", content)
	}
}

func computeParamsHashForContext(ctx context.Context, body []byte, endpointPath string, plan *core.Workflow, guardrailsHash, embedderIdentity string) string {
	return computeParamsHashWithTenant(body, endpointPath, plan, guardrailsHash, embedderIdentity, scope.EffectiveID(ctx))
}

func computeParamsHash(body []byte, endpointPath string, plan *core.Workflow, guardrailsHash, embedderIdentity string) string {
	return computeParamsHashWithTenant(body, endpointPath, plan, guardrailsHash, embedderIdentity, scope.DefaultID)
}

func computeParamsHashWithTenant(body []byte, endpointPath string, plan *core.Workflow, guardrailsHash, embedderIdentity, tenantID string) string {
	var req struct {
		Model           string              `json:"model"`
		Temperature     *float64            `json:"temperature"`
		TopP            *float64            `json:"top_p"`
		MaxTokens       *int                `json:"max_tokens"`
		MaxOutputTokens *int                `json:"max_output_tokens"`
		Tools           []map[string]any    `json:"tools"`
		ResponseFormat  any                 `json:"response_format"`
		Stream          bool                `json:"stream,omitempty"`
		StreamOptions   *core.StreamOptions `json:"stream_options"`
		Reasoning       json.RawMessage     `json:"reasoning"`
		Instructions    string              `json:"instructions"`
	}
	_ = json.Unmarshal(body, &req)

	h := sha256.New()
	h.Write([]byte(strings.TrimSpace(tenantID)))
	h.Write([]byte{0})
	h.Write([]byte(req.Model))
	h.Write([]byte{0})
	h.Write([]byte(endpointPath))
	h.Write([]byte{0})

	if plan != nil {
		h.Write([]byte(plan.ProviderType))
		h.Write([]byte{0})
		h.Write([]byte(plan.ResolvedQualifiedModel()))
		h.Write([]byte{0})
	}

	if req.Temperature != nil {
		h.Write([]byte(strconv.FormatFloat(*req.Temperature, 'f', -1, 64)))
	}
	h.Write([]byte{0})

	if req.TopP != nil {
		h.Write([]byte(strconv.FormatFloat(*req.TopP, 'f', -1, 64)))
	}
	h.Write([]byte{0})

	if req.MaxTokens != nil {
		h.Write([]byte(strconv.Itoa(*req.MaxTokens)))
	}
	h.Write([]byte{0})

	if req.MaxOutputTokens != nil {
		h.Write([]byte(strconv.Itoa(*req.MaxOutputTokens)))
	}
	h.Write([]byte{0})

	if len(req.Reasoning) > 0 {
		var canonical any
		if err := json.Unmarshal(req.Reasoning, &canonical); err == nil {
			remarshaled, _ := json.Marshal(canonical)
			h.Write(remarshaled)
		} else {
			h.Write(req.Reasoning)
		}
	}
	h.Write([]byte{0})

	if req.Instructions != "" {
		h.Write([]byte(req.Instructions))
	}
	h.Write([]byte{0})

	if len(req.Tools) > 0 {
		toolsJSON, _ := json.Marshal(sortedTools(req.Tools))
		xx := xxhash.Sum64(toolsJSON)
		h.Write([]byte(strconv.FormatUint(xx, 16)))
	}
	h.Write([]byte{0})

	if req.ResponseFormat != nil {
		rfJSON, _ := json.Marshal(req.ResponseFormat)
		h.Write(rfJSON)
	}
	h.Write([]byte{0})

	if req.Stream {
		h.Write([]byte("1"))
	}
	h.Write([]byte{0})

	if streamOptions := normalizeStreamOptionsForCache(req.StreamOptions); req.Stream && streamOptions != nil {
		soJSON, _ := json.Marshal(streamOptions)
		h.Write(soJSON)
	}
	h.Write([]byte{0})

	h.Write([]byte(guardrailsHash))
	h.Write([]byte{0})
	h.Write([]byte(embedderIdentity))

	return hex.EncodeToString(h.Sum(nil))
}

func sortedTools(tools []map[string]any) []map[string]any {
	sorted := make([]map[string]any, len(tools))
	copy(sorted, tools)
	sort.Slice(sorted, func(i, j int) bool {
		ni, _ := sorted[i]["function"].(map[string]any)
		nj, _ := sorted[j]["function"].(map[string]any)
		if ni == nil || nj == nil {
			return false
		}
		namei, _ := ni["name"].(string)
		namej, _ := nj["name"].(string)
		return namei < namej
	})
	return sorted
}

func shouldSkipSemanticCache(req *http.Request) bool {
	if shouldSkipCache(req) {
		return true
	}
	ct := req.Header.Get("X-Cache-Type")
	return strings.EqualFold(ct, "exact")
}

func headerFloat64(req *http.Request, name string) float64 {
	s := req.Header.Get(name)
	if s == "" {
		return 0
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return v
}

func headerDuration(req *http.Request, name string) time.Duration {
	s := req.Header.Get(name)
	if s == "" {
		return 0
	}
	seconds, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return time.Duration(seconds) * time.Second
}

func sha256HexOf(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func ComputeGuardrailsHash(rules []GuardrailRuleDescriptor) string {
	if len(rules) == 0 {
		return ""
	}
	seeds := make([]string, len(rules))
	for i, r := range rules {
		contentXX := xxhash.Sum64String(r.Content)
		if r.Direction == "" || strings.EqualFold(r.Direction, "input") {
			seeds[i] = fmt.Sprintf("%s:%s:%d:%s:%016x", r.Name, r.Type, r.Order, r.Mode, contentXX)
			continue
		}
		seeds[i] = fmt.Sprintf("%s:%s:%d:%s:%s:%016x", r.Name, r.Type, r.Order, r.Mode, r.Direction, contentXX)
	}
	sort.Strings(seeds)
	combined := strings.Join(seeds, "|")
	h := sha256.Sum256([]byte(combined))
	return hex.EncodeToString(h[:])
}

type GuardrailRuleDescriptor struct {
	Name      string
	Type      string
	Order     int
	Mode      string
	Direction string
	Content   string
}

func GuardrailsHashFromContext(ctx context.Context) string {
	return core.GetGuardrailsHash(ctx)
}

func WithGuardrailsHash(ctx context.Context, hash string) context.Context {
	return core.WithGuardrailsHash(ctx, hash)
}

const (
	CacheTypeExact      = "exact"
	CacheTypeSemantic   = "semantic"
	CacheTypeBoth       = "both"
	CacheHeaderExact    = "HIT (exact)"
	CacheHeaderSemantic = "HIT (semantic)"
)

func ShouldSkipExactCache(req *http.Request) bool {
	return strings.EqualFold(req.Header.Get("X-Cache-Type"), CacheTypeSemantic)
}

func ShouldSkipAllCache(req *http.Request) bool {
	if strings.EqualFold(req.Header.Get("X-Cache-Control"), "no-store") {
		return true
	}
	cc := req.Header.Get("Cache-Control")
	if cc == "" {
		return false
	}
	directives := strings.Split(strings.ToLower(cc), ",")
	for _, d := range directives {
		d = strings.TrimSpace(d)
		if d == "no-cache" || d == "no-store" {
			return true
		}
	}
	return false
}

func IoReadAllBody(c *echo.Context) ([]byte, error) {
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return nil, err
	}
	c.Request().Body = io.NopCloser(bytes.NewReader(body))
	return body, nil
}
