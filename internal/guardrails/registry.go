package guardrails

import (
	"fmt"
	"sort"
	"strings"

	"aurora/internal/response_cache"
)

// StepReference points to one named guardrail and the step it should run at.
type StepReference struct {
	Ref  string
	Step int
}

type registryEntry struct {
	guardrail  Guardrail
	direction  string
	descriptor responsecache.GuardrailRuleDescriptor
}

// Registry stores named guardrails so workflows can reference them by id.
type Registry struct {
	entries map[string]registryEntry
}

// NewRegistry creates an empty named guardrail registry.
func NewRegistry() *Registry {
	return &Registry{entries: make(map[string]registryEntry)}
}

// Len returns the number of registered named guardrails.
func (r *Registry) Len() int {
	if r == nil {
		return 0
	}
	return len(r.entries)
}

// Names returns the registered guardrail names in sorted order.
func (r *Registry) Names() []string {
	if r == nil || len(r.entries) == 0 {
		return []string{}
	}

	names := make([]string, 0, len(r.entries))
	for name := range r.entries {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Register adds one named guardrail and its hashing descriptor.
func (r *Registry) Register(g Guardrail, descriptor responsecache.GuardrailRuleDescriptor) error {
	return r.RegisterWithDirection(g, DirectionBoth, descriptor)
}

// RegisterWithDirection adds one named guardrail with its execution direction.
func (r *Registry) RegisterWithDirection(g Guardrail, direction string, descriptor responsecache.GuardrailRuleDescriptor) error {
	if r == nil {
		return fmt.Errorf("registry is required")
	}
	if g == nil {
		return fmt.Errorf("guardrail is required")
	}
	name := strings.TrimSpace(g.Name())
	if name == "" {
		return fmt.Errorf("guardrail name is required")
	}
	if _, exists := r.entries[name]; exists {
		return fmt.Errorf("duplicate guardrail registration: %q", name)
	}
	descriptor.Name = name
	descriptor.Direction = normalizeDirection(direction)
	r.entries[name] = registryEntry{
		guardrail:  g,
		direction:  normalizeDirection(direction),
		descriptor: descriptor,
	}
	return nil
}

// BuildPipeline resolves named guardrail references into an executable pipeline and hash.
func (r *Registry) BuildPipeline(steps []StepReference) (*Pipeline, string, error) {
	inputPipeline, _, hash, err := r.BuildPipelines(steps)
	return inputPipeline, hash, err
}

// BuildPipelines resolves named guardrail references into input and output pipelines.
func (r *Registry) BuildPipelines(steps []StepReference) (*Pipeline, *Pipeline, string, error) {
	if len(steps) == 0 {
		return nil, nil, "", nil
	}
	if r == nil {
		return nil, nil, "", fmt.Errorf("guardrail registry is required")
	}

	inputPipeline := NewPipeline()
	outputPipeline := NewPipeline()
	descriptors := make([]responsecache.GuardrailRuleDescriptor, 0, len(steps))
	for _, step := range steps {
		name := strings.TrimSpace(step.Ref)
		if name == "" {
			return nil, nil, "", fmt.Errorf("guardrail ref is required")
		}
		entry, ok := r.entries[name]
		if !ok {
			return nil, nil, "", fmt.Errorf("unknown guardrail ref: %q", name)
		}
		if directionIncludes(entry.direction, DirectionInput) {
			inputPipeline.Add(entry.guardrail, step.Step)
		}
		if directionIncludes(entry.direction, DirectionOutput) {
			outputPipeline.Add(entry.guardrail, step.Step)
		}
		descriptor := entry.descriptor
		descriptor.Order = step.Step
		descriptors = append(descriptors, descriptor)
	}
	if inputPipeline.Len() == 0 {
		inputPipeline = nil
	}
	if outputPipeline.Len() == 0 {
		outputPipeline = nil
	}
	return inputPipeline, outputPipeline, responsecache.ComputeGuardrailsHash(descriptors), nil
}
