// Package sessionhub provides a configurable header transformation and
// session-mapping engine for upstream providers. It intercepts inbound
// requests, applies per-provider rules (generate unique session IDs,
// rewrite headers, inject custom values), and stores mappings so the
// same inbound session always maps to the same outbound session per
// provider — avoiding duplicate detection by upstream APIs.
package sessionhub

import (
	"crypto/rand"
	"fmt"
	"os"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// HeaderMode defines how a header value is transformed.
type HeaderMode string

const (
	// HeaderModeGenerate creates a new random value (e.g. ses_abc123...).
	HeaderModeGenerate HeaderMode = "generate"
	// HeaderModePassthrough passes the original value unchanged.
	HeaderModePassthrough HeaderMode = "passthrough"
	// HeaderModeStatic replaces the value with a fixed string.
	HeaderModeStatic HeaderMode = "static"
	// HeaderModeRandomFromList picks a random value from a predefined list.
	HeaderModeRandomFromList HeaderMode = "random_from_list"
	// HeaderModeRemove strips the header entirely.
	HeaderModeRemove HeaderMode = "remove"
	// HeaderModeMap maps the inbound value to a stored unique outbound value
	// (auto-generates on first encounter, reuses thereafter).
	HeaderModeMap HeaderMode = "map"
	// HeaderModeMapOrGenerate is like "map" but falls back to "generate" when
	// the inbound header is absent — useful for clients like the OpenCode CLI
	// that only sometimes send a session header.
	HeaderModeMapOrGenerate HeaderMode = "map_or_generate"
)

// HeaderRule defines transformation for a single header.
type HeaderRule struct {
	Name   string     `yaml:"name"    json:"name"`
	Mode   HeaderMode `yaml:"mode"    json:"mode"`
	Prefix string     `yaml:"prefix"  json:"prefix"`
	Length int        `yaml:"length"  json:"length"`
	Value  string     `yaml:"value"   json:"value"`
	Values []string   `yaml:"values"  json:"values"`
}

// ProviderRule defines the header transformations bound to a single target
// (provider | pool | fallback | "*" wildcard).
type ProviderRule struct {
	Enabled bool         `yaml:"enabled"  json:"enabled"`
	Headers []HeaderRule `yaml:"headers"  json:"headers"`
}

// HubConfig is the root config for the session hub. Rules are keyed by a
// target name that the rule is bound to (provider, pool, fallback, type, or
// "*" for all).
type HubConfig struct {
	Enabled   bool                    `yaml:"enabled"   json:"enabled"`
	Providers map[string]ProviderRule `yaml:"providers" json:"providers"`
	// MappingStorage controls where live inbound->outbound mappings live:
	// "memory" (reset on restart) or "disk" (persisted to MappingsPath).
	MappingStorage string `yaml:"mapping_storage" json:"mapping_storage"`
	// Path is the file this config is persisted to / loaded from. Not serialized
	// into the YAML document body.
	Path string `yaml:"-" json:"-"`
	// MappingsPath is the file live session mappings are persisted to when
	// MappingStorage == "disk". Not serialized into the YAML document body.
	MappingsPath string `yaml:"-" json:"-"`
}

// defaultHeaderRules returns sensible defaults for known header names.
func defaultHeaderRules() []HeaderRule {
	return []HeaderRule{
		{
			Name:   "x-opencode-session",
			Mode:   HeaderModeMapOrGenerate,
			Prefix: "ses_",
			Length: 32,
		},
	}
}

// LoadConfig reads session hub config from a YAML file.
func LoadConfig(path string) (*HubConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("sessionhub: read config: %w", err)
	}
	return ParseConfig(data)
}

// ParseConfig parses session hub config from YAML bytes.
func ParseConfig(data []byte) (*HubConfig, error) {
	cfg := &HubConfig{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("sessionhub: parse config: %w", err)
	}
	normalizeConfig(cfg)
	return cfg, nil
}

func normalizeConfig(cfg *HubConfig) {
	if cfg.Providers == nil {
		cfg.Providers = make(map[string]ProviderRule)
	}
	// Apply defaults: for any provider without explicit headers, add
	// the default x-opencode-session mapping rule.
	for name, rule := range cfg.Providers {
		if len(rule.Headers) == 0 {
			rule.Enabled = true
			rule.Headers = defaultHeaderRules()
			cfg.Providers[name] = rule
		}
	}
}

// MarshalYAML renders the config back to YAML bytes (for persistence).
func (c *HubConfig) MarshalYAML() ([]byte, error) {
	return yaml.Marshal(c)
}

// GenerateID produces a random alphanumeric string of the given length.
func GenerateID(length int) string {
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	rand.Read(b)
	for i := range b {
		b[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return string(b)
}

// GenerateValue builds a full value with prefix + random alphanumeric.
func GenerateValue(prefix string, length int) string {
	if length <= 0 {
		length = 28
	}
	return prefix + GenerateID(length)
}

// SessionEntry is one inbound→outbound mapping.
type SessionEntry struct {
	InboundValue  string    `json:"inbound_value"`
	OutboundValue string    `json:"outbound_value"`
	Provider      string    `json:"provider"`
	CreatedAt     time.Time `json:"created_at"`
}

// Store is a thread-safe session mapping store. When configured with a
// persistence path, mappings are written to disk on change and restored on
// start, so outbound identities survive restarts.
type Store struct {
	mu      sync.RWMutex
	entries map[string]*SessionEntry // key = provider + "|" + inbound_value
	byOut   map[string]*SessionEntry // key = provider + "|" + outbound_value
	order   []string                 // insertion order for listing

	persistPath string
	persistOn   bool
}

// StoreOption configures a Store.
type StoreOption func(*Store)

// WithPersistence enables on-disk persistence at the given path. The store
// loads any existing mappings on start and writes on every mutation.
func WithPersistence(path string) StoreOption {
	return func(s *Store) {
		if path == "" {
			return
		}
		s.persistPath = path
		s.persistOn = true
	}
}

// NewStore creates an empty store.
func NewStore(opts ...StoreOption) *Store {
	s := &Store{
		entries: make(map[string]*SessionEntry),
		byOut:   make(map[string]*SessionEntry),
	}
	for _, o := range opts {
		if o != nil {
			o(s)
		}
	}
	if s.persistOn {
		s.loadLocked()
	}
	return s
}

func storeKey(provider, inbound string) string { return provider + "|" + inbound }
func outKey(provider, outbound string) string  { return provider + "|" + outbound }

// Get returns the outbound value for an inbound value + provider, or empty.
func (s *Store) Get(provider, inbound string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.entries[storeKey(provider, inbound)]
	if !ok {
		return ""
	}
	return e.OutboundValue
}

// GetOrCreate returns the existing outbound value or creates a new one.
func (s *Store) GetOrCreate(provider, inbound, prefix string, length int) string {
	s.mu.Lock()
	key := storeKey(provider, inbound)
	if e, ok := s.entries[key]; ok {
		v := e.OutboundValue
		s.mu.Unlock()
		return v
	}
	out := GenerateValue(prefix, length)
	e := &SessionEntry{
		InboundValue:  inbound,
		OutboundValue: out,
		Provider:      provider,
		CreatedAt:     time.Now(),
	}
	s.entries[key] = e
	s.byOut[outKey(provider, out)] = e
	s.order = append(s.order, key)
	s.persistLocked()
	s.mu.Unlock()
	return out
}

// Set manually registers a mapping.
func (s *Store) Set(provider, inbound, outbound string) {
	s.mu.Lock()
	key := storeKey(provider, inbound)
	e := &SessionEntry{
		InboundValue:  inbound,
		OutboundValue: outbound,
		Provider:      provider,
		CreatedAt:     time.Now(),
	}
	s.entries[key] = e
	s.byOut[outKey(provider, outbound)] = e
	s.order = append(s.order, key)
	s.persistLocked()
	s.mu.Unlock()
}

// Delete removes a mapping.
func (s *Store) Delete(provider, inbound string) bool {
	s.mu.Lock()
	key := storeKey(provider, inbound)
	e, ok := s.entries[key]
	if !ok {
		s.mu.Unlock()
		return false
	}
	delete(s.entries, key)
	delete(s.byOut, outKey(provider, e.OutboundValue))
	s.persistLocked()
	s.mu.Unlock()
	return true
}

// Clear removes all mappings.
func (s *Store) Clear() {
	s.mu.Lock()
	s.entries = make(map[string]*SessionEntry)
	s.byOut = make(map[string]*SessionEntry)
	s.order = nil
	s.persistLocked()
	s.mu.Unlock()
}

// StorageMode returns the current persistence mode: "disk" when enabled.
func (s *Store) StorageMode() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.persistOn {
		return "disk"
	}
	return "memory"
}

// SetPersists enables or disables on-disk writes. Enabling immediately flushes
// the current in-memory snapshot so no mappings are lost on the transition.
func (s *Store) SetPersists(on bool) {
	s.mu.Lock()
	if on == s.persistOn {
		s.mu.Unlock()
		return
	}
	s.persistOn = on
	if on {
		s.persistLocked()
	}
	s.mu.Unlock()
}

// List returns all entries.
func (s *Store) List() []*SessionEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*SessionEntry, 0, len(s.order))
	for _, key := range s.order {
		if e, ok := s.entries[key]; ok {
			out = append(out, e)
		}
	}
	return out
}

// Stats returns summary counts.
func (s *Store) Stats() (total int, byProvider map[string]int) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	byProvider = make(map[string]int)
	for _, e := range s.entries {
		total++
		byProvider[e.Provider]++
	}
	return
}

// persistLocked writes the store to disk. Caller must hold s.mu.
func (s *Store) persistLocked() {
	if !s.persistOn || s.persistPath == "" {
		return
	}
	list := make([]*SessionEntry, 0, len(s.order))
	for _, key := range s.order {
		if e, ok := s.entries[key]; ok {
			list = append(list, e)
		}
	}
	if err := writeSessionsJSON(s.persistPath, list); err != nil {
		return // non-fatal; best-effort persistence
	}
}

// loadLocked restores persisted entries from disk. Only called at construction.
func (s *Store) loadLocked() {
	if s.persistPath == "" {
		return
	}
	list, err := readSessionsJSON(s.persistPath)
	if err != nil {
		return
	}
	for _, e := range list {
		if e == nil || e.Provider == "" || e.InboundValue == "" || e.OutboundValue == "" {
			continue
		}
		key := storeKey(e.Provider, e.InboundValue)
		if _, exists := s.entries[key]; exists {
			continue
		}
		s.entries[key] = e
		s.byOut[outKey(e.Provider, e.OutboundValue)] = e
		s.order = append(s.order, key)
	}
}
