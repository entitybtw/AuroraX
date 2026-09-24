// Package addon loads optional single-file Go scripts via Yaegi.
//
// Addons are not required for core operation: extensions ship structured JSON
// first (settings, ui, provides). When a runtime hook is needed, an extension
// may materialize a single .go file under configs/addons/ and this package
// evaluates it in-process without recompiling the gateway.
//
// Security model: only files under the configured addons directory are
// accepted; paths with ".." or absolute paths are rejected. Operators control
// which addons exist on disk — treat the directory as trusted configuration,
// the same way configs/*.yaml is trusted.
package addon

import (
	"fmt"
	"os"
	"path/filepath"
	"plugin"
	"sort"
	"strings"
	"sync"

	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
)

// Kind classifies a loaded addon for routing/hooks.
type Kind string

const (
	// KindUnknown is the zero value (not yet classified).
	KindUnknown Kind = ""
	// KindTheme — palette logic / chart painters.
	KindTheme Kind = "theme"
	// KindPreset — custom retry/routing policies.
	KindPreset Kind = "preset"
	// KindUI — dashboard widgets beyond structured blocks.
	KindUI Kind = "ui"
	// KindAuth — additional OAuth grant types (authorization_code + PKCE, …).
	KindAuth Kind = "auth"
	// KindRuntime — generic lifecycle hooks.
	KindRuntime Kind = "runtime"
)

// Addon is a loaded single-file Go script ready to call exported helpers.
type Addon struct {
	// Name is the file base name without extension (stable id).
	Name string
	// Path is the absolute path to the .go source.
	Path string
	// Kind is the declared kind (from //go:build-style pragma or filename).
	Kind Kind

	mu     sync.Mutex
	i      *interp.Interpreter
	values map[string]string
	// exports are symbols successfully looked up after Eval.
	exports map[string]plugin.Symbol
}

// Store holds loaded addons keyed by Name.
type Store struct {
	mu      sync.RWMutex
	dir     string
	addons  map[string]*Addon
	lastErr map[string]string
}

// NewStore creates a store rooted at dir (empty = configs/addons).
func NewStore(dir string) *Store {
	if strings.TrimSpace(dir) == "" {
		dir = "configs/addons"
	}
	return &Store{
		dir:     dir,
		addons:  make(map[string]*Addon),
		lastErr: make(map[string]string),
	}
}

// Dir returns the configured addons directory.
func (s *Store) Dir() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.dir
}

// SetDir replaces the addons directory and clears loaded state.
func (s *Store) SetDir(dir string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.TrimSpace(dir) == "" {
		dir = "configs/addons"
	}
	s.dir = dir
	s.addons = make(map[string]*Addon)
	s.lastErr = make(map[string]string)
}

// Load reads every *.go under the addons directory (non-recursive) and
// evaluates each in a Yaegi interpreter. Returns names loaded successfully.
// Evaluation failures are recorded via Status and skipped (never fatal).
func (s *Store) Load() []string {
	s.mu.Lock()
	dir := s.dir
	s.mu.Unlock()

	entries, err := os.ReadDir(dir)
	if err != nil {
		if !os.IsNotExist(err) {
			s.mu.Lock()
			s.lastErr["<dir>"] = err.Error()
			s.mu.Unlock()
		}
		return nil
	}

	var loaded []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		if err := s.LoadFile(path); err != nil {
			continue
		}
		loaded = append(loaded, strings.TrimSuffix(e.Name(), ".go"))
	}
	sort.Strings(loaded)
	return loaded
}

// LoadFile evaluates a single .go file. The path must resolve under Dir.
func (s *Store) LoadFile(path string) error {
	s.mu.RLock()
	root := s.dir
	s.mu.RUnlock()

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("addon: resolve root: %w", err)
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("addon: resolve path: %w", err)
	}
	rel, err := filepath.Rel(absRoot, absPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return fmt.Errorf("addon: path %q escapes addons dir %q", path, root)
	}
	if !strings.HasSuffix(absPath, ".go") {
		return fmt.Errorf("addon: only .go files are supported")
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return fmt.Errorf("addon: read %s: %w", path, err)
	}

	name := strings.TrimSuffix(filepath.Base(absPath), ".go")
	i := interp.New(interp.Options{
		GoPath: os.Getenv("GOPATH"),
		Args:   []string{name},
	})
	if err := i.Use(stdlib.Symbols); err != nil {
		s.mu.Lock()
		s.lastErr[name] = err.Error()
		s.mu.Unlock()
		return fmt.Errorf("addon: stdlib symbols: %w", err)
	}
	if _, err := i.Eval(string(data)); err != nil {
		s.mu.Lock()
		s.lastErr[name] = err.Error()
		s.mu.Unlock()
		return fmt.Errorf("addon: eval %s: %w", name, err)
	}

	a := &Addon{
		Name:    name,
		Path:    absPath,
		Kind:    kindFromSource(string(data), name),
		i:       i,
		exports: make(map[string]plugin.Symbol),
		values:  make(map[string]string),
	}

	s.mu.Lock()
	s.addons[name] = a
	delete(s.lastErr, name)
	s.mu.Unlock()
	return nil
}

// Unload removes a loaded addon by name.
func (s *Store) Unload(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.addons, name)
}

// Get returns a loaded addon or nil.
func (s *Store) Get(name string) *Addon {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.addons[name]
}

// List returns loaded addon names sorted.
func (s *Store) List() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]string, 0, len(s.addons))
	for n := range s.addons {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// ByKind returns loaded addons matching kind.
func (s *Store) ByKind(kind Kind) []*Addon {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*Addon
	for _, a := range s.addons {
		if a.Kind == kind {
			out = append(out, a)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Status is a JSON-friendly view of loaded addons and the last load errors.
type Status struct {
	Dir    string            `json:"dir"`
	Loaded []string          `json:"loaded"`
	Errors map[string]string `json:"errors,omitempty"`
	Kinds  map[string]string `json:"kinds,omitempty"`
}

// Status snapshots the store for the admin API / diagnostics.
func (s *Store) Status() Status {
	s.mu.RLock()
	defer s.mu.RUnlock()
	st := Status{
		Dir:    s.dir,
		Loaded: make([]string, 0, len(s.addons)),
		Kinds:  make(map[string]string, len(s.addons)),
		Errors: make(map[string]string, len(s.lastErr)),
	}
	for n := range s.addons {
		st.Loaded = append(st.Loaded, n)
	}
	sort.Strings(st.Loaded)
	for _, n := range st.Loaded {
		st.Kinds[n] = string(s.addons[n].Kind)
	}
	for k, v := range s.lastErr {
		st.Errors[k] = v
	}
	if len(st.Errors) == 0 {
		st.Errors = nil
	}
	return st
}

// Eval evaluates source inside the addon's interpreter and returns the value.
func (a *Addon) Eval(src string) (plugin.Symbol, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.i == nil {
		return nil, fmt.Errorf("addon %s: not loaded", a.Name)
	}
	return a.i.Eval(src)
}

// CallString looks up an exported func and calls it with string args,
// returning a string result when possible.
func (a *Addon) CallString(fn string, args ...any) (string, error) {
	vals, err := a.Call(fn, args...)
	if err != nil {
		return "", err
	}
	if len(vals) == 0 {
		return "", nil
	}
	if s, ok := vals[0].(string); ok {
		return s, nil
	}
	return fmt.Sprintf("%v", vals[0]), nil
}

// Call invokes a named export with the given arguments.
func (a *Addon) Call(fn string, args ...any) ([]any, error) {
	v, err := a.lookup(fn)
	if err != nil {
		return nil, err
	}
	return callSymbol(v, args...)
}

// Has reports whether the named export exists (func or value).
func (a *Addon) Has(name string) bool {
	_, err := a.lookup(name)
	return err == nil
}

// LookupValue evaluates a Go expression and returns the result as any.
func (a *Addon) LookupValue(expr string) (any, error) {
	v, err := a.Eval(expr)
	if err != nil {
		return nil, err
	}
	return v, nil
}

func (a *Addon) lookup(name string) (plugin.Symbol, error) {
	a.mu.Lock()
	if s, ok := a.exports[name]; ok {
		a.mu.Unlock()
		return s, nil
	}
	a.mu.Unlock()

	v, err := a.Eval(name)
	if err != nil {
		return nil, fmt.Errorf("addon %s: symbol %q: %w", a.Name, name, err)
	}
	a.mu.Lock()
	a.exports[name] = v
	a.mu.Unlock()
	return v, nil
}

// kindFromSource reads an optional first-line pragma: // addon-kind: theme
func kindFromSource(src, name string) Kind {
	for _, line := range strings.Split(src, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "//") {
			if line != "" && !strings.HasPrefix(line, "package ") {
				break
			}
			if strings.HasPrefix(line, "package ") {
				// keep scanning a few header lines for pragma
				continue
			}
			continue
		}
		if i := strings.Index(line, "addon-kind:"); i >= 0 {
			k := Kind(strings.ToLower(strings.TrimSpace(line[i+len("addon-kind:"):])))
			switch k {
			case KindTheme, KindPreset, KindUI, KindAuth, KindRuntime:
				return k
			}
		}
	}
	// Filename convention: theme-*.go, preset-*.go, …
	base := strings.ToLower(name)
	for _, k := range []Kind{KindTheme, KindPreset, KindUI, KindAuth, KindRuntime} {
		if strings.HasPrefix(base, string(k)+"-") || strings.HasPrefix(base, string(k)+"_") || base == string(k) {
			return k
		}
	}
	return KindRuntime
}

// Reload reloads a single file after an operator edit.
func (s *Store) Reload(name string) error {
	path := filepath.Join(s.Dir(), name+".go")
	return s.LoadFile(path)
}
