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
	go func() { _ = server.Serve(listener) }()
	return listener.Addr().String(), func() {
		server.Stop()
		_ = listener.Close()
	}
}

func TestClientCheckPermissionForwardsRequestAndTimeout(t *testing.T) {
	server := &recordingAuthorizationServer{allowed: true}
	target, stop := startAuthorizationTestServer(t, server)
	defer stop()

	client, err := NewClient(target, time.Second)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	defer client.Close()

	allowed, err := client.CheckPermission(context.Background(), "admin-1", "inventory.update")
	if err != nil || !allowed {
		t.Fatalf("CheckPermission() = %t, %v; want true, nil", allowed, err)
	}
	if server.request.GetUserId() != "admin-1" || server.request.GetPermission() != "inventory.update" {
		t.Fatalf("request = %+v", server.request)
	}

	deniedTarget, stopDenied := startAuthorizationTestServer(t, &recordingAuthorizationServer{err: status.Error(codes.Unavailable, "down")})
	defer stopDenied()
	deniedClient, err := NewClient(deniedTarget, time.Second)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	defer deniedClient.Close()
	if _, err := deniedClient.CheckPermission(context.Background(), "admin-1", "inventory.view"); status.Code(err) != codes.Unavailable {
		t.Fatalf("expected Unavailable, got %v", err)
	}
}
