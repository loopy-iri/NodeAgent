// Package grpccompat exposes a PasarGuard-compatible gRPC NodeService so an
// external panel (e.g. PasarGuard) can manage ONLY the node's shared core
// config, while the multi-tenant user/quota state remains owned by our HTTP
// control plane.
//
// Rationale: the external panel does not know this node is "sold" (multi-tenant).
// So this server:
//   - accepts Start(config) and applies the config to the shared core;
//   - IGNORES the users the panel sends (SyncUser/SyncUsers are accepted but are
//     no-ops) so the panel cannot clobber tenant users;
//   - makes Stop a no-op so the panel cannot take the sold node down;
//   - returns empty (not errors) for stats/logs so the panel UI stays happy.
//
// It is gated by a dedicated core key and only started when that key is set.
package grpccompat

import (
	"context"
	"crypto/subtle"
	"crypto/tls"
	"io"
	"log"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/pasarguard/node/common"
	"github.com/pasarguard/node/pkg/sysstats"
	"github.com/pasarguard/node/shared"
)

const nodeVersion = "0.5.2-mt"

// Server implements common.NodeServiceServer with sandboxed semantics.
type Server struct {
	common.UnimplementedNodeServiceServer
	mgr     *shared.Manager
	coreKey string
}

func New(mgr *shared.Manager, coreKey string) *Server {
	return &Server{mgr: mgr, coreKey: coreKey}
}

// Serve starts the gRPC server on addr with the given TLS config until the
// listener is closed. Returns a stop function.
func Serve(tlsConfig *tls.Config, addr string, srv *Server) (func(), error) {
	const maxMsg = 64 * 1024 * 1024
	gs := grpc.NewServer(
		grpc.Creds(credentials.NewTLS(tlsConfig)),
		grpc.MaxRecvMsgSize(maxMsg),
		grpc.MaxSendMsgSize(maxMsg),
		grpc.UnaryInterceptor(srv.unaryAuth),
		grpc.StreamInterceptor(srv.streamAuth),
	)
	common.RegisterNodeServiceServer(gs, srv)

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	go func() {
		log.Printf("gRPC (PasarGuard-compat) listening on %s", addr)
		if err := gs.Serve(ln); err != nil {
			log.Printf("grpc compat server stopped: %v", err)
		}
	}()
	return gs.GracefulStop, nil
}

// --- auth ---

func (s *Server) authorize(ctx context.Context) error {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "missing metadata")
	}
	keys := md.Get("x-api-key")
	if len(keys) == 0 {
		return status.Error(codes.Unauthenticated, "missing x-api-key")
	}
	if subtle.ConstantTimeCompare([]byte(keys[0]), []byte(s.coreKey)) != 1 {
		return status.Error(codes.PermissionDenied, "invalid core key")
	}
	return nil
}

func (s *Server) unaryAuth(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	if err := s.authorize(ctx); err != nil {
		return nil, err
	}
	return handler(ctx, req)
}

func (s *Server) streamAuth(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	if err := s.authorize(ss.Context()); err != nil {
		return err
	}
	return handler(srv, ss)
}

// --- core management (honored) ---

// Start applies the panel's Xray config to the shared core. Users in the payload
// are intentionally ignored; tenant users are owned by the HTTP control plane.
func (s *Server) Start(ctx context.Context, b *common.Backend) (*common.BaseInfoResponse, error) {
	if b.GetType() != common.BackendType_XRAY {
		return nil, status.Error(codes.InvalidArgument, "only xray backend is supported")
	}
	if cfg := b.GetConfig(); cfg != "" {
		if err := s.mgr.ApplyConfig(ctx, cfg); err != nil {
			return nil, status.Errorf(codes.Internal, "apply config: %v", err)
		}
	}
	return s.baseInfo(), nil
}

func (s *Server) GetBaseInfo(context.Context, *common.Empty) (*common.BaseInfoResponse, error) {
	return s.baseInfo(), nil
}

func (s *Server) baseInfo() *common.BaseInfoResponse {
	return &common.BaseInfoResponse{
		Started:     s.mgr.Started(),
		CoreVersion: s.mgr.Version(),
		NodeVersion: nodeVersion,
	}
}

func (s *Server) GetSystemStats(context.Context, *common.Empty) (*common.SystemStatsResponse, error) {
	stats, err := sysstats.GetSystemStats()
	if err != nil {
		return &common.SystemStatsResponse{}, nil
	}
	return stats, nil
}

func (s *Server) GetBackendStats(context.Context, *common.Empty) (*common.BackendStatsResponse, error) {
	return &common.BackendStatsResponse{}, nil
}

// --- user operations (accepted but NOT applied) ---

func (s *Server) Stop(context.Context, *common.Empty) (*common.Empty, error) {
	// No-op: the external panel must not be able to stop a sold node.
	return &common.Empty{}, nil
}

func (s *Server) SyncUsers(context.Context, *common.Users) (*common.Empty, error) {
	return &common.Empty{}, nil // accepted, ignored
}

func (s *Server) SyncUser(stream grpc.ClientStreamingServer[common.User, common.Empty]) error {
	for {
		if _, err := stream.Recv(); err != nil {
			if err == io.EOF {
				return stream.SendAndClose(&common.Empty{})
			}
			return err
		}
		// user ignored
	}
}

func (s *Server) SyncUsersChunked(stream grpc.ClientStreamingServer[common.UsersChunk, common.Empty]) error {
	for {
		if _, err := stream.Recv(); err != nil {
			if err == io.EOF {
				return stream.SendAndClose(&common.Empty{})
			}
			return err
		}
		// chunk ignored
	}
}

// --- stats/logs (empty, not errors, to keep the panel UI happy) ---

func (s *Server) GetStats(context.Context, *common.StatRequest) (*common.StatResponse, error) {
	return &common.StatResponse{}, nil
}

func (s *Server) GetOutboundsLatency(context.Context, *common.LatencyRequest) (*common.LatencyResponse, error) {
	return &common.LatencyResponse{}, nil
}

func (s *Server) GetUserOnlineStats(context.Context, *common.StatRequest) (*common.OnlineStatResponse, error) {
	return &common.OnlineStatResponse{}, nil
}

func (s *Server) GetUserOnlineIpListStats(context.Context, *common.StatRequest) (*common.StatsOnlineIpListResponse, error) {
	return &common.StatsOnlineIpListResponse{}, nil
}

func (s *Server) GetLogs(_ *common.Empty, _ grpc.ServerStreamingServer[common.Log]) error {
	return nil // empty log stream
}
