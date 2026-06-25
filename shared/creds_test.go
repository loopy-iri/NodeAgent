package shared

import (
	"context"
	"testing"

	"github.com/pasarguard/node/common"
	"github.com/pasarguard/node/config"
	"github.com/pasarguard/node/tenant"
)

func vlessUser(email, id string) *common.User {
	return &common.User{
		Email:    email,
		Proxies:  &common.Proxy{Vless: &common.Vless{Id: id}},
		Inbounds: []string{"vless-in"},
	}
}

func newCredTestManager(t *testing.T) *Manager {
	t.Helper()
	reg, err := tenant.NewRegistry(tenant.NewMemStore())
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	for _, id := range []string{"t1", "t2"} {
		if _, err := reg.CreateTenant(tenant.CreateParams{ID: id, APIKey: "k-" + id, QuotaBytes: 1 << 30}); err != nil {
			t.Fatalf("CreateTenant %s: %v", id, err)
		}
	}
	// core stays nil: credential checks and bookkeeping run without a live Xray.
	return NewManager(&config.Config{}, reg)
}

func TestCredentialUniqueAcrossTenants(t *testing.T) {
	m := newCredTestManager(t)
	ctx := context.Background()
	const uuid = "11111111-1111-1111-1111-111111111111"

	if err := m.SetTenantUsers(ctx, "t1", []*common.User{vlessUser("alice", uuid)}); err != nil {
		t.Fatalf("t1 set: %v", err)
	}
	// t2 tries to use the same uuid -> must be rejected.
	err := m.SetTenantUsers(ctx, "t2", []*common.User{vlessUser("bob", uuid)})
	if err == nil {
		t.Fatal("expected cross-tenant credential collision to be rejected")
	}
	// A distinct uuid for t2 is fine.
	if err := m.SetTenantUsers(ctx, "t2", []*common.User{vlessUser("bob", "22222222-2222-2222-2222-222222222222")}); err != nil {
		t.Fatalf("t2 distinct uuid should succeed: %v", err)
	}
}

func TestCredentialDuplicateWithinBatch(t *testing.T) {
	m := newCredTestManager(t)
	const uuid = "33333333-3333-3333-3333-333333333333"
	err := m.SetTenantUsers(context.Background(), "t1", []*common.User{
		vlessUser("alice", uuid),
		vlessUser("carol", uuid),
	})
	if err == nil {
		t.Fatal("expected duplicate-credential-in-batch to be rejected")
	}
}

func TestSameTenantResyncKeepsCredential(t *testing.T) {
	m := newCredTestManager(t)
	ctx := context.Background()
	const uuid = "44444444-4444-4444-4444-444444444444"

	if err := m.SetTenantUsers(ctx, "t1", []*common.User{vlessUser("alice", uuid)}); err != nil {
		t.Fatalf("first sync: %v", err)
	}
	// Re-syncing the same tenant's same user (same credential) must not collide.
	if err := m.SetTenantUsers(ctx, "t1", []*common.User{vlessUser("alice", uuid)}); err != nil {
		t.Fatalf("re-sync should succeed: %v", err)
	}
	// After removing the user, the credential is freed and another tenant can use it.
	if err := m.SetTenantUsers(ctx, "t1", []*common.User{}); err != nil {
		t.Fatalf("clear t1: %v", err)
	}
	if err := m.SetTenantUsers(ctx, "t2", []*common.User{vlessUser("bob", uuid)}); err != nil {
		t.Fatalf("t2 should reuse freed credential: %v", err)
	}
}
