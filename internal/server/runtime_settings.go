package server

import (
	"aurora/configuration"
	"aurora/internal/core"
	"aurora/internal/token_saver"
)

// SetKeepOnlyAliasesAtModelsEndpoint updates whether GET /v1/models hides concrete provider models.
func (s *Server) SetKeepOnlyAliasesAtModelsEndpoint(enabled bool) {
	if s == nil || s.handler == nil {
		return
	}
	s.handler.keepOnlyAliasesAtModelsEndpoint = enabled
}

// SetEnabledPassthroughProviders updates the allowed provider types for passthrough requests.
func (s *Server) SetEnabledPassthroughProviders(providerTypes []string) {
	if s == nil || s.handler == nil {
		return
	}
	s.handler.setEnabledPassthroughProviders(providerTypes)
}

// SetAllowPassthroughV1Alias updates whether /p/{provider}/v1/... aliases are normalized.
func (s *Server) SetAllowPassthroughV1Alias(enabled bool) {
	if s == nil || s.handler == nil {
		return
	}
	s.handler.normalizePassthroughV1Prefix = enabled
}

func promptCacheConfigFromConfig(cfg config.PromptCacheConfig) *core.PromptCacheConfig {
	return &core.PromptCacheConfig{
		Mode:                 core.PromptCacheMode(cfg.Mode),
		SystemPromptCache:    cfg.SystemPromptCache,
		FirstMessageCache:    cfg.FirstMessageCache,
		ToolsCache:           cfg.ToolsCache,
		MinTokensBeforeCache: cfg.MinTokensBeforeCache,
	}
}

// SetPromptCacheConfig updates the runtime prompt cache config.
func (s *Server) SetPromptCacheConfig(cfg config.PromptCacheConfig) {
	if s == nil || s.handler == nil {
		return
	}
	s.handler.promptCacheConfig = promptCacheConfigFromConfig(cfg)
	if s.handler.translatedSvc != nil {
		s.handler.translatedSvc.setPromptCacheConfig(promptCacheConfigFromConfig(cfg))
	}
}

// SetTokenSaver updates the runtime Token Saver service used by chat completions.
func (s *Server) SetTokenSaver(cfg config.TokenSaverConfig) {
	if s == nil || s.handler == nil {
		return
	}
	service := tokensaver.NewService(cfg)
	s.handler.tokenSaver = service
	if s.handler.translatedSvc != nil {
		s.handler.translatedSvc.setTokenSaver(service)
	}
}
