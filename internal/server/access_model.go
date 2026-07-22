package server

import "aurora/internal/gateway"

// RequestModelAuthorizer validates request-scoped access to concrete models.
type RequestModelAuthorizer = gateway.ModelAuthorizer
