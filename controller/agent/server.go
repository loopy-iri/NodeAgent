// Package agent exposes the node's control surface over HTTP/JSON with two-level
// auth: master scope (the main panel, via the master key) and tenant scope (a
// customer, via its API key). It wires the tenant.Registry and shared.Manager
// together and runs the background enforcement loop.
//
// Auth uses the X-API-Key header. The master key is matched first; any other key
// is resolved to a tenant and quota-enforced. TLS termination is expected to be
// provided in production (see PRODUCTION_PLAN.md section 4).
package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/pasarguard/node/shared"
	"github.com/pasarguard/node/tenant"
)

// Server holds the agent's dependencies.
type Server struct {
	reg   *tenant.Registry
	mgr   *shared.Manager
	authn *tenant.Authenticator
}

func NewServer(reg *tenant.Registry, mgr *shared.Manager, authn *tenant.Authenticator) *Server {
	return &Server{reg: reg, mgr: mgr, authn: authn}
}

// Router builds the agent HTTP handler.
func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	r.Get("/health", s.handleHealth)

	// Master scope: the main panel administers the node.
	r.Group(func(r chi.Router) {
		r.Use(s.authMiddleware)
		r.Use(s.requireScope(tenant.ScopeMaster))

		r.Post("/admin/config", s.applyConfig)
		r.Get("/admin/tenants", s.listTenants)
		r.Post("/admin/tenants", s.createTenant)
		r.Delete("/admin/tenants/{id}", s.deleteTenant)
		r.Patch("/admin/tenants/{id}/quota", s.setQuota)
		r.Post("/admin/tenants/{id}/suspend", s.suspendTenant)
		r.Post("/admin/tenants/{id}/resume", s.resumeTenant)
		r.Post("/admin/tenants/{id}/reset", s.resetTenant)
		r.Get("/admin/tenants/{id}/usage", s.tenantUsage)
	})

	// Tenant scope: a customer manages its own users and reads its usage.
	r.Group(func(r chi.Router) {
		r.Use(s.authMiddleware)
		r.Use(s.requireScope(tenant.ScopeTenant))

		r.Put("/tenant/users", s.syncUsers)
		r.Get("/tenant/me", s.myUsage)
	})

	return r
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":       "ok",
		"core_started": s.mgr.Started(),
		"core_version": s.mgr.Version(),
	})
}

// --- helpers ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func decodeJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

// tenantIDFromContext returns the authenticated tenant id (tenant scope only).
func tenantIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(ctxTenantID).(string)
	return id
}
