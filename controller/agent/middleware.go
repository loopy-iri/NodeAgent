package agent

import (
	"context"
	"net/http"
	"time"

	"github.com/pasarguard/node/tenant"
)

type ctxKey int

const (
	ctxScope ctxKey = iota
	ctxTenantID
)

// authMiddleware resolves the X-API-Key header to a scope. For tenant scope it
// also applies enforcement (suspended/expired/quota) and stores the tenant id.
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("X-API-Key")
		if key == "" {
			writeError(w, http.StatusUnauthorized, "missing X-API-Key header")
			return
		}

		scope, decision := s.authn.Authenticate(key, time.Now().Unix())
		ctx := context.WithValue(r.Context(), ctxScope, scope)

		if scope == tenant.ScopeTenant {
			if !decision.Allowed {
				writeDecision(w, decision)
				return
			}
			ctx = context.WithValue(ctx, ctxTenantID, decision.Tenant.ID)
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// requireScope rejects requests whose resolved scope does not match.
func (s *Server) requireScope(want tenant.Scope) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got, _ := r.Context().Value(ctxScope).(tenant.Scope)
			if got != want {
				writeError(w, http.StatusForbidden, "insufficient scope")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// writeDecision maps a tenant authorization decision to an HTTP status.
func writeDecision(w http.ResponseWriter, d tenant.Decision) {
	switch d.Code {
	case tenant.CodeUnauthenticated:
		writeError(w, http.StatusUnauthorized, "invalid api key")
	case tenant.CodeResourceExhausted:
		// 402 Payment Required fits quota exhaustion in a billing context.
		writeJSON(w, http.StatusPaymentRequired, map[string]string{
			"error":  "quota exhausted",
			"reason": string(d.Reason),
		})
	case tenant.CodePermissionDenied:
		writeJSON(w, http.StatusForbidden, map[string]string{
			"error":  "access denied",
			"reason": string(d.Reason),
		})
	default:
		writeError(w, http.StatusForbidden, "access denied")
	}
}
