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
	request       *identityv1.CheckPermissionRequest
	batchRequest  *identityv1.CheckPermissionsRequest
	allowed       bool
	batchAllowed  map[string]bool
	err           error
}

func (s *recordingAuthorizationServer) CheckPermission(_ context.Context, request *identityv1.CheckPermissionRequest) (*identityv1.CheckPermissionResponse, error) {
	s.request = request
	if s.err != nil {
		return nil, s.err
	}
	return &identityv1.CheckPermissionResponse{Allowed: s.allowed}, nil
}

func (s *recordingAuthorizationServer) CheckPermissions(_ context.Context, request *identityv1.CheckPermissionsRequest) (*identityv1.CheckPermissionsResponse, error) {
	s.batchRequest = request
	if s.err != nil {
		return nil, s.err
	}
	decisions := make([]*identityv1.PermissionDecision, 0, len(request.GetPermissions()))
	for _, perm := range request.GetPermissions() {
		decisions = append(decisions, &identityv1.PermissionDecision{
			Permission: perm,
			Allowed:    s.batchAllowed[perm],
		})
	}
	return &identityv1.CheckPermissionsResponse{Decisions: decisions}, nil
}

func startAuthorizationTestServer(t *testing.T, implementation identityv1.AuthorizationServiceServer) (string, func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	server := grpc.NewServer()
	identityv1.RegisterAuthorizationServiceServer(server, implementation)
	go func() { _ = server.Serve(listener) }()
	return listener.Addr().String(), func() {
		server.Stop()
		_ = listener.Close()
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

	allowed, err := client.CheckPermission(context.Background(), "admin-1", "product.delete")
	if err != nil || !allowed {
		t.Fatalf("CheckPermission() = %t, %v; want true, nil", allowed, err)
	}
	if server.request.GetUserId() != "admin-1" || server.request.GetPermission() != "product.delete" {
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

	allowed, err := client.CheckPermission(context.Background(), "customer-1", "product.delete")
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

	allowed, err := client.CheckPermission(context.Background(), "user-1", "product.delete")
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
	allowed, err := client.CheckPermission(context.Background(), "admin-1", "product.delete")
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

func TestNewClientRejectsBlankTarget(t *testing.T) {
	for _, target := range []string{"", "   "} {
		client, err := NewClient(target, time.Second)
		if err == nil {
			_ = client.Close()
			t.Fatalf("NewClient(%q) error = nil, want rejection", target)
		}
	}
}

func TestClientCheckAnyPermissionUsesBatchCheckPermissions(t *testing.T) {
	server := &recordingAuthorizationServer{
		batchAllowed: map[string]bool{
			"product.create": false,
			"product.update": true,
		},
	}
	target, stop := startAuthorizationTestServer(t, server)
	defer stop()

	client, err := NewClient(target, time.Second)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	defer client.Close()

	allowed, err := client.CheckAnyPermission(context.Background(), "editor-1", "product.create", "product.update")
	if err != nil || !allowed {
		t.Fatalf("CheckAnyPermission() = %t, %v; want true, nil", allowed, err)
	}
	if server.batchRequest.GetUserId() != "editor-1" || len(server.batchRequest.GetPermissions()) != 2 {
		t.Fatalf("batchRequest = %+v", server.batchRequest)
	}
}
