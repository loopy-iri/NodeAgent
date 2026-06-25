package tenant

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
)

// HashKey returns the lowercase hex SHA-256 of an API key. Keys are only ever
// stored and compared as hashes.
func HashKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

// Scope is the privilege level resolved for a request.
type Scope int

const (
	ScopeNone   Scope = iota
	ScopeMaster       // the main panel: full administrative control
	ScopeTenant       // a customer: limited to its own tenant, quota-enforced
)

// Code is a transport-agnostic authorization result code. The gRPC/REST layer
// maps these to status codes.
type Code string

const (
	CodeOK                Code = "ok"
	CodeUnauthenticated   Code = "unauthenticated"
	CodePermissionDenied  Code = "permission_denied"
	CodeResourceExhausted Code = "resource_exhausted"
)

// Decision is the outcome of authorizing a tenant request.
type Decision struct {
	Allowed bool
	Code    Code
	Reason  Reason
	Tenant  *Tenant
}

// Authorize resolves a customer key to a tenant and applies enforcement
// (status, expiry, quota+credit). It is read-only; status transitions are done
// by Enforce.
func (r *Registry) Authorize(apiKey string, now int64) Decision {
	hash := HashKey(apiKey)

	r.mu.RLock()
	defer r.mu.RUnlock()

	id, ok := r.byKey[hash]
	if !ok {
		return Decision{Code: CodeUnauthenticated}
	}
	t, ok := r.byID[id]
	if !ok {
		return Decision{Code: CodeUnauthenticated}
	}

	switch {
	case t.Status == StatusDeleted:
		return Decision{Code: CodeUnauthenticated, Tenant: t.clone()}
	case t.Status == StatusSuspended:
		return Decision{Code: CodePermissionDenied, Reason: t.Reason, Tenant: t.clone()}
	case t.ExpireAt > 0 && now > t.ExpireAt:
		return Decision{Code: CodePermissionDenied, Reason: ReasonExpired, Tenant: t.clone()}
	case t.UsedBytes >= t.effectiveLimit():
		return Decision{Code: CodeResourceExhausted, Reason: ReasonQuota, Tenant: t.clone()}
	default:
		return Decision{Allowed: true, Code: CodeOK, Tenant: t.clone()}
	}
}

// Authenticator resolves a raw API key to a scope. The master key (used by the
// main panel) is matched in constant time; everything else is treated as a
// tenant key and authorized through the registry.
type Authenticator struct {
	masterHash string
	reg        *Registry
}

func NewAuthenticator(masterKey string, reg *Registry) *Authenticator {
	var h string
	if masterKey != "" {
		h = HashKey(masterKey)
	}
	return &Authenticator{masterHash: h, reg: reg}
}

// Authenticate returns the resolved scope and, for tenant scope, the
// enforcement decision. For master scope the decision is always allowed.
func (a *Authenticator) Authenticate(apiKey string, now int64) (Scope, Decision) {
	if a.masterHash != "" && constantTimeEqual(HashKey(apiKey), a.masterHash) {
		return ScopeMaster, Decision{Allowed: true, Code: CodeOK}
	}
	return ScopeTenant, a.reg.Authorize(apiKey, now)
}

func constantTimeEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
