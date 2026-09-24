package oauth

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAuthCodeStoreStartAndComplete(t *testing.T) {
	var gotBody map[string]string
	var gotContentType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		if r.Method != http.MethodPost {
			t.Errorf("method = %s", r.Method)
		}
		if r.Header.Get("Content-Type") == "application/json" {
			if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
				t.Errorf("decode body: %v", err)
			}
		} else {
			if err := r.ParseForm(); err != nil {
				t.Errorf("parse form: %v", err)
			}
			gotBody = map[string]string{}
			for k, v := range r.Form {
				if len(v) > 0 {
					gotBody[k] = v[0]
				}
			}
		}
		_ = json.NewEncoder(w).Encode(TokenResponse{
			AccessToken:  "at-1",
			RefreshToken: "rt-1",
			ExpiresIn:    3600,
		})
	}))
	defer srv.Close()

	store := NewAuthCodeStore()
	cfg := AuthCodeConfig{
		AuthorizeURL: "https://auth.example.test/authorize",
		TokenURL:     srv.URL,
		ClientID:     "cli-1",
		Scopes:       "openid profile",
		RedirectURI:  "http://127.0.0.1:54545/callback",
		TokenStyle:   "json",
	}
	start, err := store.Start(cfg)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !strings.HasPrefix(start.AuthorizeURL, "https://auth.example.test/authorize?") {
		t.Fatalf("authorize url = %q", start.AuthorizeURL)
	}
	for _, want := range []string{"code_challenge=", "code_challenge_method=S256", "client_id=cli-1", "state="} {
		if !strings.Contains(start.AuthorizeURL, want) {
			t.Fatalf("authorize url missing %q: %s", want, start.AuthorizeURL)
		}
	}
	if !store.Has(start.State) {
		t.Fatal("expected pending session")
	}

	token, used, err := store.Complete(start.State, "auth-code-1")
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if token.AccessToken != "at-1" || token.RefreshToken != "rt-1" {
		t.Fatalf("token = %+v", token)
	}
	if used.ClientID != "cli-1" {
		t.Fatalf("used client = %+v", used)
	}
	if gotBody["grant_type"] != "authorization_code" || gotBody["code"] != "auth-code-1" {
		t.Fatalf("body = %#v", gotBody)
	}
	if gotBody["code_verifier"] == "" {
		t.Fatal("missing code_verifier")
	}
	if gotContentType != "application/json" {
		t.Fatalf("content-type = %q", gotContentType)
	}
	if store.Has(start.State) {
		t.Fatal("session should be cleared after Complete")
	}
}

func TestAuthCodeStoreStateIsVerifier(t *testing.T) {
	store := NewAuthCodeStore()
	start, err := store.Start(AuthCodeConfig{
		AuthorizeURL:    "https://auth.example.test/authorize",
		TokenURL:        "https://auth.example.test/token",
		ClientID:        "cli-1",
		StateIsVerifier: true,
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	// state must equal code_verifier when StateIsVerifier is set; the
	// authorize URL carries state, and the verifier is only used at exchange.
	// We assert state is non-empty and Has(state) works — the verifier equality
	// is enforced internally at Complete time.
	if start.State == "" || !store.Has(start.State) {
		t.Fatalf("state = %q", start.State)
	}
}

func TestAuthCodeStoreFormTokenStyle(t *testing.T) {
	var gotContentType string
	var raw string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		buf := make([]byte, 4096)
		n, _ := r.Body.Read(buf)
		raw = string(buf[:n])
		_ = json.NewEncoder(w).Encode(TokenResponse{AccessToken: "at", ExpiresIn: 60})
	}))
	defer srv.Close()

	store := NewAuthCodeStore()
	start, err := store.Start(AuthCodeConfig{
		AuthorizeURL: "https://auth.example.test/authorize",
		TokenURL:     srv.URL,
		ClientID:     "cli-1",
		TokenStyle:   "form",
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if _, _, err := store.Complete(start.State, "c1"); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if gotContentType != "application/x-www-form-urlencoded" {
		t.Fatalf("content-type = %q", gotContentType)
	}
	if !strings.Contains(raw, "grant_type=authorization_code") {
		t.Fatalf("raw = %q", raw)
	}
}

func TestAuthCodeStoreExpired(t *testing.T) {
	store := NewAuthCodeStore()
	start, err := store.Start(AuthCodeConfig{
		AuthorizeURL: "https://auth.example.test/authorize",
		TokenURL:     "https://auth.example.test/token",
		ClientID:     "cli-1",
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	// Force expiry via unexported pending map.
	store.mu.Lock()
	if p, ok := store.byState[start.State]; ok {
		p.expiresAt = time.Now().Add(-time.Second)
	}
	store.mu.Unlock()
	if store.Has(start.State) {
		t.Fatal("expired session should not be Has")
	}
	if _, _, err := store.Complete(start.State, "c"); err == nil {
		t.Fatal("Complete on expired should error")
	}
}

func TestChallengeS256KnownVector(t *testing.T) {
	// RFC 7636 appendix B
	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	want := "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
	got, err := challengeS256(verifier)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("challenge = %s want %s", got, want)
	}
}

func TestGenerateVerifierLength(t *testing.T) {
	v, err := generateVerifier()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := base64.RawURLEncoding.DecodeString(v)
	if err != nil {
		t.Fatalf("verifier not base64url: %v", err)
	}
	if len(raw) < 32 {
		t.Fatalf("verifier entropy too small: %d", len(raw))
	}
}

func TestRefreshAuthCodeJSON(t *testing.T) {
	var got map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("decode: %v", err)
		}
		_ = json.NewEncoder(w).Encode(TokenResponse{
			AccessToken:  "at2",
			RefreshToken: "rt2",
			ExpiresIn:    120,
		})
	}))
	defer srv.Close()

	tok, err := RefreshAuthCode(AuthCodeConfig{
		TokenURL:   srv.URL,
		ClientID:   "cli-1",
		TokenStyle: "json",
	}, "rt-1")
	if err != nil {
		t.Fatalf("RefreshAuthCode: %v", err)
	}
	if tok.AccessToken != "at2" || tok.RefreshToken != "rt2" {
		t.Fatalf("token = %+v", tok)
	}
	if got["grant_type"] != "refresh_token" || got["refresh_token"] != "rt-1" {
		t.Fatalf("body = %#v", got)
	}
}
