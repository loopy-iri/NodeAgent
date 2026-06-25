package grpccompat

import (
	"context"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"

	"github.com/pasarguard/node/common"
	"github.com/pasarguard/node/config"
	"github.com/pasarguard/node/pkg/netutil"
	"github.com/pasarguard/node/shared"
	"github.com/pasarguard/node/tenant"
)

// startBufServer runs the compat gRPC server over an in-memory listener (no TLS)
// so we can test the semantics without certificates.
func startBufServer(t *testing.T, srv *Server) common.NodeServiceClient {
	t.Helper()
	lis := bufconn.Listen(1024 * 1024)
	gs := grpc.NewServer(
		grpc.UnaryInterceptor(srv.unaryAuth),
		grpc.StreamInterceptor(srv.streamAuth),
	)
	common.RegisterNodeServiceServer(gs, srv)
	go func() { _ = gs.Serve(lis) }()
	t.Cleanup(gs.Stop)

	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return common.NewNodeServiceClient(conn)
}

func authCtx(key string) context.Context {
	return metadata.NewOutgoingContext(context.Background(), metadata.Pairs("x-api-key", key))
}

func TestGRPCAuth(t *testing.T) {
	reg, _ := tenant.NewRegistry(tenant.NewMemStore())
	client := startBufServer(t, New(shared.NewManager(&config.Config{}, reg), "core-secret"))

	// Wrong key -> denied.
	if _, err := client.GetBaseInfo(authCtx("wrong"), &common.Empty{}); err == nil {
		t.Fatal("expected auth failure for wrong core key")
	}
	// Correct key -> ok.
	if _, err := client.GetBaseInfo(authCtx("core-secret"), &common.Empty{}); err != nil {
		t.Fatalf("expected success with correct core key, got %v", err)
	}
}

func TestGRPCSyncUsersIsNoop(t *testing.T) {
	reg, _ := tenant.NewRegistry(tenant.NewMemStore())
	mgr := shared.NewManager(&config.Config{}, reg)
	client := startBufServer(t, New(mgr, "core-secret"))

	ctx := authCtx("core-secret")

	// SyncUsers from the external panel must be accepted (no error)...
	if _, err := client.SyncUsers(ctx, &common.Users{Users: []*common.User{
		{Email: "panel-user", Proxies: &common.Proxy{Vless: &common.Vless{Id: "x"}}, Inbounds: []string{"vless-in"}},
	}}); err != nil {
		t.Fatalf("SyncUsers should be accepted, got %v", err)
	}
	// ...but must NOT create any tenant or apply users in our system.
	if got := len(reg.List()); got != 0 {
		t.Fatalf("panel SyncUsers must not create tenants/users, found %d", got)
	}

	// Stop must be a no-op (returns ok, does nothing).
	if _, err := client.Stop(ctx, &common.Empty{}); err != nil {
		t.Fatalf("Stop should be a no-op success, got %v", err)
	}
}

// TestGRPCStartAppliesConfig requires a real xray binary; it proves the panel
// can set the shared core config over gRPC.
func TestGRPCStartAppliesConfig(t *testing.T) {
	exe := os.Getenv("XRAY_EXECUTABLE_PATH")
	if exe == "" {
		t.Skip("set XRAY_EXECUTABLE_PATH to run the gRPC Start integration test")
	}
	cfg := &config.Config{
		XrayExecutablePath:  exe,
		XrayAssetsPath:      os.Getenv("XRAY_ASSETS_PATH"),
		GeneratedConfigPath: t.TempDir(),
		LogBufferSize:       10000,
		StartupLogTailSize:  200,
	}
	reg, _ := tenant.NewRegistry(tenant.NewMemStore())
	mgr := shared.NewManager(cfg, reg)
	defer mgr.Stop()
	client := startBufServer(t, New(mgr, "core-secret"))

	port := netutil.FindFreePort()
	xrayCfg := fmt.Sprintf(`{"log":{"loglevel":"warning"},"inbounds":[{"tag":"vless-in","listen":"127.0.0.1","port":%d,"protocol":"vless","settings":{"clients":[],"decryption":"none"}}],"outbounds":[{"tag":"direct","protocol":"freedom"}]}`, port)

	ctx, cancel := context.WithTimeout(authCtx("core-secret"), 30*time.Second)
	defer cancel()

	resp, err := client.Start(ctx, &common.Backend{Type: common.BackendType_XRAY, Config: xrayCfg})
	if err != nil {
		t.Fatalf("Start over gRPC failed: %v", err)
	}
	if !resp.GetStarted() {
		t.Fatalf("core should be started, got %+v", resp)
	}
	if !mgr.Started() {
		t.Fatal("manager core should be running after gRPC Start")
	}
}
