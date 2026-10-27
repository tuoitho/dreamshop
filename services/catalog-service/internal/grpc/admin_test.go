package grpc

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"ecommerce.local/catalog-service/internal/domain"
	"ecommerce.local/catalog-service/internal/media"
	catalogv1 "ecommerce.local/contracts/go/catalog/v1"
	commonv1 "ecommerce.local/contracts/go/common/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

type fakeMediaSigner struct {
	bucket      string
	signed      *media.SignedUpload
	ensureErr   error
	err         error
	ensureCalls int
	calls       int
	lastSize    int64
}

func (f *fakeMediaSigner) Bucket() string {
	if f.bucket == "" {
		return "product-media"
	}
	return f.bucket
}

func (f *fakeMediaSigner) EnsureBucket(_ context.Context) error {
	f.ensureCalls++
	return f.ensureErr
}

func (f *fakeMediaSigner) PresignPut(objectKey, _ string, sizeBytes int64, _ time.Duration) (*media.SignedUpload, error) {
	f.calls++
	f.lastSize = sizeBytes
	if f.err != nil {
		return nil, f.err
	}
	if f.signed != nil {
		return f.signed, nil
	}
	return &media.SignedUpload{
		UploadURL:        "/product-media/" + objectKey + "?X-Amz-Signature=sig",
		PublicURL:        "/product-media/" + objectKey,
		ExpiresInSeconds: 300,
	}, nil
}

type mapAuthorization struct {
	perms map[string]bool
	err   error
	calls int
}

func (m *mapAuthorization) CheckPermission(_ context.Context, _ string, permission string) (bool, error) {
	m.calls++
	if m.err != nil {
		return false, m.err
	}
	return m.perms[permission], nil
}

func (m *mapAuthorization) CheckAnyPermission(_ context.Context, _ string, permissions ...string) (bool, error) {
	m.calls++
	if m.err != nil {
		return false, m.err
	}
	for _, p := range permissions {
		if m.perms[p] {
			return true, nil
		}
	}
	return false, nil
}

func newAdminTestClient(t *testing.T, repo domain.Repository, auth authorizationClient, opts ...ServerOption) catalogv1.CatalogServiceClient {
	t.Helper()
	lis := bufconn.Listen(1024 * 1024)
	srv := grpc.NewServer()
	catalogv1.RegisterCatalogServiceServer(srv, NewServer(repo, auth, opts...))
	go func() {
		if err := srv.Serve(lis); err != nil {
			t.Errorf("grpc serve: %v", err)
		}
	}()
	t.Cleanup(func() {
		srv.Stop()
		_ = lis.Close()
	})

	conn, err := grpc.NewClient("passthrough:///catalog-admin-test",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("grpc dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return catalogv1.NewCatalogServiceClient(conn)
}

func TestAdminProductCRUDAndAuthorization(t *testing.T) {
	repo := &memoryRepo{products: demoProducts()}
	auth := &mapAuthorization{
		perms: map[string]bool{
			"product.view":   true,
			"product.create": true,
			"product.update": true,
			"inventory.view": true,
		},
	}
	client := newAdminTestClient(t, repo, auth)
	ctx := context.Background()

	listResp, err := client.ListAdminProducts(ctx, &catalogv1.ListAdminProductsRequest{
		UserId: "admin-1",
		Status: "all",
	})
	if err != nil {
		t.Fatalf("ListAdminProducts: %v", err)
	}
	if listResp.GetTotalItems() != 2 || listResp.GetActiveCount() != 2 {
		t.Fatalf("unexpected ListAdminProducts counts: %+v", listResp)
	}

	createResp, err := client.CreateProduct(ctx, &catalogv1.CreateProductRequest{
		UserId:        "admin-1",
		Name:          "Tai nghe Studio Pro",
		Brand:         "SoundLab",
		CategoryId:    "tai-nghe",
		CategoryName:  "Tai nghe",
		Price:         &commonv1.Money{Currency: "VND", AmountMinor: 4990000},
		OriginalPrice: &commonv1.Money{Currency: "VND", AmountMinor: 5990000},
		Images:        []string{"/product-media/products/2026/10/studio.webp"},
		Videos:        []string{"/product-media/products/2026/10/studio.mp4"},
		Variants:      []*catalogv1.ProductVariant{{Label: "Đen", ColorHex: "#111827"}},
		Details:       []string{"ANC chủ động"},
	})
	if err != nil {
		t.Fatalf("CreateProduct: %v", err)
	}
	createdID := createResp.GetProduct().GetId()
	if createdID == "" || len(createResp.GetProduct().GetVideos()) != 1 {
		t.Fatalf("unexpected CreateProduct response: %+v", createResp.GetProduct())
	}

	// Duplicate custom ID -> AlreadyExists (PRODUCT_ID_EXISTS)
	_, err = client.CreateProduct(ctx, &catalogv1.CreateProductRequest{
		UserId:       "admin-1",
		Id:           "dt-1",
		Name:         "Duplicate",
		Brand:        "Nova",
		CategoryId:   "dien-thoai",
		CategoryName: "Điện thoại",
		Price:        &commonv1.Money{Currency: "VND", AmountMinor: 1000000},
		Variants:     []*catalogv1.ProductVariant{{Label: "Đen"}},
	})
	assertErrorReason(t, err, codes.AlreadyExists, "PRODUCT_ID_EXISTS")

	updateResp, err := client.UpdateProduct(ctx, &catalogv1.UpdateProductRequest{
		UserId:       "admin-1",
		Id:           createdID,
		Name:         "Tai nghe Studio Pro V2",
		Brand:        "SoundLab",
		CategoryId:   "tai-nghe",
		CategoryName: "Tai nghe",
		Price:        &commonv1.Money{Currency: "VND", AmountMinor: 4590000},
		Images:       []string{"/product-media/products/2026/10/studio.webp"},
		Videos:       nil,
		Variants:     []*catalogv1.ProductVariant{{Label: "Bạc", ColorHex: "#cbd5e1"}},
	})
	if err != nil {
		t.Fatalf("UpdateProduct: %v", err)
	}
	if updateResp.GetProduct().GetName() != "Tai nghe Studio Pro V2" || len(updateResp.GetProduct().GetVideos()) != 0 {
		t.Fatalf("unexpected UpdateProduct response: %+v", updateResp.GetProduct())
	}

	getResp, err := client.GetAdminProduct(ctx, &catalogv1.GetAdminProductRequest{
		UserId: "admin-1",
		Id:     createdID,
	})
	if err != nil || getResp.GetProduct().GetId() != createdID {
		t.Fatalf("GetAdminProduct: %v, %+v", err, getResp)
	}

	invResp, err := client.ListInventoryCatalogItems(ctx, &catalogv1.ListInventoryCatalogItemsRequest{
		UserId:         "admin-1",
		IncludeDeleted: true,
	})
	if err != nil || len(invResp.GetItems()) != 3 {
		t.Fatalf("ListInventoryCatalogItems: %v, count=%d", err, len(invResp.GetItems()))
	}
}

func TestCreateProductMediaUploadAuthorizationAndStorageHandling(t *testing.T) {
	repo := &memoryRepo{products: demoProducts()}
	signer := &fakeMediaSigner{}
	// Operator with only product.update (without product.create) can still upload media while editing.
	auth := &mapAuthorization{perms: map[string]bool{"product.update": true}}
	client := newAdminTestClient(t, repo, auth, WithMediaSigner(signer, 5*time.Minute, 24*time.Hour))

	resp, err := client.CreateProductMediaUpload(context.Background(), &catalogv1.CreateProductMediaUploadRequest{
		UserId:      "editor-1",
		Filename:    "demo-clip.mp4",
		ContentType: "video/mp4",
		SizeBytes:   12 * 1024 * 1024,
	})
	if err != nil {
		t.Fatalf("CreateProductMediaUpload: %v", err)
	}
	if resp.GetMediaType() != "VIDEO" || resp.GetUploadId() == "" || resp.GetUploadUrl() == "" || resp.GetExpiresInSeconds() != 300 {
		t.Fatalf("unexpected CreateProductMediaUpload response: %+v", resp)
	}
	if signer.ensureCalls != 1 || signer.lastSize != 12*1024*1024 {
		t.Fatalf("expected EnsureBucket=1 and PresignPut sizeBytes=%d, got ensureCalls=%d lastSize=%d", 12*1024*1024, signer.ensureCalls, signer.lastSize)
	}

	// Unsupported MIME type returns InvalidArgument without calling signer.
	_, err = client.CreateProductMediaUpload(context.Background(), &catalogv1.CreateProductMediaUploadRequest{
		UserId:      "editor-1",
		Filename:    "xss.svg",
		ContentType: "image/svg+xml",
		SizeBytes:   512,
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("CreateProductMediaUpload(svg) code = %v, want InvalidArgument", status.Code(err))
	}

	// Unconfigured signer fails closed with MEDIA_STORAGE_UNAVAILABLE.
	unconfiguredClient := newAdminTestClient(t, repo, auth)
	_, err = unconfiguredClient.CreateProductMediaUpload(context.Background(), &catalogv1.CreateProductMediaUploadRequest{
		UserId:      "editor-1",
		Filename:    "photo.webp",
		ContentType: "image/webp",
		SizeBytes:   1024,
	})
	assertErrorReason(t, err, codes.Unavailable, "MEDIA_STORAGE_UNAVAILABLE")

	// Signer error also surfaces MEDIA_STORAGE_UNAVAILABLE.
	failingClient := newAdminTestClient(t, repo, auth, WithMediaSigner(&fakeMediaSigner{err: errors.New("s3 down")}, 5*time.Minute, 24*time.Hour))
	_, err = failingClient.CreateProductMediaUpload(context.Background(), &catalogv1.CreateProductMediaUploadRequest{
		UserId:      "editor-1",
		Filename:    "photo.webp",
		ContentType: "image/webp",
		SizeBytes:   1024,
	})
	assertErrorReason(t, err, codes.Unavailable, "MEDIA_STORAGE_UNAVAILABLE")

	// Operator lacking both product.create and product.update receives PRODUCT_PERMISSION_DENIED.
	deniedClient := newAdminTestClient(t, repo, &mapAuthorization{perms: map[string]bool{"product.view": true}}, WithMediaSigner(signer, 5*time.Minute, 24*time.Hour))
	_, err = deniedClient.CreateProductMediaUpload(context.Background(), &catalogv1.CreateProductMediaUploadRequest{
		UserId:      "viewer-1",
		Filename:    "photo.webp",
		ContentType: "image/webp",
		SizeBytes:   1024,
	})
	assertErrorReason(t, err, codes.PermissionDenied, "PRODUCT_PERMISSION_DENIED")
}
