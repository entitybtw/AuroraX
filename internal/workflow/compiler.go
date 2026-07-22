package workflow

import (
	"errors"
	"net/http"

	"aurora/internal/core"
	"aurora/internal/guardrails"
)

type wfCompiler struct {
	registry    guardrails.Catalog
	featureCaps core.WorkflowFeatures
}

// NewCompiler creates the default workflow compiler for the v1 payload.
func NewCompiler(registry guardrails.Catalog) Compiler {
	return NewCompilerWithFeatureCaps(registry, core.DefaultWorkflowFeatures())
}

// NewCompilerWithFeatureCaps creates the default workflow compiler for the
// v1 payload with process-level feature caps applied at compile time.
func NewCompilerWithFeatureCaps(registry guardrails.Catalog, featureCaps core.WorkflowFeatures) Compiler {
	return &wfCompiler{
		registry:    registry,
		featureCaps: featureCaps,
	}
}

func (c *wfCompiler) Compile(version Version) (*CompiledWorkflow, error) {
	features := version.Payload.Features.activeFeatures().ApplyUpperBound(c.featureCaps)
	policy := &core.ResolvedWorkflowPolicy{
		VersionID:      version.ID,
		Version:        version.Version,
		ScopeProvider:  version.Scope.Provider,
		ScopeModel:     version.Scope.Model,
		ScopeUserPath:  version.Scope.UserPath,
		Name:           version.Name,
		WorkflowHash:   version.WorkflowHash,
		Features:       features,
		GuardrailsHash: "",
	}

	var inputPipeline *guardrails.Pipeline
	var outputPipeline *guardrails.Pipeline
	if policy.Features.Guardrails {
		steps := make([]guardrails.StepReference, 0, len(version.Payload.Guardrails))
		for _, step := range version.Payload.Guardrails {
			steps = append(steps, guardrails.StepReference{
				Ref:  step.Ref,
				Step: step.Step,
			})
		}

		var err error
		inputPipeline, outputPipeline, policy.GuardrailsHash, err = c.buildGuardrails(steps)
		if err != nil {
			return nil, err
		}
	}

	return &CompiledWorkflow{
		Version:        version,
		Policy:         policy,
		InputPipeline:  inputPipeline,
		OutputPipeline: outputPipeline,
		Pipeline:       inputPipeline,
	}, nil
}

func (c *wfCompiler) buildGuardrails(steps []guardrails.StepReference) (*guardrails.Pipeline, *guardrails.Pipeline, string, error) {
	if len(steps) == 0 {
		return nil, nil, "", nil
	}
	if c == nil || c.registry == nil {
		return nil, nil, "", core.NewProviderError("", http.StatusBadGateway, "guardrails are enabled but no guardrail registry is configured", nil)
	}
	if c.registry.Len() == 0 {
		return nil, nil, "", core.NewProviderError("", http.StatusBadGateway, "guardrails are enabled but no guardrails are loaded", nil)
	}
	inputPipeline, outputPipeline, hash, err := c.registry.BuildPipelines(steps)
	if err == nil {
		return inputPipeline, outputPipeline, hash, nil
	}
	if gatewayErr, ok := errors.AsType[*core.GatewayError](err); ok {
		return nil, nil, "", gatewayErr
	}
	return nil, nil, "", core.NewProviderError("", http.StatusBadGateway, "compile guardrails: "+err.Error(), err)
}
