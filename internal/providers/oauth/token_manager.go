// Package oauth implements the RFC 8628 OAuth 2.0 device authorization grant
// for providers that require browser-based authentication (e.g. free tier).
package oauth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	DefaultServer   = "https://example.com/console"
	DefaultClientID = "opencode-cli"
	refreshSkew     = 5 * time.Minute
)

// DeviceCodeResponse is the server's response to a device authorization request.
type DeviceCodeResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

// TokenResponse is the server's response to a token exchange or refresh request.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

type tokenError struct {
	Error string `json:"error"`
}

// StoredToken is the persisted form of an OAuth token.
type StoredToken struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	Server       string    `json:"server"`
	ClientID     string    `json:"client_id"`
	AccountID    string    `json:"account_id,omitempty"`
	Email        string    `json:"email,omitempty"`
}

// Manager manages OAuth tokens for a single provider.
type Manager struct {
	mu                sync.RWMutex
	token             *StoredToken
	server            string
	clientID          string
	dataDir           string
	providerName      string
	httpClient        *http.Client
	tokenFileModTime  time.Time
}

// NewManager creates a new OAuth token manager.
func NewManager(server, clientID, dataDir, providerName string) *Manager {
	if server == "" {
		server = DefaultServer
	}
	if clientID == "" {
		clientID = DefaultClientID
	}
	if dataDir == "" {
		dataDir = "data"
	}
	m := &Manager{
		server:       server,
		clientID:     clientID,
		dataDir:      dataDir,
		providerName: providerName,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
	}
	m.loadToken()
	return m
}

// GetAccessToken returns a valid access token. Returns empty string if none stored.
// Checks if token file has been modified externally and reloads if needed.
func (m *Manager) GetAccessToken() string {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.tryReloadToken() {
		// Token was reloaded
	}
	if m.token == nil {
		return ""
	}
	return m.token.AccessToken
}

// tryReloadToken checks if token file was modified and reloads if needed.
// Returns true if token was reloaded.
func (m *Manager) tryReloadToken() bool {
	path := m.tokenFilePath()
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	if !info.ModTime().After(m.tokenFileModTime) {
		return false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var t StoredToken
	if err := json.Unmarshal(data, &t); err != nil {
		return false
	}
	m.token = &t
	m.tokenFileModTime = info.ModTime()
	return true
}

// HasToken reports whether a token is stored.
func (m *Manager) HasToken() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tryReloadToken()
	return m.token != nil && m.token.AccessToken != ""
}

// TokenInfo returns a copy of the stored token metadata.
func (m *Manager) TokenInfo() *StoredToken {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tryReloadToken()
	if m.token == nil {
		return nil
	}
	cp := *m.token
	return &cp
}

// StartDeviceFlow initiates the OAuth 2.0 device authorization grant.
func (m *Manager) StartDeviceFlow() (*DeviceCodeResponse, error) {
	body, _ := json.Marshal(map[string]string{
		"client_id": m.clientID,
	})
	req, err := http.NewRequest("POST", m.server+"/auth/device/code", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create device code request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("device code request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("device code request returned HTTP %d: %s", resp.StatusCode, string(b))
	}

	var dc DeviceCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&dc); err != nil {
		return nil, fmt.Errorf("decode device code response: %w", err)
	}
	return &dc, nil
}

// PollForToken polls the token endpoint until the user authorizes or the device code expires.
// interval is adjusted (+5s) on "slow_down" responses.
func (m *Manager) PollForToken(deviceCode string, interval time.Duration, expiresAt time.Time) (*TokenResponse, error) {
	for {
		if time.Now().After(expiresAt) {
			return nil, fmt.Errorf("device code expired")
		}

		time.Sleep(interval)

		token, pending, err := m.exchangeToken(deviceCode)
		if err != nil {
			return nil, err
		}
		if token != nil {
			return token, nil
		}
		if pending == "slow_down" {
			interval += 5 * time.Second
		}
	}
}

// exchangeToken attempts to exchange a device code for an access token.
// Returns (token, "", nil) on success, (nil, "authorization_pending", nil) when pending,
// (nil, "", error) on failure.
func (m *Manager) exchangeToken(deviceCode string) (*TokenResponse, string, error) {
	body, _ := json.Marshal(map[string]string{
		"grant_type":  "urn:ietf:params:oauth:grant-type:device_code",
		"device_code": deviceCode,
		"client_id":   m.clientID,
	})
	req, err := http.NewRequest("POST", m.server+"/auth/device/token", bytes.NewReader(body))
	if err != nil {
		return nil, "", fmt.Errorf("create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("read token response: %w", err)
	}

	if resp.StatusCode == http.StatusOK {
		var token TokenResponse
		if err := json.Unmarshal(b, &token); err != nil {
			return nil, "", fmt.Errorf("decode token response: %w", err)
		}
		return &token, "", nil
	}

	var terr tokenError
	if err := json.Unmarshal(b, &terr); err != nil {
		return nil, "", fmt.Errorf("decode token error: %w", err)
	}

	switch terr.Error {
	case "authorization_pending":
		return nil, "authorization_pending", nil
	case "slow_down":
		return nil, "slow_down", nil
	case "expired_token":
		return nil, "", fmt.Errorf("device code expired, user must re-authorize")
	case "access_denied", "authorization_denied":
		return nil, "", fmt.Errorf("authorization denied by user")
	default:
		return nil, "", fmt.Errorf("token error: %s", terr.Error)
	}
}

// RefreshToken refreshes the stored access token using the refresh token.
func (m *Manager) RefreshToken() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.token == nil || m.token.RefreshToken == "" {
		return fmt.Errorf("no refresh token available")
	}

	body, _ := json.Marshal(map[string]string{
		"grant_type":    "refresh_token",
		"refresh_token": m.token.RefreshToken,
		"client_id":     m.clientID,
	})
	req, err := http.NewRequest("POST", m.server+"/auth/device/token", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("refresh request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("refresh returned HTTP %d: %s", resp.StatusCode, string(b))
	}

	var token TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return fmt.Errorf("decode refresh response: %w", err)
	}

	m.token.AccessToken = token.AccessToken
	m.token.RefreshToken = token.RefreshToken
	m.token.ExpiresAt = time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)

	if err := m.saveToken(); err != nil {
		slog.Warn("oauth: failed to persist refreshed token", "provider", m.providerName, "error", err)
	}
	return nil
}

// SaveTokenFromResponse stores a token response obtained from the device flow.
func (m *Manager) SaveTokenFromResponse(token *TokenResponse) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.token = &StoredToken{
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(token.ExpiresIn) * time.Second),
		Server:       m.server,
		ClientID:     m.clientID,
	}
	return m.saveToken()
}

// EnsureFreshToken refreshes the token if it's about to expire. Thread-safe.
func (m *Manager) EnsureFreshToken() error {
	m.mu.RLock()
	if m.token == nil {
		m.mu.RUnlock()
		return nil
	}
	if time.Now().Add(refreshSkew).Before(m.token.ExpiresAt) {
		m.mu.RUnlock()
		return nil
	}
	m.mu.RUnlock()
	return m.RefreshToken()
}

// ClearToken removes the stored token.
func (m *Manager) ClearToken() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.token = nil
	return m.removeTokenFile()
}

func (m *Manager) tokenFilePath() string {
	return filepath.Join(m.dataDir, fmt.Sprintf("oauth_%s.json", m.providerName))
}

func (m *Manager) loadToken() {
	path := m.tokenFilePath()
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var t StoredToken
	if err := json.Unmarshal(data, &t); err != nil {
		slog.Warn("oauth: failed to load stored token", "provider", m.providerName, "error", err)
		return
	}
	m.token = &t
	if info, err := os.Stat(path); err == nil {
		m.tokenFileModTime = info.ModTime()
	}
}

func (m *Manager) saveToken() error {
	if err := os.MkdirAll(m.dataDir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m.token, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(m.tokenFilePath(), data, 0o600); err != nil {
		return err
	}
	if info, err := os.Stat(m.tokenFilePath()); err == nil {
		m.tokenFileModTime = info.ModTime()
	}
	return nil
}

func (m *Manager) removeTokenFile() error {
	path := m.tokenFilePath()
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// ExchangeTokenPublic is a public wrapper for polling. Returns (token, false, nil) on success,
// (nil, true, nil) when pending, (nil, false, error) on failure.
func (m *Manager) ExchangeTokenPublic(deviceCode string, interval time.Duration, expiresAt time.Time) (*TokenResponse, bool, error) {
	for {
		if time.Now().After(expiresAt) {
			return nil, false, fmt.Errorf("device code expired")
		}

		time.Sleep(interval)

		token, pending, err := m.exchangeToken(deviceCode)
		if err != nil {
			return nil, false, err
		}
		if token != nil {
			return token, false, nil
		}
		if pending == "slow_down" {
			interval += 5 * time.Second
		}
		if pending == "pending" {
			return nil, true, nil
		}
	}
}
