package catalog

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	catalogv1 "ecommerce.local/contracts/go/catalog/v1"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type recordingCatalogServer struct {
	catalogv1.UnimplementedCatalogServiceServer
	softDeleteReq  *catalogv1.SoftDeleteProductRequest
	softDeleteErr  error
	createErr      error
	mediaUploadErr error
}

func (s *recordingCatalogServer) SoftDeleteProduct(_ context.Context, req *catalogv1.SoftDeleteProductRequest) (*catalogv1.SoftDeleteProductResponse, error) {
	s.softDeleteReq = req
	if s.softDeleteErr != nil {
		return nil, s.softDeleteErr
	}
	return &catalogv1.SoftDeleteProductResponse{Id: req.GetId(), Deleted: req.GetDeleted()}, nil
}

func (s *recordingCatalogServer) CreateProduct(_ context.Context, req *catalogv1.CreateProductRequest) (*catalogv1.CreateProductResponse, error) {
	if s.createErr != nil {
		return nil, s.createErr
	}
	return &catalogv1.CreateProductResponse{Product: &catalogv1.GetProductResponse{Id: req.GetId(), Name: req.GetName()}}, nil
}

func (s *recordingCatalogServer) CreateProductMediaUpload(_ context.Context, _ *catalogv1.CreateProductMediaUploadRequest) (*catalogv1.CreateProductMediaUploadResponse, error) {
	if s.mediaUploadErr != nil {
		return nil, s.mediaUploadErr
	}
	return &catalogv1.CreateProductMediaUploadResponse{UploadId: "u-1", MediaType: "IMAGE"}, nil
}

func newRecordingCatalogClient(t *testing.T, server catalogv1.CatalogServiceServer, timeout time.Duration) *client {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	grpcServer := grpc.NewServer()
	catalogv1.RegisterCatalogServiceServer(grpcServer, server)
	go func() { _ = grpcServer.Serve(listener) }()
	t.Cleanup(func() {
		grpcServer.Stop()
		_ = listener.Close()
	})

	client, err := NewClient(listener.Addr().String(), timeout)
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func TestClientSoftDeleteForwardsUserIDProductIDAndDeleted(t *testing.T) {
	server := &recordingCatalogServer{}
	client := newRecordingCatalogClient(t, server, time.Second)

	if err := client.SoftDeleteProduct(context.Background(), "admin-1", "dt-1", true); err != nil {
		t.Fatalf("SoftDeleteProduct() error = %v", err)
	}
	if got := server.softDeleteReq; got.GetUserId() != "admin-1" || got.GetId() != "dt-1" || !got.GetDeleted() {
		t.Fatalf("SoftDeleteProduct request = %+v", got)
	}
}

func TestClientMapsProductPermissionErrorInfoToErrPermissionDenied(t *testing.T) {
	st := status.New(codes.PermissionDenied, "permission denied")
	st, err := st.WithDetails(&errdetails.ErrorInfo{Domain: "ecommerce.catalog.v1", Reason: "PRODUCT_PERMISSION_DENIED"})
	if err != nil {
		t.Fatalf("with details: %v", err)
	}
	client := newRecordingCatalogClient(t, &recordingCatalogServer{softDeleteErr: st.Err()}, time.Second)

	got := client.SoftDeleteProduct(context.Background(), "customer-1", "dt-1", true)
	if !errors.Is(got, ErrPermissionDenied) {
		t.Fatalf("error = %v, want ErrPermissionDenied", got)
	}
}

func TestClientMapsCatalogAuthorizationUnavailableErrorInfoToErrAuthorizationUnavailable(t *testing.T) {
	st := status.New(codes.Unavailable, "authorization unavailable")
	st, err := st.WithDetails(&errdetails.ErrorInfo{Domain: "ecommerce.catalog.v1", Reason: "AUTHORIZATION_UNAVAILABLE"})
	if err != nil {
		t.Fatalf("with details: %v", err)
	}
	client := newRecordingCatalogClient(t, &recordingCatalogServer{softDeleteErr: st.Err()}, time.Second)

	got := client.SoftDeleteProduct(context.Background(), "admin-1", "dt-1", true)
	if !errors.Is(got, ErrAuthorizationUnavailable) {
		t.Fatalf("error = %v, want ErrAuthorizationUnavailable", got)
	}
}

func TestClientSoftDeleteMapsNotFoundToErrProductNotFound(t *testing.T) {
	client := newRecordingCatalogClient(t, &recordingCatalogServer{softDeleteErr: status.Error(codes.NotFound, "missing")}, time.Second)

	got := client.SoftDeleteProduct(context.Background(), "admin-1", "dt-1", true)
	if !errors.Is(got, ErrProductNotFound) {
		t.Fatalf("error = %v, want ErrProductNotFound", got)
	}
}

func TestClientSoftDeletePreservesBareCatalogUnavailableError(t *testing.T) {
	client := newRecordingCatalogClient(t, &recordingCatalogServer{softDeleteErr: status.Error(codes.Unavailable, "catalog down")}, time.Second)

	err := client.SoftDeleteProduct(context.Background(), "admin-1", "dt-1", true)
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("error code = %s, want Unavailable; error = %v", status.Code(err), err)
	}
	if errors.Is(err, ErrPermissionDenied) || errors.Is(err, ErrAuthorizationUnavailable) {
		t.Fatalf("bare Catalog outage unexpectedly matched authorization sentinel: %v", err)
	}
}

func TestClientMapsAdminCreateAndMediaErrors(t *testing.T) {
	existsSt, _ := status.New(codes.AlreadyExists, "exists").WithDetails(&errdetails.ErrorInfo{
		Domain: "ecommerce.catalog.v1",
		Reason: "PRODUCT_ID_EXISTS",
	})
	clientExists := newRecordingCatalogClient(t, &recordingCatalogServer{createErr: existsSt.Err()}, time.Second)
	if _, err := clientExists.CreateProduct(context.Background(), &catalogv1.CreateProductRequest{UserId: "admin-1", Id: "dt-1"}); !errors.Is(err, ErrProductExists) {
		t.Fatalf("CreateProduct(exists) = %v, want ErrProductExists", err)
	}

	attachedSt, _ := status.New(codes.FailedPrecondition, "attached").WithDetails(&errdetails.ErrorInfo{
		Domain: "ecommerce.catalog.v1",
		Reason: "MEDIA_ALREADY_ATTACHED",
	})
	clientAttached := newRecordingCatalogClient(t, &recordingCatalogServer{createErr: attachedSt.Err()}, time.Second)
	if _, err := clientAttached.CreateProduct(context.Background(), &catalogv1.CreateProductRequest{UserId: "admin-1"}); !errors.Is(err, ErrMediaAlreadyAttached) {
		t.Fatalf("CreateProduct(attached) = %v, want ErrMediaAlreadyAttached", err)
	}

	storageSt, _ := status.New(codes.Unavailable, "storage down").WithDetails(&errdetails.ErrorInfo{
		Domain: "ecommerce.catalog.v1",
		Reason: "MEDIA_STORAGE_UNAVAILABLE",
	})
	clientStorage := newRecordingCatalogClient(t, &recordingCatalogServer{mediaUploadErr: storageSt.Err()}, time.Second)
	if _, err := clientStorage.CreateProductMediaUpload(context.Background(), &catalogv1.CreateProductMediaUploadRequest{UserId: "admin-1"}); !errors.Is(err, ErrMediaStorageUnavailable) {
		t.Fatalf("CreateProductMediaUpload(storage down) = %v, want ErrMediaStorageUnavailable", err)
	}
}
