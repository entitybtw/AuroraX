package server

import (
	"context"
	"crypto/subtle"
	"errors"
	"strings"

	"github.com/labstack/echo/v5"

	"aurora/internal/audit_logging"
	"aurora/internal/authentication_keys"
	"aurora/internal/authorization_scope"
	"aurora/internal/core"
)

type BearerTokenAuthenticator interface {
	Enabled() bool
	Authenticate(ctx context.Context, token string) (authkeys.AuthenticationResult, error)
}

type IdentitySessionValidator interface {
	Enabled() bool
	ValidateSession(ctx context.Context, accessToken string) (userID, email, tenantID string, roleIDs []string, err error)
	CheckPermission(ctx context.Context, userID, action, resource string) (bool, error)
}

type TenantStatusValidator interface {
	Get(ctx context.Context, id string) (scope.Tenant, error)
}

func AuthMiddleware(masterKey string, skipPaths []string) echo.MiddlewareFunc {
	return AuthMiddlewareWithAuthenticator(masterKey, nil, skipPaths)
}

func AuthMiddlewareWithAuthenticator(masterKey string, authenticator BearerTokenAuthenticator, skipPaths []string) echo.MiddlewareFunc {
	return AuthMiddlewareWithFullConfig(masterKey, authenticator, nil, skipPaths)
}

func AuthMiddlewareWithFullConfig(masterKey string, authenticator BearerTokenAuthenticator, identityValidator IdentitySessionValidator, skipPaths []string) echo.MiddlewareFunc {
	return AuthMiddlewareWithFullConfigAndTenants(masterKey, authenticator, identityValidator, nil, skipPaths)
}

func AuthMiddlewareWithFullConfigAndTenants(masterKey string, authenticator BearerTokenAuthenticator, identityValidator IdentitySessionValidator, tenantValidator TenantStatusValidator, skipPaths []string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			identityEnabled := identityValidator != nil && identityValidator.Enabled()
			authConfigured := masterKey != "" || (authenticator != nil && authenticator.Enabled()) || identityEnabled

			if !authConfigured {
				auditlog.EnrichEntryWithAuthMethod(c, auditlog.AuthMethodNoKey)
				return next(c)
			}

			requestPath := c.Request().URL.Path
			for _, skipPath := range skipPaths {
				if strings.HasSuffix(skipPath, "/*") {
					prefix := strings.TrimSuffix(skipPath, "*")
					if strings.HasPrefix(requestPath, prefix) {
						auditlog.EnrichEntryWithAuthMethod(c, auditlog.AuthMethodNoKey)
						return next(c)
					}
				} else if requestPath == skipPath {
					auditlog.EnrichEntryWithAuthMethod(c, auditlog.AuthMethodNoKey)
					return next(c)
				}
			}

			token := ""
			authHeader := c.Request().Header.Get("Authorization")
			if authHeader != "" {
				const bearerPrefix = "Bearer "
				if !strings.HasPrefix(authHeader, bearerPrefix) {
					authErr := authenticationError(c, "invalid authorization header format, expected 'Bearer <token>'")
					return c.JSON(authErr.HTTPStatusCode(), authErr.ToJSON())
				}
				token = strings.TrimPrefix(authHeader, bearerPrefix)
			} else {
				token = c.Request().Header.Get("x-api-key")
			}

			if token == "" {
				authErr := authenticationError(c, "missing authorization header")
				return c.JSON(authErr.HTTPStatusCode(), authErr.ToJSON())
			}

			if masterKey != "" && subtle.ConstantTimeCompare([]byte(token), []byte(masterKey)) == 1 {
				auditlog.EnrichEntryWithAuthMethod(c, auditlog.AuthMethodMasterKey)
				ctx := c.Request().Context()
				ctx = core.WithIdentityAuthMethod(ctx, core.AuthMethodMasterKey)
				c.SetRequest(c.Request().WithContext(contextWithProviderKeyOverride(ctx, c)))
				return next(c)
			}

			if identityEnabled {
				userID, email, identityTenantID, roleIDs, vErr := identityValidator.ValidateSession(c.Request().Context(), token)
				if vErr == nil && userID != "" {
					ctx := c.Request().Context()
					ctx = scope.WithTenantID(ctx, identityTenantID)
					ctx = core.WithIdentityUserID(ctx, userID)
					ctx = core.WithIdentityEmail(ctx, email)
					ctx = core.WithIdentityRoleIDs(ctx, roleIDs)
					ctx = core.WithIdentityAuthMethod(ctx, core.AuthMethodJWT)
					c.SetRequest(c.Request().WithContext(contextWithProviderKeyOverride(ctx, c)))

					if isAdminAPIPath(requestPath) {
						action, resource, needsPermission := PermissionForRoute(c.Request().Method, requestPath)
						if needsPermission {
							allowed, pErr := identityValidator.CheckPermission(ctx, userID, action, resource)
							if pErr != nil || !allowed {
								if pErr != nil {
									auditlog.EnrichEntryWithError(c, "permission_check_error", pErr.Error())
								}
								authErr := authorizationError(c, "insufficient permissions")
								return c.JSON(authErr.HTTPStatusCode(), authErr.ToJSON())
							}
						}
					}

					return next(c)
				}

				if isAdminAPIPath(requestPath) {
					authErr := authenticationError(c, "invalid or expired session token")
					return c.JSON(authErr.HTTPStatusCode(), authErr.ToJSON())
				}
			}

			if isAdminAPIPath(requestPath) {
				authErr := authenticationError(c, "admin API requires valid session or master key")
				return c.JSON(authErr.HTTPStatusCode(), authErr.ToJSON())
			}

			if authenticator != nil && authenticator.Enabled() {
				auditlog.EnrichEntryWithAuthMethod(c, auditlog.AuthMethodAPIKey)
				authResult, err := authenticator.Authenticate(c.Request().Context(), token)
				if err == nil {
					if err := validateActiveTenant(c.Request().Context(), tenantValidator, authResult.TenantID); err != nil {
						authErr := authenticationErrorWithAudit(c, "authentication failed", err.Error())
						return c.JSON(authErr.HTTPStatusCode(), authErr.ToJSON())
					}
					ctx := core.WithAuthKeyID(c.Request().Context(), authResult.ID)
					ctx = scope.WithTenantID(ctx, authResult.TenantID)
					if !authResult.RateLimits.Empty() {
						ctx = core.WithAuthKeyRateLimits(ctx, authResult.RateLimits)
					}
					accessPolicy := core.AuthKeyAccessPolicy{
						AllowedProviders: authResult.AllowedProviders,
						AllowedModels:    authResult.AllowedModels,
						DeniedModels:     authResult.DeniedModels,
						ProviderPoolID:   authResult.ProviderPoolID,
					}
					if !accessPolicy.Empty() {
						ctx = core.WithAuthKeyAccessPolicy(ctx, accessPolicy)
					}
					if userPath := strings.TrimSpace(authResult.UserPath); userPath != "" {
						ctx = core.WithEffectiveUserPath(ctx, userPath)
						if snapshot := core.GetRequestSnapshot(ctx); snapshot != nil {
							ctx = core.WithRequestSnapshot(ctx, snapshot.WithUserPath(userPath))
						}
						c.Request().Header.Set(core.UserPathHeader, userPath)
						auditlog.EnrichEntryWithUserPath(c, userPath)
					}
					ctx = contextWithProviderKeyOverride(ctx, c)
					c.SetRequest(c.Request().WithContext(ctx))
					auditlog.EnrichEntryWithAuthKeyID(c, authResult.ID)
					return next(c)
				}

				authErr := authenticationErrorWithAudit(c, authFailureMessage(err), "authentication failed")
				return c.JSON(authErr.HTTPStatusCode(), authErr.ToJSON())
			}

			authErr := authenticationError(c, "invalid master key or session")
			return c.JSON(authErr.HTTPStatusCode(), authErr.ToJSON())
		}
	}
}

func contextWithProviderKeyOverride(ctx context.Context, c *echo.Context) context.Context {
	if c == nil || c.Request() == nil || core.GetIdentityAuthMethod(ctx) != core.AuthMethodMasterKey {
		return ctx
	}
	keyID := strings.TrimSpace(c.Request().Header.Get("X-Provider-Key-Id"))
	if keyID == "" {
		keyID = strings.TrimSpace(c.Request().Header.Get("X-Bf-Api-Key-Id"))
	}
	if keyID == "" {
		keyID = strings.TrimSpace(c.Request().Header.Get("X-Bf-Api-Key"))
	}
	if keyID == "" {
		return ctx
	}
	return core.WithProviderKeyID(ctx, keyID)
}

func isAdminAPIPath(path string) bool {
	return path == "/admin/api/v1" || strings.HasPrefix(path, "/admin/api/v1/")
}

func authFailureMessage(err error) string {
	if err == nil {
		return "invalid API key"
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return "authentication unavailable"
	}
	message := strings.TrimSpace(err.Error())
	if message == "" {
		return "invalid API key"
	}
	return message
}

func authenticationError(c *echo.Context, message string) *core.GatewayError {
	auditlog.EnrichEntryWithError(c, string(core.ErrorTypeAuthentication), message)
	return core.NewAuthenticationError("", message)
}

func authenticationErrorWithAudit(c *echo.Context, auditMessage, responseMessage string) *core.GatewayError {
	auditlog.EnrichEntryWithError(c, string(core.ErrorTypeAuthentication), auditMessage)
	return core.NewAuthenticationError("", responseMessage)
}

func authorizationError(c *echo.Context, message string) *core.GatewayError {
	auditlog.EnrichEntryWithError(c, string(core.ErrorTypeAuthorization), message)
	return core.NewAuthorizationError("", message)
}
