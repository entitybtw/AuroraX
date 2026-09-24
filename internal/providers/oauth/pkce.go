// Package oauth: OAuth 2.0 authorization-code + PKCE (RFC 7636) helpers.
// Endpoints come from extension-supplied config — no hardcoded origins.
package oauth

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// AuthCodeConfig supplies authorization-code + PKCE wiring (extension-driven).
type AuthCodeConfig struct {
	AuthorizeURL string
	TokenURL     string
	ClientID     string
	Scopes       string
	RedirectURI  string
	// TokenStyle: "json" (default) sends a JSON body; "form" uses
	// application/x-www-form-urlencoded.
	TokenStyle string
	// StateIsVerifier: when true, state equals the code_verifier (some
	// providers require state to carry the verifier).
	StateIsVerifier bool
}

// AuthCodeStart is the client-facing result of starting an authorize step.
type AuthCodeStart struct {
	AuthorizeURL string
	State        string
	RedirectURI  string
	ExpiresIn    int
}

type pendingAuth struct {
	cfg       AuthCodeConfig
	verifier  string
	state     string
	expiresAt time.Time
}

// AuthCodeStore tracks in-flight PKCE authorize sessions keyed by state.
type AuthCodeStore struct {
	mu      sync.Mutex
	byState map[string]*pendingAuth
}

// NewAuthCodeStore creates an empty store.
func NewAuthCodeStore() *AuthCodeStore {
	return &AuthCodeStore{byState: make(map[string]*pendingAuth)}
}

// Start creates a PKCE session and returns the authorize URL to open.
func (s *AuthCodeStore) Start(cfg AuthCodeConfig) (*AuthCodeStart, error) {
	cfg.AuthorizeURL = strings.TrimSpace(cfg.AuthorizeURL)
	cfg.TokenURL = strings.TrimSpace(cfg.TokenURL)
	cfg.ClientID = strings.TrimSpace(cfg.ClientID)
	cfg.RedirectURI = strings.TrimSpace(cfg.RedirectURI)
	if cfg.AuthorizeURL == "" || cfg.ClientID == "" {
		return nil, fmt.Errorf("oauth: authorize_url and client_id are required")
	}
	if cfg.RedirectURI == "" {
		cfg.RedirectURI = "http://127.0.0.1:54545/callback"
	}
	if cfg.TokenStyle == "" {
		cfg.TokenStyle = "json"
	}

	verifier, err := generateVerifier()
	if err != nil {
		return nil, err
	}
	challenge, err := challengeS256(verifier)
	if err != nil {
		return nil, err
	}
	state, err := generateState()
	if err != nil {
		return nil, err
	}
	if cfg.StateIsVerifier {
		state = verifier
	}

	u, err := url.Parse(cfg.AuthorizeURL)
	if err != nil {
		return nil, fmt.Errorf("oauth: invalid authorize_url: %w", err)
	}
	q := u.Query()
	q.Set("response_type", "code")
	q.Set("client_id", cfg.ClientID)
	q.Set("redirect_uri", cfg.RedirectURI)
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")
	q.Set("state", state)
	if scopes := strings.TrimSpace(cfg.Scopes); scopes != "" {
		q.Set("scope", scopes)
	}
	u.RawQuery = q.Encode()

	expiresIn := 600
	s.mu.Lock()
	s.byState[state] = &pendingAuth{
		cfg:       cfg,
		verifier:  verifier,
		state:     state,
		expiresAt: time.Now().Add(time.Duration(expiresIn) * time.Second),
	}
	s.pruneLocked()
	s.mu.Unlock()

	return &AuthCodeStart{
		AuthorizeURL: u.String(),
		State:        state,
		RedirectURI:  cfg.RedirectURI,
		ExpiresIn:    expiresIn,
	}, nil
}

// Complete exchanges an authorization code for tokens and clears the session.
func (s *AuthCodeStore) Complete(state, code string) (*TokenResponse, AuthCodeConfig, error) {
	state = strings.TrimSpace(state)
	code = strings.TrimSpace(code)
	if state == "" || code == "" {
		return nil, AuthCodeConfig{}, fmt.Errorf("oauth: state and code are required")
	}

	s.mu.Lock()
	pending := s.byState[state]
	delete(s.byState, state)
	s.mu.Unlock()

	if pending == nil {
		return nil, AuthCodeConfig{}, fmt.Errorf("oauth: unknown or expired state")
	}
	if time.Now().After(pending.expiresAt) {
		return nil, AuthCodeConfig{}, fmt.Errorf("oauth: authorize session expired")
	}

	token, err := exchangeAuthorizationCode(pending.cfg, code, pending.verifier, pending.state)
	if err != nil {
		return nil, AuthCodeConfig{}, err
	}
	return token, pending.cfg, nil
}

// Has reports whether a non-expired session exists for state.
func (s *AuthCodeStore) Has(state string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.byState[strings.TrimSpace(state)]
	if p == nil {
		return false
	}
	if time.Now().After(p.expiresAt) {
		delete(s.byState, strings.TrimSpace(state))
		return false
	}
	return true
}

func (s *AuthCodeStore) pruneLocked() {
	now := time.Now()
	for k, p := range s.byState {
		if now.After(p.expiresAt) {
			delete(s.byState, k)
		}
	}
}

func generateVerifier() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func generateState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func challengeS256(verifier string) (string, error) {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:]), nil
}

func exchangeAuthorizationCode(cfg AuthCodeConfig, code, verifier, state string) (*TokenResponse, error) {
	if cfg.TokenURL == "" {
		return nil, fmt.Errorf("oauth: token_url is required for authorization code exchange")
	}

	fields := map[string]string{
		"grant_type":    "authorization_code",
		"code":          code,
		"redirect_uri":  cfg.RedirectURI,
		"client_id":     cfg.ClientID,
		"code_verifier": verifier,
	}
	if state != "" {
		fields["state"] = state
	}

	var body []byte
	var contentType string
	switch strings.ToLower(strings.TrimSpace(cfg.TokenStyle)) {
	case "form":
		v := url.Values{}
		for k, val := range fields {
			v.Set(k, val)
		}
		body = []byte(v.Encode())
		contentType = "application/x-www-form-urlencoded"
	default:
		var err error
		body, err = json.Marshal(fields)
		if err != nil {
			return nil, err
		}
		contentType = "application/json"
	}

	req, err := http.NewRequest("POST", cfg.TokenURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("oauth: create token request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("oauth: token request failed: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("oauth: read token response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("oauth: token exchange HTTP %d: %s", resp.StatusCode, string(raw))
	}

	var token TokenResponse
	if err := json.Unmarshal(raw, &token); err != nil {
		return nil, fmt.Errorf("oauth: decode token response: %w", err)
	}
	if token.AccessToken == "" {
		return nil, fmt.Errorf("oauth: token response missing access_token")
	}
	return &token, nil
}

// RefreshAuthCode refreshes via the authorization-code token endpoint (JSON
// or form body per TokenStyle). Rotates refresh_token when returned.
func RefreshAuthCode(cfg AuthCodeConfig, refreshToken string) (*TokenResponse, error) {
	if cfg.TokenURL == "" {
		return nil, fmt.Errorf("oauth: token_url is required for refresh")
	}
	if refreshToken == "" {
		return nil, fmt.Errorf("oauth: no refresh token")
	}
	if cfg.ClientID == "" {
		return nil, fmt.Errorf("oauth: client_id is required for refresh")
	}

	fields := map[string]string{
		"grant_type":    "refresh_token",
		"refresh_token": refreshToken,
		"client_id":     cfg.ClientID,
	}
	if scopes := strings.TrimSpace(cfg.Scopes); scopes != "" {
		fields["scope"] = scopes
	}

	var body []byte
	var contentType string
	switch strings.ToLower(strings.TrimSpace(cfg.TokenStyle)) {
	case "form":
		v := url.Values{}
		for k, val := range fields {
			v.Set(k, val)
		}
		body = []byte(v.Encode())
		contentType = "application/x-www-form-urlencoded"
	default:
		var err error
		body, err = json.Marshal(fields)
		if err != nil {
			return nil, err
		}
		contentType = "application/json"
	}

	req, err := http.NewRequest("POST", cfg.TokenURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("oauth: create refresh request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("oauth: refresh request failed: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("oauth: read refresh response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("oauth: refresh HTTP %d: %s", resp.StatusCode, string(raw))
	}

	var token TokenResponse
	if err := json.Unmarshal(raw, &token); err != nil {
		return nil, fmt.Errorf("oauth: decode refresh response: %w", err)
	}
	if token.AccessToken == "" {
		return nil, fmt.Errorf("oauth: refresh response missing access_token")
	}
	return &token, nil
}

// ConstantTimeEquals compares two strings in constant time.
func ConstantTimeEquals(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
