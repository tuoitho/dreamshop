package authorization

import (
	"context"
	"net"
	"testing"
	"time"

	identityv1 "ecommerce.local/contracts/go/identity/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type recordingAuthorizationServer struct {
	identityv1.UnimplementedAuthorizationServiceServer
	request *identityv1.CheckPermissionRequest
	allowed bool
	err     error
}

func (s *recordingAuthorizationServer) CheckPermission(_ context.Context, request *identityv1.CheckPermissionRequest) (*identityv1.CheckPermissionResponse, error) {
	s.request = request
	if s.err != nil {
		return nil, s.err
	}
	return &identityv1.CheckPermissionResponse{Allowed: s.allowed}, nil
}

func startAuthorizationTestServer(t *testing.T, implementation identityv1.AuthorizationServiceServer) (string, func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	server := grpc.NewServer()
	identityv1.RegisterAuthorizationServiceServer(server, implementation)
	go server.Serve(listener)
	return listener.Addr().String(), func() {
		server.Stop()
		listener.Close()
	}
}

func TestClientCheckPermissionForwardsRequestAndAllowed(t *testing.T) {
	server := &recordingAuthorizationServer{allowed: true}
	target, stop := startAuthorizationTestServer(t, server)
	defer stop()

	client, err := NewClient(target, time.Second)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	defer client.Close()

	allowed, err := client.CheckPermission(context.Background(), "user-1", "cart.item.add")
	if err != nil || !allowed {
		t.Fatalf("CheckPermission() = %t, %v; want true, nil", allowed, err)
	}
	if server.request.GetUserId() != "user-1" || server.request.GetPermission() != "cart.item.add" {
		t.Fatalf("request = %+v", server.request)
	}
}

func TestClientCheckPermissionReturnsDenied(t *testing.T) {
	target, stop := startAuthorizationTestServer(t, &recordingAuthorizationServer{})
	defer stop()
	client, err := NewClient(target, time.Second)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	defer client.Close()

	allowed, err := client.CheckPermission(context.Background(), "user-1", "cart.item.add")
	if err != nil || allowed {
		t.Fatalf("CheckPermission() = %t, %v; want false, nil", allowed, err)
	}
}

func TestClientCheckPermissionPreservesStatus(t *testing.T) {
	target, stop := startAuthorizationTestServer(t, &recordingAuthorizationServer{err: status.Error(codes.PermissionDenied, "denied")})
	defer stop()
	client, err := NewClient(target, time.Second)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	defer client.Close()

	allowed, err := client.CheckPermission(context.Background(), "user-1", "cart.item.add")
	if allowed || status.Code(err) != codes.PermissionDenied {
		t.Fatalf("CheckPermission() = %t, %v; want false, PermissionDenied", allowed, err)
	}
}

type blockingAuthorizationServer struct {
	identityv1.UnimplementedAuthorizationServiceServer
}

func (blockingAuthorizationServer) CheckPermission(ctx context.Context, _ *identityv1.CheckPermissionRequest) (*identityv1.CheckPermissionResponse, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestClientCheckPermissionAppliesTimeout(t *testing.T) {
	target, stop := startAuthorizationTestServer(t, blockingAuthorizationServer{})
	defer stop()
	client, err := NewClient(target, 20*time.Millisecond)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	defer client.Close()

	started := time.Now()
	allowed, err := client.CheckPermission(context.Background(), "user-1", "cart.item.add")
	if allowed || status.Code(err) != codes.DeadlineExceeded {
		t.Fatalf("CheckPermission() = %t, %v; want false, DeadlineExceeded", allowed, err)
	}
	if elapsed := time.Since(started); elapsed >= time.Second {
		t.Fatalf("CheckPermission() took %s; want less than one second", elapsed)
	}
}

func TestNewClientRejectsNonPositiveTimeout(t *testing.T) {
	for _, timeout := range []time.Duration{0, -time.Millisecond} {
		t.Run(timeout.String(), func(t *testing.T) {
			client, err := NewClient("127.0.0.1:50054", timeout)
			if err == nil {
				_ = client.Close()
				t.Fatalf("NewClient() error = nil for timeout %s, want rejection", timeout)
			}
		})
	}
}
