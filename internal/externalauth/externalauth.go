// Package externalauth is the neutral bridge between the gateway and
// extension-supplied auth addons. The gateway ships no grant flows of its
// own: providers and admin routes dispatch generic calls (JSON in, JSON out)
// to whichever auth addon extensions have loaded.
package externalauth

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"aurora/internal/addon"
)

// ErrDisabled is returned when no auth addon is loaded.
var ErrDisabled = errors.New("external auth is disabled: apply an extension that ships an auth addon")

// ErrProviderScope reports that every loaded auth addon declined the call
// because none of them owns the requested provider (skip replies only). The
// HTTP relay maps it to 400 with a readable message instead of a 502 that
// blames a grant failure.
var ErrProviderScope = errors.New("provider not found or external auth not configured")

// Bridge routes generic auth calls to loaded auth addons (kind "auth").
// It is safe for concurrent use.
type Bridge struct {
	mu    sync.RWMutex
	store *addon.Store
}

// NewBridge creates an empty bridge. Attach the addon store with SetStore
// once addon loading has completed.
func NewBridge() *Bridge {
	return &Bridge{}
}

// SetStore attaches the addon store (nil resets the bridge to disabled).
func (b *Bridge) SetStore(store *addon.Store) {
	b.mu.Lock()
	b.store = store
	b.mu.Unlock()
}

// enabledAddons returns the loaded auth addons, or nil when none exist.
func (b *Bridge) enabledAddons() []*addon.Addon {
	b.mu.RLock()
	store := b.store
	b.mu.RUnlock()
	if store == nil {
		return nil
	}
	return store.ByKind(addon.KindAuth)
}

// Enabled reports whether at least one auth addon is available.
func (b *Bridge) Enabled() bool {
	return len(b.enabledAddons()) > 0
}

// Count returns the number of loaded auth addons.
func (b *Bridge) Count() int {
	return len(b.enabledAddons())
}

// Call dispatches a named method to the loaded auth addons and returns the
// first successful result. Addon methods receive a single JSON argument and
// reply with a JSON string. A reply carrying {"error": …, "skip": true} means
// "this provider is not mine" — the bridge moves on to the next addon.
func (b *Bridge) Call(method string, args any) (string, error) {
	list := b.enabledAddons()
	if len(list) == 0 {
		return "", ErrDisabled
	}
	payload, err := json.Marshal(args)
	if err != nil {
		return "", fmt.Errorf("encode arguments: %w", err)
	}
	var lastErr error
	skippedOnly := true
	for _, a := range list {
		out, callErr := a.CallString(method, string(payload))
		if callErr != nil {
			lastErr = callErr
			skippedOnly = false
			continue
		}
		if msg, skipped := skipReply(out); skipped {
			lastErr = errors.New(msg)
			continue
		}
		return out, nil
	}
	if lastErr == nil {
		lastErr = ErrDisabled
	}
	// Every addon either declined or errored: a pure-skip sweep means the
	// provider simply is not owned by any linked-account extension.
	if skippedOnly && !errors.Is(lastErr, ErrDisabled) {
		if name, _ := args.(map[string]any)["provider"].(string); name != "" {
			return "", fmt.Errorf("%w: %s", ErrProviderScope, name)
		}
		return "", ErrProviderScope
	}
	return "", fmt.Errorf("auth addon call %s: %w", method, lastErr)
}

// CallList dispatches a method that returns a JSON array and merges results
// from every auth addon, de-duplicating entries by their "name" field.
func (b *Bridge) CallList(method string, args any) (string, error) {
	list := b.enabledAddons()
	if len(list) == 0 {
		return "", ErrDisabled
	}
	payload, err := json.Marshal(args)
	if err != nil {
		return "", fmt.Errorf("encode arguments: %w", err)
	}
	seen := make(map[string]bool)
	merged := make([]map[string]any, 0)
	var lastErr error
	for _, a := range list {
		out, callErr := a.CallString(method, string(payload))
		if callErr != nil {
			lastErr = callErr
			continue
		}
		if msg, skipped := skipReply(out); skipped {
			lastErr = errors.New(msg)
			continue
		}
		var items []map[string]any
		if err := json.Unmarshal([]byte(out), &items); err != nil {
			lastErr = fmt.Errorf("auth addon %s: invalid list payload: %w", method, err)
			continue
		}
		for _, item := range items {
			key, _ := item["name"].(string)
			if key != "" {
				if seen[key] {
					continue
				}
				seen[key] = true
			}
			merged = append(merged, item)
		}
	}
	if len(merged) == 0 && lastErr != nil {
		return "", fmt.Errorf("auth addon call %s: %w", method, lastErr)
	}
	encoded, err := json.Marshal(merged)
	if err != nil {
		return "", fmt.Errorf("encode list result: %w", err)
	}
	return string(encoded), nil
}

// skipReply reports whether an addon reply is a "not my provider" marker and
// returns its error text.
func skipReply(raw string) (string, bool) {
	var obj map[string]any
	if err := json.Unmarshal([]byte(raw), &obj); err != nil {
		return "", false
	}
	if skip, _ := obj["skip"].(bool); !skip {
		return "", false
	}
	msg, _ := obj["error"].(string)
	if msg == "" {
		msg = "provider not handled by this auth addon"
	}
	return msg, true
}

// Token fetches a ready-to-use bearer token for the named provider. The
// addon performs any freshness checks / refreshes internally.
func (b *Bridge) Token(provider string) (string, error) {
	raw, err := b.Call("Token", map[string]any{"provider": provider})
	if err != nil {
		return "", err
	}
	var resp struct {
		Token string `json:"token"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return "", fmt.Errorf("auth addon token result: %w", err)
	}
	if resp.Error != "" {
		return "", errors.New(resp.Error)
	}
	return resp.Token, nil
}
