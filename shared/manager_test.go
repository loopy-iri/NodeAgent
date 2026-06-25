package shared

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/pasarguard/node/common"
	"github.com/pasarguard/node/config"
	"github.com/pasarguard/node/pkg/netutil"
	"github.com/pasarguard/node/tenant"
)

// TestSharedManagerWithRealXray exercises the full shared-core path against a
// real xray binary: start core, add a tenant user, read stats, enforce quota
// (which removes the user), and resume (which re-adds it).
//
// It is skipped unless XRAY_EXECUTABLE_PATH points at an xray executable, so the
// normal `go test ./...` run stays green on machines without xray.
func TestSharedManagerWithRealXray(t *testing.T) {
	exe := os.Getenv("XRAY_EXECUTABLE_PATH")
	if exe == "" {
		t.Skip("set XRAY_EXECUTABLE_PATH to run the shared-core integration test")
	}

	cfg := &config.Config{
		XrayExecutablePath:  exe,
		XrayAssetsPath:      os.Getenv("XRAY_ASSETS_PATH"),
		GeneratedConfigPath: t.TempDir(),
		LogBufferSize:       10000,
		StartupLogTailSize:  200,
	}

	reg, err := tenant.NewRegistry(tenant.NewMemStore())
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	m := NewManager(cfg, reg)

	inboundPort := netutil.FindFreePort()
	configJSON := fmt.Sprintf(`{
        "log": {"loglevel": "warning"},
        "inbounds": [
            {
                "tag": "vless-in",
                "listen": "127.0.0.1",
                "port": %d,
                "protocol": "vless",
                "settings": {"clients": [], "decryption": "none"}
            }
        ],
        "outbounds": [
            {"tag": "direct", "protocol": "freedom"}
        ]
    }`, inboundPort)

	ctx := context.Background()
	if err := m.StartCore(ctx, configJSON); err != nil {
		t.Fatalf("StartCore: %v", err)
	}
	defer m.Stop()

	if !m.Started() {
		t.Fatal("core should be started")
	}
	t.Logf("xray core started, version=%s", m.Version())

	// Register a tenant with a 1 MiB quota.
	const quota = int64(1 << 20)
	if _, err := reg.CreateTenant(tenant.CreateParams{ID: "t1", APIKey: "key-1", QuotaBytes: quota}); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}

	user := &common.User{
		Email:    "alice",
		Proxies:  &common.Proxy{Vless: &common.Vless{Id: uuid.NewString()}},
		Inbounds: []string{"vless-in"},
	}
	if err := m.SetTenantUsers(ctx, "t1", []*common.User{user}); err != nil {
		t.Fatalf("SetTenantUsers (add): %v", err)
	}

	// Stats query must succeed against the live core (no traffic yet => empty).
	if err := m.CollectAndEnforce(ctx, time.Now().Unix()); err != nil {
		t.Fatalf("CollectAndEnforce (initial): %v", err)
	}
	if tn, _ := reg.Get("t1"); tn.Status != tenant.StatusActive {
		t.Fatalf("tenant should still be active, got %s", tn.Status)
	}

	// Simulate going over quota, then enforce: tenant must be suspended and its
	// users removed from the core without error.
	if _, err := reg.AddUsage("t1", quota+1); err != nil {
		t.Fatalf("AddUsage: %v", err)
	}
	if err := m.CollectAndEnforce(ctx, time.Now().Unix()); err != nil {
		t.Fatalf("CollectAndEnforce (over quota): %v", err)
	}
	if tn, _ := reg.Get("t1"); tn.Status != tenant.StatusSuspended || tn.Reason != tenant.ReasonQuota {
		t.Fatalf("tenant should be suspended for quota, got status=%s reason=%s", tn.Status, tn.Reason)
	}

	// Renewal: reset the period and resume; users are re-added to the live core.
	if _, err := reg.ResetPeriod("t1"); err != nil {
		t.Fatalf("ResetPeriod: %v", err)
	}
	if err := m.ResumeTenant(ctx, "t1"); err != nil {
		t.Fatalf("ResumeTenant: %v", err)
	}
	if tn, _ := reg.Get("t1"); tn.Status != tenant.StatusActive || tn.UsedBytes != 0 {
		t.Fatalf("after renewal expected active/zero usage, got status=%s used=%d", tn.Status, tn.UsedBytes)
	}
}
