package core

import (
	"context"
	"time"
)

// contextKey is a custom type for context keys to avoid collisions.
type contextKey string

const (
	requestCarrierKey            contextKey = "request-carrier"
	requestIDKey                 contextKey = "request-id"
	requestSnapshotKey           contextKey = "request-snapshot"
	whiteBoxPromptKey            contextKey = "white-box-prompt"
	workflowKey                  contextKey = "workflow"
	authKeyIDKey                 contextKey = "auth-key-id"
	authKeyRateLimitsKey         contextKey = "auth-key-rate-limits"
	authKeyAccessPolicyKey       contextKey = "auth-key-access-policy"
	providerKeyIDKey             contextKey = "provider-key-id"
	effectiveUserPathKey         contextKey = "effective-user-path"
	batchPreparationMetadataKey  contextKey = "batch-preparation-metadata"
	enforceReturningUsageDataKey contextKey = "enforce-returning-usage-data"
	guardrailsHashKey            contextKey = "guardrails-hash"
	fallbackUsedKey              contextKey = "fallback-used"
	requestOriginKey             contextKey = "request-origin"
	identityUserIDKey            contextKey = "identity-user-id"
	identityEmailKey             contextKey = "identity-email"
	identityRoleIDsKey           contextKey = "identity-role-ids"
	identityAuthMethodKey        contextKey = "identity-auth-method"
)

// RequestCarrier consolidates all per-request context values into a single
// struct stored as one context value. Mutating its fields is allocation-free
// after the carrier is created, eliminating the 12+ context.WithValue + 20+
// http.Request.WithContext calls per request.
type RequestCarrier struct {
	RequestID                 string
	Snapshot                  *RequestSnapshot
	WhiteBoxPrompt            *WhiteBoxPrompt
	Workflow                  *Workflow
	AuthKeyID                 string
	AuthKeyRateLimits         AuthKeyRateLimits
	AuthKeyRateLimitsSet      bool
	AuthKeyAccessPolicy       AuthKeyAccessPolicy
	AuthKeyAccessPolicySet    bool
	ProviderKeyID             string
	EffectiveUserPath         string
	BatchPreparationMetadata  *BatchPreparationMetadata
	EnforceReturningUsageData bool
	GuardrailsHash            string
	FallbackUsed              bool
	RequestOrigin             RequestOrigin
	IdentityUserID            string
	IdentityEmail             string
	IdentityRoleIDs           []string
	IdentityAuthMethod        AuthMethod
}

func GetOrCreateCarrier(ctx context.Context) (*RequestCarrier, context.Context) {
	if c, ok := ctx.Value(requestCarrierKey).(*RequestCarrier); ok {
		return c, ctx
	}
	c := &RequestCarrier{}
	return c, context.WithValue(ctx, requestCarrierKey, c)
}

func getCarrier(ctx context.Context) *RequestCarrier {
	if c, ok := ctx.Value(requestCarrierKey).(*RequestCarrier); ok {
		return c
	}
	return nil
}

// AuthKeyRateLimits defines optional per-key request and token limits.
type AuthKeyRateLimits struct {
	RequestsPerMinute int `json:"requests_per_minute,omitempty" bson:"requests_per_minute,omitempty"`
	RequestsPerDay    int `json:"requests_per_day,omitempty" bson:"requests_per_day,omitempty"`
	TokensPerMinute   int `json:"tokens_per_minute,omitempty" bson:"tokens_per_minute,omitempty"`
	TokensPerDay      int `json:"tokens_per_day,omitempty" bson:"tokens_per_day,omitempty"`
}

// AuthKeyAccessPolicy defines request-scoped provider/model restrictions for a managed auth key.
type AuthKeyAccessPolicy struct {
	AllowedProviders []string `json:"allowed_providers,omitempty" bson:"allowed_providers,omitempty"`
	AllowedModels    []string `json:"allowed_models,omitempty" bson:"allowed_models,omitempty"`
	DeniedModels     []string `json:"denied_models,omitempty" bson:"denied_models,omitempty"`
	ProviderPoolID   *string  `json:"provider_pool_id,omitempty" bson:"provider_pool_id,omitempty"`
}

// Empty reports whether no access restriction is configured.
func (p AuthKeyAccessPolicy) Empty() bool {
	return p.AllowedProviders == nil && p.AllowedModels == nil && len(p.DeniedModels) == 0 && p.ProviderPoolID == nil
}

// Empty reports whether no auth key limit is configured.
func (l AuthKeyRateLimits) Empty() bool {
	return l.RequestsPerMinute == 0 && l.RequestsPerDay == 0 && l.TokensPerMinute == 0 && l.TokensPerDay == 0
}

// AuthKeyRateLimitWindow describes one rate-limit window (e.g. requests_per_minute)
// for a managed key — how much has been consumed and when it resets.
type AuthKeyRateLimitWindow struct {
	Scope     string    `json:"scope"`
	Limit     int       `json:"limit"`
	Used      int       `json:"used"`
	Remaining int       `json:"remaining"`
	ResetAt   time.Time `json:"reset_at"`
}

// AuthKeyRateLimitSnapshot is the live view of one key's rate-limit state.
type AuthKeyRateLimitSnapshot struct {
	RequestsPerMinute *AuthKeyRateLimitWindow `json:"requests_per_minute,omitempty"`
	RequestsPerDay    *AuthKeyRateLimitWindow `json:"requests_per_day,omitempty"`
	TokensPerMinute   *AuthKeyRateLimitWindow `json:"tokens_per_minute,omitempty"`
	TokensPerDay      *AuthKeyRateLimitWindow `json:"tokens_per_day,omitempty"`
}

// RequestOrigin identifies whether a request came from an external caller or an
// internal gateway-owned workflow.
type RequestOrigin string

const (
	RequestOriginExternal  RequestOrigin = "external"
	RequestOriginGuardrail RequestOrigin = "guardrail"
)

// WithRequestID returns a new context with the request ID attached.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	if c := getCarrier(ctx); c != nil {
		c.RequestID = requestID
		return ctx
	}
	return context.WithValue(ctx, requestIDKey, requestID)
}

// GetRequestID retrieves the request ID from the context.
func GetRequestID(ctx context.Context) string {
	if c := getCarrier(ctx); c != nil {
		return c.RequestID
	}
	if v := ctx.Value(requestIDKey); v != nil {
		if id, ok := v.(string); ok {
			return id
		}
	}
	return ""
}

// WithRequestSnapshot returns a new context with the request snapshot attached.
func WithRequestSnapshot(ctx context.Context, snapshot *RequestSnapshot) context.Context {
	if c := getCarrier(ctx); c != nil {
		c.Snapshot = snapshot
		return ctx
	}
	return context.WithValue(ctx, requestSnapshotKey, snapshot)
}

// GetRequestSnapshot retrieves the request snapshot from the context.
func GetRequestSnapshot(ctx context.Context) *RequestSnapshot {
	if c := getCarrier(ctx); c != nil {
		return c.Snapshot
	}
	if v := ctx.Value(requestSnapshotKey); v != nil {
		if snapshot, ok := v.(*RequestSnapshot); ok {
			return snapshot
		}
	}
	return nil
}

// WithWhiteBoxPrompt returns a new context with the white-box prompt attached.
func WithWhiteBoxPrompt(ctx context.Context, prompt *WhiteBoxPrompt) context.Context {
	if c := getCarrier(ctx); c != nil {
		c.WhiteBoxPrompt = prompt
		return ctx
	}
	return context.WithValue(ctx, whiteBoxPromptKey, prompt)
}

// GetWhiteBoxPrompt retrieves the white-box prompt from the context.
func GetWhiteBoxPrompt(ctx context.Context) *WhiteBoxPrompt {
	if c := getCarrier(ctx); c != nil {
		return c.WhiteBoxPrompt
	}
	if v := ctx.Value(whiteBoxPromptKey); v != nil {
		if prompt, ok := v.(*WhiteBoxPrompt); ok {
			return prompt
		}
	}
	return nil
}

// WithWorkflow returns a new context with the workflow attached.
func WithWorkflow(ctx context.Context, workflow *Workflow) context.Context {
	if c := getCarrier(ctx); c != nil {
		c.Workflow = workflow
		return ctx
	}
	return context.WithValue(ctx, workflowKey, workflow)
}

// GetWorkflow retrieves the workflow from the context.
func GetWorkflow(ctx context.Context) *Workflow {
	if c := getCarrier(ctx); c != nil {
		return c.Workflow
	}
	if v := ctx.Value(workflowKey); v != nil {
		if workflow, ok := v.(*Workflow); ok {
			return workflow
		}
	}
	return nil
}

// WithAuthKeyID returns a new context with the authenticated managed auth key id attached.
func WithAuthKeyID(ctx context.Context, id string) context.Context {
	if c := getCarrier(ctx); c != nil {
		c.AuthKeyID = id
		return ctx
	}
	return context.WithValue(ctx, authKeyIDKey, id)
}

// GetAuthKeyID retrieves the managed auth key id from the context.
func GetAuthKeyID(ctx context.Context) string {
	if c := getCarrier(ctx); c != nil {
		return c.AuthKeyID
	}
	if v := ctx.Value(authKeyIDKey); v != nil {
		if id, ok := v.(string); ok {
			return id
		}
	}
	return ""
}

// WithAuthKeyRateLimits returns a new context with managed auth key limits attached.
func WithAuthKeyRateLimits(ctx context.Context, limits AuthKeyRateLimits) context.Context {
	if c := getCarrier(ctx); c != nil {
		c.AuthKeyRateLimits = limits
		c.AuthKeyRateLimitsSet = true
		return ctx
	}
	return context.WithValue(ctx, authKeyRateLimitsKey, limits)
}

// GetAuthKeyRateLimits retrieves managed auth key limits from context.
func GetAuthKeyRateLimits(ctx context.Context) (AuthKeyRateLimits, bool) {
	if c := getCarrier(ctx); c != nil {
		return c.AuthKeyRateLimits, c.AuthKeyRateLimitsSet
	}
	if v := ctx.Value(authKeyRateLimitsKey); v != nil {
		if limits, ok := v.(AuthKeyRateLimits); ok {
			return limits, true
		}
	}
	return AuthKeyRateLimits{}, false
}

// WithAuthKeyAccessPolicy returns a new context with managed auth key access restrictions attached.
func WithAuthKeyAccessPolicy(ctx context.Context, policy AuthKeyAccessPolicy) context.Context {
	if c := getCarrier(ctx); c != nil {
		c.AuthKeyAccessPolicy = policy
		c.AuthKeyAccessPolicySet = true
		return ctx
	}
	return context.WithValue(ctx, authKeyAccessPolicyKey, policy)
}

// GetAuthKeyAccessPolicy retrieves managed auth key access restrictions from context.
func GetAuthKeyAccessPolicy(ctx context.Context) (AuthKeyAccessPolicy, bool) {
	if c := getCarrier(ctx); c != nil {
		return c.AuthKeyAccessPolicy, c.AuthKeyAccessPolicySet
	}
	if v := ctx.Value(authKeyAccessPolicyKey); v != nil {
		if policy, ok := v.(AuthKeyAccessPolicy); ok {
			return policy, true
		}
	}
	return AuthKeyAccessPolicy{}, false
}

// WithProviderKeyID returns a new context with a configured provider instance override attached.
func WithProviderKeyID(ctx context.Context, keyID string) context.Context {
	if c := getCarrier(ctx); c != nil {
		c.ProviderKeyID = keyID
		return ctx
	}
	return context.WithValue(ctx, providerKeyIDKey, keyID)
}

// GetProviderKeyID retrieves a configured provider instance override from context.
func GetProviderKeyID(ctx context.Context) string {
	if c := getCarrier(ctx); c != nil {
		return c.ProviderKeyID
	}
	if v := ctx.Value(providerKeyIDKey); v != nil {
		if keyID, ok := v.(string); ok {
			return keyID
		}
	}
	return ""
}

// WithEffectiveUserPath returns a new context with an effective user path override attached.
func WithEffectiveUserPath(ctx context.Context, userPath string) context.Context {
	if c := getCarrier(ctx); c != nil {
		c.EffectiveUserPath = userPath
		return ctx
	}
	return context.WithValue(ctx, effectiveUserPathKey, userPath)
}

// GetEffectiveUserPath retrieves the effective user path override from context.
func GetEffectiveUserPath(ctx context.Context) string {
	if c := getCarrier(ctx); c != nil {
		return c.EffectiveUserPath
	}
	if v := ctx.Value(effectiveUserPathKey); v != nil {
		if userPath, ok := v.(string); ok {
			return userPath
		}
	}
	return ""
}

// WithBatchPreparationMetadata returns a new context with batch preprocessing metadata attached.
func WithBatchPreparationMetadata(ctx context.Context, metadata *BatchPreparationMetadata) context.Context {
	if c := getCarrier(ctx); c != nil {
		c.BatchPreparationMetadata = metadata
		return ctx
	}
	return context.WithValue(ctx, batchPreparationMetadataKey, metadata)
}

// GetBatchPreparationMetadata retrieves batch preprocessing metadata from the context.
func GetBatchPreparationMetadata(ctx context.Context) *BatchPreparationMetadata {
	if c := getCarrier(ctx); c != nil {
		return c.BatchPreparationMetadata
	}
	if v := ctx.Value(batchPreparationMetadataKey); v != nil {
		if metadata, ok := v.(*BatchPreparationMetadata); ok {
			return metadata
		}
	}
	return nil
}

// WithEnforceReturningUsageData returns a new context with the streaming usage policy attached.
func WithEnforceReturningUsageData(ctx context.Context, enforce bool) context.Context {
	if c := getCarrier(ctx); c != nil {
		c.EnforceReturningUsageData = enforce
		return ctx
	}
	return context.WithValue(ctx, enforceReturningUsageDataKey, enforce)
}

// GetEnforceReturningUsageData reports whether the request should ask providers
// to include usage in streaming responses when possible.
func GetEnforceReturningUsageData(ctx context.Context) bool {
	if c := getCarrier(ctx); c != nil {
		return c.EnforceReturningUsageData
	}
	if v := ctx.Value(enforceReturningUsageDataKey); v != nil {
		if enforce, ok := v.(bool); ok {
			return enforce
		}
	}
	return false
}

// WithGuardrailsHash returns a new context with the guardrails hash attached.
func WithGuardrailsHash(ctx context.Context, hash string) context.Context {
	if c := getCarrier(ctx); c != nil {
		c.GuardrailsHash = hash
		return ctx
	}
	return context.WithValue(ctx, guardrailsHashKey, hash)
}

// GetGuardrailsHash retrieves the guardrails hash from the context.
func GetGuardrailsHash(ctx context.Context) string {
	if c := getCarrier(ctx); c != nil {
		return c.GuardrailsHash
	}
	if v := ctx.Value(guardrailsHashKey); v != nil {
		if h, ok := v.(string); ok {
			return h
		}
	}
	return ""
}

// WithFallbackUsed returns a new context marked as having used a fallback model.
func WithFallbackUsed(ctx context.Context) context.Context {
	if c := getCarrier(ctx); c != nil {
		c.FallbackUsed = true
		return ctx
	}
	return context.WithValue(ctx, fallbackUsedKey, true)
}

// GetFallbackUsed reports whether the request was served by a fallback model.
func GetFallbackUsed(ctx context.Context) bool {
	if c := getCarrier(ctx); c != nil {
		return c.FallbackUsed
	}
	if v := ctx.Value(fallbackUsedKey); v != nil {
		if used, ok := v.(bool); ok {
			return used
		}
	}
	return false
}

// WithRequestOrigin returns a new context with the logical request origin attached.
func WithRequestOrigin(ctx context.Context, origin RequestOrigin) context.Context {
	if c := getCarrier(ctx); c != nil {
		c.RequestOrigin = origin
		return ctx
	}
	return context.WithValue(ctx, requestOriginKey, origin)
}

// WithIdentityUserID returns a new context with the authenticated identity user ID attached.
func WithIdentityUserID(ctx context.Context, userID string) context.Context {
	if c := getCarrier(ctx); c != nil {
		c.IdentityUserID = userID
		return ctx
	}
	return context.WithValue(ctx, identityUserIDKey, userID)
}

// GetIdentityUserID retrieves the authenticated identity user ID from context.
func GetIdentityUserID(ctx context.Context) string {
	if c := getCarrier(ctx); c != nil {
		return c.IdentityUserID
	}
	if v := ctx.Value(identityUserIDKey); v != nil {
		if id, ok := v.(string); ok {
			return id
		}
	}
	return ""
}

// WithIdentityEmail returns a new context with the authenticated user email attached.
func WithIdentityEmail(ctx context.Context, email string) context.Context {
	if c := getCarrier(ctx); c != nil {
		c.IdentityEmail = email
		return ctx
	}
	return context.WithValue(ctx, identityEmailKey, email)
}

// GetIdentityEmail retrieves the authenticated user email from context.
func GetIdentityEmail(ctx context.Context) string {
	if c := getCarrier(ctx); c != nil {
		return c.IdentityEmail
	}
	if v := ctx.Value(identityEmailKey); v != nil {
		if email, ok := v.(string); ok {
			return email
		}
	}
	return ""
}

// WithIdentityRoleIDs returns a new context with the authenticated user's role IDs attached.
func WithIdentityRoleIDs(ctx context.Context, roleIDs []string) context.Context {
	if c := getCarrier(ctx); c != nil {
		c.IdentityRoleIDs = roleIDs
		return ctx
	}
	return context.WithValue(ctx, identityRoleIDsKey, roleIDs)
}

// GetIdentityRoleIDs retrieves the authenticated user's role IDs from context.
func GetIdentityRoleIDs(ctx context.Context) []string {
	if c := getCarrier(ctx); c != nil {
		return c.IdentityRoleIDs
	}
	if v := ctx.Value(identityRoleIDsKey); v != nil {
		if ids, ok := v.([]string); ok {
			return ids
		}
	}
	return nil
}

// AuthMethod describes how a request was authenticated.
type AuthMethod string

const (
	AuthMethodNone       AuthMethod = "none"
	AuthMethodMasterKey  AuthMethod = "master_key"
	AuthMethodJWT        AuthMethod = "jwt"
	AuthMethodManagedKey AuthMethod = "managed_key"
)

// WithIdentityAuthMethod returns a new context with the auth method attached.
func WithIdentityAuthMethod(ctx context.Context, method AuthMethod) context.Context {
	if c := getCarrier(ctx); c != nil {
		c.IdentityAuthMethod = method
		return ctx
	}
	return context.WithValue(ctx, identityAuthMethodKey, method)
}

// IdentityUserInfo holds the validated identity user details extracted from a JWT.
type IdentityUserInfo struct {
	UserID  string   `json:"user_id"`
	Email   string   `json:"email"`
	RoleIDs []string `json:"role_ids"`
}

// GetIdentityAuthMethod retrieves the auth method from context.
func GetIdentityAuthMethod(ctx context.Context) AuthMethod {
	if c := getCarrier(ctx); c != nil {
		return c.IdentityAuthMethod
	}
	if v := ctx.Value(identityAuthMethodKey); v != nil {
		if m, ok := v.(AuthMethod); ok {
			return m
		}
	}
	return AuthMethodNone
}

// GetRequestOrigin retrieves the request origin from context.
func GetRequestOrigin(ctx context.Context) RequestOrigin {
	if c := getCarrier(ctx); c != nil {
		if c.RequestOrigin != "" {
			return c.RequestOrigin
		}
		return RequestOriginExternal
	}
	if v := ctx.Value(requestOriginKey); v != nil {
		if origin, ok := v.(RequestOrigin); ok && origin != "" {
			return origin
		}
	}
	return RequestOriginExternal
}
