// Package hooks is the neutral bridge between the gateway and extension
// addons for everything that is not an auth grant flow.
//
// externalauth owns the auth surface (device / authorization-code flows).
// This package owns the rest of the extension hook kit: lifecycle events,
// egress decisions, retry policy and live UI data. Every call follows the
// same contract as the auth bridge — one JSON string in, one JSON string out
// — so an addon stays a single-file Yaegi script.
//
// Addons opt in by exporting the hook name; an addon that does not export it
// is skipped rather than treated as an error, which is what lets several
// extensions coexist without knowing about each other.
package hooks

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"aurora/internal/addon"
)

// ErrDisabled reports that no addon store is attached.
var ErrDisabled = errors.New("hooks are disabled: apply an extension that ships a runtime addon")

// ErrNotOwned reports that every candidate addon declined the hook.
var ErrNotOwned = errors.New("no addon handles this hook")

// ExtensionInfo is the part of an extension record an addon is allowed to
// read. Settings come from the shipped manifest; Config holds the operator's
// saved values for ui.fields (keys, subscription lists, intervals, …).
type ExtensionInfo struct {
	ID       string            `json:"id"`
	Dir      string            `json:"dir"`
	Settings map[string]string `json:"settings,omitempty"`
	Config   map[string]string `json:"config,omitempty"`
	Applied  bool              `json:"applied"`
}

// OwnerOf maps an addon source path to the id of the extension that shipped
// it. Addons outside an extension's files tree return "".
type OwnerOf func(addonPath string) string

// Lookup resolves an extension id to the info merged into hook payloads.
type Lookup func(extID string) (ExtensionInfo, bool)

// Result is a single addon's reply to a dispatched hook.
type Result struct {
	Addon string `json:"addon"`
	Value string `json:"value,omitempty"`
	Error string `json:"error,omitempty"`
}

// Bridge routes generic hook calls to loaded addons. It is safe for
// concurrent use.
type Bridge struct {
	mu      sync.RWMutex
	store   *addon.Store
	ownerOf OwnerOf
	lookup  Lookup
}

// NewBridge creates an empty bridge. Attach the addon store once loading has
// completed.
func NewBridge() *Bridge {
	return &Bridge{}
}

// SetStore attaches the addon store (nil disables the bridge).
func (b *Bridge) SetStore(store *addon.Store) {
	b.mu.Lock()
	b.store = store
	b.mu.Unlock()
}

// SetContext installs the callbacks used to enrich hook payloads with the
// owning extension's identity and settings. Both may be nil, in which case
// payloads carry only the caller's own fields.
func (b *Bridge) SetContext(ownerOf OwnerOf, lookup Lookup) {
	b.mu.Lock()
	b.ownerOf = ownerOf
	b.lookup = lookup
	b.mu.Unlock()
}

// Enabled reports whether an addon store is attached.
func (b *Bridge) Enabled() bool {
	return b.storeRef() != nil
}

// Count returns the number of loaded addons.
func (b *Bridge) Count() int {
	store, _, _ := b.refs()
	if store == nil {
		return 0
	}
	return len(store.List())
}

// refs reads the store pointer and callbacks under one read lock.
func (b *Bridge) refs() (*addon.Store, OwnerOf, Lookup) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.store, b.ownerOf, b.lookup
}

func (b *Bridge) storeRef() *addon.Store {
	store, _, _ := b.refs()
	return store
}

// all returns every loaded addon, or nil when no store is attached.
func (b *Bridge) all() []*addon.Addon {
	store, _, _ := b.refs()
	if store == nil {
		return nil
	}
	names := store.List()
	out := make([]*addon.Addon, 0, len(names))
	for _, n := range names {
		if a := store.Get(n); a != nil {
			out = append(out, a)
		}
	}
	return out
}

// candidates returns the addons of the requested kind that export method.
// candidates returns the addons of the requested kind that export method.
// When ownerFilter is non-empty only addons shipped by that extension match,
// which is what scopes a per-extension endpoint to its own code.
func (b *Bridge) candidates(kind addon.Kind, method, ownerFilter string) ([]*addon.Addon, error) {
	list := b.all()
	if list == nil {
		return nil, ErrDisabled
	}
	_, ownerOf, _ := b.refs()
	var out []*addon.Addon
	for _, a := range list {
		if a.Kind != kind {
			continue
		}
		if ownerOf != nil {
			id := ownerOf(a.Path)
			if ownerFilter != "" && id != ownerFilter {
				continue
			}
			if id != "" {
				// A detached-but-still-loaded addon must not keep serving
				// hooks for an extension the operator disabled.
				if info, ok := b.infoFor(id); ok && !info.Applied {
					continue
				}
			}
		} else if ownerFilter != "" {
			continue
		}
		if !a.Has(method) {
			continue
		}
		out = append(out, a)
	}
	return out, nil
}

func (b *Bridge) infoFor(extID string) (ExtensionInfo, bool) {
	b.mu.RLock()
	lookup := b.lookup
	b.mu.RUnlock()
	if lookup == nil || extID == "" {
		return ExtensionInfo{}, false
	}
	return lookup(extID)
}

// payload renders the single JSON argument handed to an addon: the caller's
// fields merged with the owning extension's identity and settings.
func (b *Bridge) payload(hook, addonPath string, args map[string]any) string {
	out := make(map[string]any, len(args)+4)
	for k, v := range args {
		out[k] = v
	}
	out["hook"] = hook
	if ownerOf := func() OwnerOf { _, o, _ := b.refs(); return o }(); ownerOf != nil {
		if id := ownerOf(addonPath); id != "" {
			out["extension"] = id
			if info, ok := b.infoFor(id); ok {
				if info.Dir != "" {
					out["dir"] = info.Dir
				}
				if len(info.Settings) > 0 {
					out["settings"] = info.Settings
				}
				if len(info.Config) > 0 {
					out["config"] = info.Config
				}
			}
		}
	}
	raw, err := json.Marshal(out)
	if err != nil {
		return "{}"
	}
	return string(raw)
}

// skipReply reports whether an addon declined with {"skip": true} and returns
// its message.
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
		msg = "hook not handled by this addon"
	}
	return msg, true
}

// CallOwned dispatches method only to addons shipped by extID. Used by
// per-extension endpoints (live page data) so one extension's addon can never
// answer for another.
func (b *Bridge) CallOwned(kind addon.Kind, extID, method string, args map[string]any) (string, error) {
	if strings.TrimSpace(extID) == "" {
		return "", ErrNotOwned
	}
	list, err := b.candidates(kind, method, extID)
	if err != nil {
		return "", err
	}
	if len(list) == 0 {
		return "", ErrNotOwned
	}
	a := list[0]
	out, callErr := a.CallString(method, b.payload(method, a.Path, args))
	if callErr != nil {
		return "", callErr
	}
	if msg, skipped := skipReply(out); skipped {
		return "", errors.New(msg)
	}
	return out, nil
}

// UI collects live ExtensionUI documents from ui-kind addons. Auth-kind
// addons keep their own path (they are invoked directly by the admin UI
// contribution endpoint).
func (b *Bridge) UI() []Result {
	return b.CallAll(addon.KindUI, "UI", nil)
}

// Call dispatches method to every addon of kind and returns the first
// successful reply. An addon that does not export method is skipped silently;
// one that answers {"skip": true} means "not mine" and the search continues.
func (b *Bridge) Call(kind addon.Kind, method string, args map[string]any) (string, error) {
	list, err := b.candidates(kind, method, "")
	if err != nil {
		return "", err
	}
	if len(list) == 0 {
		return "", ErrNotOwned
	}
	var lastErr error
	skippedOnly := true
	for _, a := range list {
		out, callErr := a.CallString(method, b.payload(method, a.Path, args))
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
	if skippedOnly && lastErr != nil {
		return "", fmt.Errorf("%w: %v", ErrNotOwned, lastErr)
	}
	if lastErr == nil {
		lastErr = ErrNotOwned
	}
	return "", fmt.Errorf("hook %s: %w", method, lastErr)
}

// CallAll dispatches method to every addon of kind and collects all replies.
// Used by hooks whose semantics are "each addon contributes its own slice"
// (live UI blocks, status lists). Addon failures are reported per entry
// instead of failing the whole call.
func (b *Bridge) CallAll(kind addon.Kind, method string, args map[string]any) []Result {
	list, err := b.candidates(kind, method, "")
	if err != nil || len(list) == 0 {
		return nil
	}
	out := make([]Result, 0, len(list))
	for _, a := range list {
		raw, callErr := a.CallString(method, b.payload(method, a.Path, args))
		if callErr != nil {
			out = append(out, Result{Addon: a.Name, Error: callErr.Error()})
			continue
		}
		if msg, skipped := skipReply(raw); skipped {
			out = append(out, Result{Addon: a.Name, Error: msg})
			continue
		}
		out = append(out, Result{Addon: a.Name, Value: raw})
	}
	return out
}

// Fire runs a best-effort event against every addon of kind. Failures are
// logged and never propagate: a broken addon must not break the request or
// the lifecycle transition that triggered the event.
func (b *Bridge) Fire(kind addon.Kind, event string, args map[string]any) {
	list, err := b.candidates(kind, event, "")
	if err != nil || len(list) == 0 {
		return
	}
	for _, a := range list {
		if _, callErr := a.CallString(event, b.payload(event, a.Path, args)); callErr != nil {
			slog.Warn("addon hook failed", "addon", a.Name, "hook", event, "error", callErr)
		}
	}
}
