package grpc

import (
	"bytes"
	"context"
	"log/slog"
	"net"
	"strings"
	"testing"
	"time"

	"ecommerce.local/catalog-service/internal/authorization"
	"ecommerce.local/catalog-service/internal/domain"
	catalogv1 "ecommerce.local/contracts/go/catalog/v1"
	identityv1 "ecommerce.local/contracts/go/identity/v1"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

// memoryRepo is a domain.Repository backed by the pure in-memory search so
// handler tests stay free of infrastructure.
type memoryRepo struct {
	products        []*domain.Product
	mediaAssets     []*domain.MediaAsset
	softDeleteCalls int
}

func (m *memoryRepo) GetByID(_ context.Context, id string) (*domain.Product, error) {
	for _, p := range m.products {
		if p.ID == id {
			return p, nil
		}
	}
	return nil, domain.ErrProductNotFound
}

func (m *memoryRepo) Search(_ context.Context, filter domain.SearchFilter) (*domain.SearchResult, error) {
	return domain.SearchProducts(m.products, filter), nil
}

func (m *memoryRepo) SoftDelete(_ context.Context, id string, deleted bool) error {
	m.softDeleteCalls++
	for _, p := range m.products {
		if p.ID == id {
			p.Deleted = deleted
			if p.Version <= 0 {
				p.Version = 1
			}
			p.Version++
			return nil
		}
	}
	return domain.ErrProductNotFound
}

func (m *memoryRepo) ListAdmin(_ context.Context, filter domain.AdminSearchFilter) (*domain.AdminSearchResult, error) {
	return domain.ListAdminProducts(m.products, filter), nil
}

func (m *memoryRepo) ListInventoryItems(_ context.Context, filter domain.InventoryCatalogFilter) ([]*domain.InventoryCatalogItem, error) {
	return domain.ListInventoryCatalogItems(m.products, filter), nil
}

func (m *memoryRepo) Create(_ context.Context, product *domain.Product, _ time.Duration) (*domain.Product, error) {
	for _, existing := range m.products {
		if existing.ID == product.ID {
			return nil, domain.ErrProductExists
		}
		for _, ev := range existing.Variants {
			for _, nv := range product.Variants {
				if ev.SkuID == nv.SkuID {
					return nil, domain.ErrSkuConflict
				}
			}
		}
	}
	cp := *product
	if cp.Version <= 0 {
		cp.Version = 1
	}
	now := time.Now().UTC()
	cp.CreatedAt = now
	cp.UpdatedAt = now
	m.products = append(m.products, &cp)
	return &cp, nil
}

func (m *memoryRepo) Update(_ context.Context, product *domain.Product, _ time.Duration) (*domain.Product, error) {
	for _, existing := range m.products {
		if existing.ID == product.ID {
			continue
		}
		for _, ev := range existing.Variants {
			for _, nv := range product.Variants {
				if ev.SkuID == nv.SkuID {
					return nil, domain.ErrSkuConflict
				}
			}
		}
	}
	for i, existing := range m.products {
		if existing.ID == product.ID {
			cp := *product
			cp.Deleted = existing.Deleted
			cp.Rating = existing.Rating
			cp.ReviewCount = existing.ReviewCount
			cp.SoldCount = existing.SoldCount
			prevVersion := existing.Version
			if prevVersion <= 0 {
				prevVersion = 1
			}
			cp.Version = prevVersion + 1
			cp.CreatedAt = existing.CreatedAt
			cp.UpdatedAt = time.Now().UTC()
			m.products[i] = &cp
			return &cp, nil
		}
	}
	return nil, domain.ErrProductNotFound
}

func (m *memoryRepo) CreateMediaAsset(_ context.Context, asset *domain.MediaAsset) error {
	cp := *asset
	m.mediaAssets = append(m.mediaAssets, &cp)
	return nil
}

type fakeAuthorization struct {
	allowed    bool
	err        error
	userID     string
	permission string
	calls      int
}

func (f *fakeAuthorization) CheckPermission(_ context.Context, userID, permission string) (bool, error) {
	f.calls++
	f.userID = userID
	f.permission = permission
	return f.allowed, f.err
}

func demoProducts() []*domain.Product {
	return []*domain.Product{
		{
			ID:                 "dt-1",
			Name:               "Điện thoại thông minh Nova X15 Pro 5G (16GB/512GB)",
			ShortDescription:   "",
			Description:        "Chip xử lý mạnh mẽ thế hệ mới.",
			Brand:              "Nova",
			CategoryID:         "dien-thoai",
			CategoryName:       "Điện thoại",
			PriceMinor:         18990000,
			OriginalPriceMinor: 21990000,
			MaxPriceMinor:      20990000,
			Images:             []string{"https://picsum.photos/seed/dt-1-1/800/800"},
			Rating:             4.9,
			ReviewCount:        1284,
			SoldCount:          2310,
			IsNew:              false,
			Badge:              "hot",
			Featured:           true,
			Emoji:              "📱",
			ImageHue:           []uint32{220, 260},
			Variants: []domain.ProductVariant{
				{SkuID: "dt-1-v1", Label: "Đen", ColorHex: "#1f2937", PriceMinor: 18990000, OriginalPriceMinor: 21990000},
				{SkuID: "dt-1-v2", Label: "Xanh", ColorHex: "#1e40af", PriceMinor: 20990000, OriginalPriceMinor: 22990000},
			},
			Details: []string{"Màn hình: 6.8\" AMOLED 144Hz"},
		},
		{
			ID:                 "dt-2",
			Name:               "Điện thoại thông minh Lotus 12A (6GB/128GB)",
			Brand:              "Lotus",
			CategoryID:         "dien-thoai",
			CategoryName:       "Điện thoại",
			PriceMinor:         5490000,
			OriginalPriceMinor: 6990000,
			MaxPriceMinor:      5490000,
			Rating:             4.7,
			ReviewCount:        3521,
			SoldCount:          8920,
			Emoji:              "📲",
			ImageHue:           []uint32{150, 190},
			Variants: []domain.ProductVariant{
				{SkuID: "dt-2-v1", Label: "6GB/128GB", PriceMinor: 5490000, OriginalPriceMinor: 6990000},
			},
		},
	}
}

func newTestClient(t *testing.T, repo domain.Repository) catalogv1.CatalogServiceClient {
	t.Helper()
	return newTestClientWithAuth(t, repo, &fakeAuthorization{allowed: true})
}

func newTestClientWithAuth(t *testing.T, repo domain.Repository, auth authorizationClient) catalogv1.CatalogServiceClient {
	t.Helper()

	lis := bufconn.Listen(1024 * 1024)
	srv := grpc.NewServer()
	catalogv1.RegisterCatalogServiceServer(srv, NewServer(repo, auth))
	go func() {
		if err := srv.Serve(lis); err != nil {
			t.Errorf("grpc serve: %v", err)
		}
	}()
	t.Cleanup(func() {
		srv.Stop()
		_ = lis.Close()
	})

	conn, err := grpc.NewClient("passthrough:///catalog-service-test",
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

func TestGetProductHappy(t *testing.T) {
	client := newTestClient(t, &memoryRepo{products: demoProducts()})

	resp, err := client.GetProduct(context.Background(), &catalogv1.GetProductRequest{Id: "dt-1"})
	if err != nil {
		t.Fatalf("GetProduct: %v", err)
	}

	if resp.GetId() != "dt-1" || resp.GetName() != "Điện thoại thông minh Nova X15 Pro 5G (16GB/512GB)" {
		t.Fatalf("unexpected identity fields: %+v", resp)
	}
	if resp.GetBrand() != "Nova" || resp.GetCategoryId() != "dien-thoai" || resp.GetCategoryName() != "Điện thoại" {
		t.Fatalf("unexpected catalog fields: %+v", resp)
	}
	if resp.GetDescription() == "" {
		t.Fatal("expected description to be populated")
	}
	if resp.GetPrice().GetCurrency() != "VND" || resp.GetPrice().GetAmountMinor() != 18990000 {
		t.Fatalf("unexpected price: %+v", resp.GetPrice())
	}
	if resp.GetOriginalPrice().GetAmountMinor() != 21990000 {
		t.Fatalf("unexpected original price: %+v", resp.GetOriginalPrice())
	}
	if len(resp.GetImages()) != 1 || resp.GetImages()[0] != "https://picsum.photos/seed/dt-1-1/800/800" {
		t.Fatalf("unexpected images: %v", resp.GetImages())
	}
	if resp.GetRating() != 4.9 || resp.GetReviewCount() != 1284 || resp.GetSoldCount() != 2310 {
		t.Fatalf("unexpected rating/review/sold: %+v", resp)
	}
	if !resp.GetFeatured() || resp.GetBadge() != "hot" || resp.GetEmoji() != "📱" {
		t.Fatalf("unexpected merchandising fields: %+v", resp)
	}
	if len(resp.GetImageHue()) != 2 || resp.GetImageHue()[0] != 220 || resp.GetImageHue()[1] != 260 {
		t.Fatalf("unexpected imageHue: %v", resp.GetImageHue())
	}
	if len(resp.GetVariants()) != 2 || resp.GetVariants()[0].GetLabel() != "Đen" || resp.GetVariants()[0].GetColorHex() != "#1f2937" {
		t.Fatalf("unexpected variants: %v", resp.GetVariants())
	}
	if len(resp.GetDetails()) != 1 || resp.GetDetails()[0] != "Màn hình: 6.8\" AMOLED 144Hz" {
		t.Fatalf("unexpected details: %v", resp.GetDetails())
	}
}

func assertErrorReason(t *testing.T, err error, wantCode codes.Code, wantReason string) {
	t.Helper()
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %v", err)
	}
	if st.Code() != wantCode {
		t.Fatalf("status code = %v, want %v", st.Code(), wantCode)
	}
	for _, detail := range st.Proto().GetDetails() {
		if detail.MessageIs(&errdetails.ErrorInfo{}) {
			info := &errdetails.ErrorInfo{}
			if err := detail.UnmarshalTo(info); err != nil {
				t.Fatalf("unmarshal ErrorInfo: %v", err)
			}
			if info.GetDomain() != "ecommerce.catalog.v1" || info.GetReason() != wantReason {
				t.Fatalf("ErrorInfo = domain=%q reason=%q, want ecommerce.catalog.v1 / %q", info.GetDomain(), info.GetReason(), wantReason)
			}
			return
		}
	}
	t.Fatalf("expected google.rpc.ErrorInfo detail %q on %v", wantReason, err)
}

func TestSoftDeleteProductChangesDeletedFlag(t *testing.T) {
	repo := &memoryRepo{products: demoProducts()}
	auth := &fakeAuthorization{allowed: true}
	client := newTestClientWithAuth(t, repo, auth)

	_, err := client.SoftDeleteProduct(context.Background(), &catalogv1.SoftDeleteProductRequest{
		Id:      "dt-1",
		Deleted: true,
		UserId:  "admin-1",
	})
	if err != nil {
		t.Fatalf("SoftDeleteProduct: %v", err)
	}
	if auth.calls != 1 || auth.userID != "admin-1" || auth.permission != "product.delete" {
		t.Fatalf("authorization = calls=%d user=%q permission=%q, want 1/admin-1/product.delete", auth.calls, auth.userID, auth.permission)
	}

	product, err := client.GetProduct(context.Background(), &catalogv1.GetProductRequest{Id: "dt-1"})
	if err != nil || !product.GetDeleted() {
		t.Fatalf("expected deleted product, response=%+v err=%v", product, err)
	}
}

func TestSoftDeleteProductValidationSkipsAuthorizationAndRepo(t *testing.T) {
	cases := []struct {
		name string
		req  *catalogv1.SoftDeleteProductRequest
	}{
		{name: "blank id", req: &catalogv1.SoftDeleteProductRequest{Id: " ", Deleted: true, UserId: "admin-1"}},
		{name: "blank user_id", req: &catalogv1.SoftDeleteProductRequest{Id: "dt-1", Deleted: true, UserId: " "}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &memoryRepo{products: demoProducts()}
			auth := &fakeAuthorization{allowed: true}
			client := newTestClientWithAuth(t, repo, auth)

			_, err := client.SoftDeleteProduct(context.Background(), tc.req)
			if status.Code(err) != codes.InvalidArgument {
				t.Fatalf("SoftDeleteProduct() code = %v, want InvalidArgument", status.Code(err))
			}
			if auth.calls != 0 || repo.softDeleteCalls != 0 {
				t.Fatalf("side effects on invalid input: auth=%d repo=%d, want 0/0", auth.calls, repo.softDeleteCalls)
			}
		})
	}
}

func TestSoftDeleteProductDeniedLeavesProductUntouched(t *testing.T) {
	repo := &memoryRepo{products: demoProducts()}
	auth := &fakeAuthorization{allowed: false}
	client := newTestClientWithAuth(t, repo, auth)

	_, err := client.SoftDeleteProduct(context.Background(), &catalogv1.SoftDeleteProductRequest{
		Id:      "dt-1",
		Deleted: true,
		UserId:  "customer-1",
	})
	assertErrorReason(t, err, codes.PermissionDenied, "PRODUCT_PERMISSION_DENIED")
	if auth.calls != 1 || auth.userID != "customer-1" || auth.permission != "product.delete" {
		t.Fatalf("authorization = calls=%d user=%q permission=%q, want 1/customer-1/product.delete", auth.calls, auth.userID, auth.permission)
	}
	if repo.softDeleteCalls != 0 || repo.products[0].Deleted {
		t.Fatalf("denied SoftDeleteProduct mutated repository: calls=%d deleted=%t", repo.softDeleteCalls, repo.products[0].Deleted)
	}
}

func TestSoftDeleteProductAuthorizationOutageLeavesProductUntouched(t *testing.T) {
	repo := &memoryRepo{products: demoProducts()}
	auth := &fakeAuthorization{err: status.Error(codes.Unavailable, "identity unavailable")}
	client := newTestClientWithAuth(t, repo, auth)

	_, err := client.SoftDeleteProduct(context.Background(), &catalogv1.SoftDeleteProductRequest{
		Id:      "dt-1",
		Deleted: true,
		UserId:  "admin-1",
	})
	assertErrorReason(t, err, codes.Unavailable, "AUTHORIZATION_UNAVAILABLE")
	if repo.softDeleteCalls != 0 || repo.products[0].Deleted {
		t.Fatalf("unavailable SoftDeleteProduct mutated repository: calls=%d deleted=%t", repo.softDeleteCalls, repo.products[0].Deleted)
	}
}

func TestSoftDeleteProductNilAuthorizationFailsClosed(t *testing.T) {
	repo := &memoryRepo{products: demoProducts()}
	client := newTestClientWithAuth(t, repo, nil)

	_, err := client.SoftDeleteProduct(context.Background(), &catalogv1.SoftDeleteProductRequest{
		Id:      "dt-1",
		Deleted: true,
		UserId:  "admin-1",
	})
	assertErrorReason(t, err, codes.Unavailable, "AUTHORIZATION_UNAVAILABLE")
	if repo.softDeleteCalls != 0 {
		t.Fatalf("nil authorization mutated repository: calls=%d", repo.softDeleteCalls)
	}
}

func TestSoftDeleteProductPreservesCallerCancellation(t *testing.T) {
	repo := &memoryRepo{products: demoProducts()}
	srv := NewServer(repo, &fakeAuthorization{err: context.Canceled})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := srv.SoftDeleteProduct(ctx, &catalogv1.SoftDeleteProductRequest{
		Id:      "dt-1",
		Deleted: true,
		UserId:  "admin-1",
	})
	if status.Code(err) != codes.Canceled {
		t.Fatalf("SoftDeleteProduct() code = %v, want Canceled", status.Code(err))
	}
	if repo.softDeleteCalls != 0 {
		t.Fatalf("canceled SoftDeleteProduct mutated repository: calls=%d", repo.softDeleteCalls)
	}
}

type traceRecordingAuthorizationServer struct {
	identityv1.UnimplementedAuthorizationServiceServer
	traceID string
}

func (s *traceRecordingAuthorizationServer) CheckPermission(ctx context.Context, _ *identityv1.CheckPermissionRequest) (*identityv1.CheckPermissionResponse, error) {
	if values := metadata.ValueFromIncomingContext(ctx, "x-trace-id"); len(values) == 1 {
		s.traceID = values[0]
	}
	return nil, status.Error(codes.Unavailable, "identity unavailable")
}

func TestSoftDeleteProductForwardsIncomingTraceToIdentityAndLogsIt(t *testing.T) {
	previousLogger := slog.Default()
	var logs bytes.Buffer
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previousLogger) })

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen for Identity authorization: %v", err)
	}
	identityServer := &traceRecordingAuthorizationServer{}
	grpcServer := grpc.NewServer()
	identityv1.RegisterAuthorizationServiceServer(grpcServer, identityServer)
	go func() { _ = grpcServer.Serve(listener) }()
	t.Cleanup(func() {
		grpcServer.Stop()
		_ = listener.Close()
	})

	authClient, err := authorization.NewClient(listener.Addr().String(), time.Second)
	if err != nil {
		t.Fatalf("create authorization client: %v", err)
	}
	t.Cleanup(func() { _ = authClient.Close() })

	repo := &memoryRepo{products: demoProducts()}
	client := newTestClientWithAuth(t, repo, authClient)
	ctx := metadata.AppendToOutgoingContext(context.Background(), "x-trace-id", "trace-abc-123")

	_, err = client.SoftDeleteProduct(ctx, &catalogv1.SoftDeleteProductRequest{
		Id:      "dt-1",
		Deleted: true,
		UserId:  "admin-1",
	})
	assertErrorReason(t, err, codes.Unavailable, "AUTHORIZATION_UNAVAILABLE")
	if identityServer.traceID != "trace-abc-123" {
		t.Fatalf("Identity metadata trace = %q, want trace-abc-123", identityServer.traceID)
	}
	if !strings.Contains(logs.String(), `"trace_id":"trace-abc-123"`) {
		t.Fatalf("authorization log = %s, want propagated trace_id", logs.String())
	}
}

func TestGetProductNotFound(t *testing.T) {
	client := newTestClient(t, &memoryRepo{products: demoProducts()})

	_, err := client.GetProduct(context.Background(), &catalogv1.GetProductRequest{Id: "missing"})
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected a gRPC status error, got %v", err)
	}
	if st.Code() != codes.NotFound {
		t.Fatalf("expected codes.NotFound, got %v", st.Code())
	}

	var info *errdetails.ErrorInfo
	for _, detail := range st.Proto().GetDetails() {
		if detail.MessageIs(&errdetails.ErrorInfo{}) {
			info = &errdetails.ErrorInfo{}
			if err := detail.UnmarshalTo(info); err != nil {
				t.Fatalf("unmarshal ErrorInfo: %v", err)
			}
		}
	}
	if info == nil {
		t.Fatal("expected google.rpc.ErrorInfo detail on NotFound")
	}
	if info.GetReason() != "PRODUCT_NOT_FOUND" {
		t.Fatalf("expected reason PRODUCT_NOT_FOUND, got %q", info.GetReason())
	}
	if info.GetMetadata()["product_id"] != "missing" {
		t.Fatalf("expected product_id metadata, got %v", info.GetMetadata())
	}
}

func TestGetProductEmptyIDInvalidArgument(t *testing.T) {
	client := newTestClient(t, &memoryRepo{products: demoProducts()})

	_, err := client.GetProduct(context.Background(), &catalogv1.GetProductRequest{Id: ""})
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected a gRPC status error, got %v", err)
	}
	if st.Code() != codes.InvalidArgument {
		t.Fatalf("expected codes.InvalidArgument, got %v", st.Code())
	}
}

func TestSearchProductsHappy(t *testing.T) {
	client := newTestClient(t, &memoryRepo{products: demoProducts()})

	resp, err := client.SearchProducts(context.Background(), &catalogv1.SearchProductsRequest{
		CategoryId: "dien-thoai",
		Page:       1,
		PageSize:   10,
	})
	if err != nil {
		t.Fatalf("SearchProducts: %v", err)
	}

	if resp.GetTotalItems() != 2 || resp.GetTotalPages() != 1 {
		t.Fatalf("unexpected totals: %+v", resp)
	}
	if resp.GetPage() != 1 || resp.GetPageSize() != 10 {
		t.Fatalf("unexpected pagination: %+v", resp)
	}
	if len(resp.GetItems()) != 2 {
		t.Fatalf("expected 2 items, got %d", len(resp.GetItems()))
	}
	// Default popularity order: dt-2 (8920 sold) before dt-1 (2310).
	if resp.GetItems()[0].GetId() != "dt-2" || resp.GetItems()[1].GetId() != "dt-1" {
		t.Fatalf("unexpected order: %v", resp.GetItems())
	}
	item := resp.GetItems()[0]
	if item.GetPrice().GetCurrency() != "VND" || item.GetPrice().GetAmountMinor() != 5490000 {
		t.Fatalf("unexpected summary price: %+v", item.GetPrice())
	}
	if len(item.GetImageHue()) != 2 {
		t.Fatalf("expected imageHue in summary, got %v", item.GetImageHue())
	}
}

func TestSearchProductsSortAndQuery(t *testing.T) {
	client := newTestClient(t, &memoryRepo{products: demoProducts()})

	resp, err := client.SearchProducts(context.Background(), &catalogv1.SearchProductsRequest{
		Query: "nova",
		Sort:  "gia-cao",
	})
	if err != nil {
		t.Fatalf("SearchProducts: %v", err)
	}
	if resp.GetTotalItems() != 1 || len(resp.GetItems()) != 1 {
		t.Fatalf("expected one Nova match, got %+v", resp)
	}
	if resp.GetItems()[0].GetId() != "dt-1" {
		t.Fatalf("expected dt-1, got %v", resp.GetItems()[0].GetId())
	}
}

func TestSearchProductsInvalidParams(t *testing.T) {
	client := newTestClient(t, &memoryRepo{products: demoProducts()})

	cases := []struct {
		name string
		req  *catalogv1.SearchProductsRequest
	}{
		{name: "negative rating", req: &catalogv1.SearchProductsRequest{RatingMin: -1}},
		{name: "negative price min", req: &catalogv1.SearchProductsRequest{PriceMin: -1000}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := client.SearchProducts(context.Background(), tc.req)
			st, ok := status.FromError(err)
			if !ok {
				t.Fatalf("expected a gRPC status error, got %v", err)
			}
			if st.Code() != codes.InvalidArgument {
				t.Fatalf("expected codes.InvalidArgument, got %v", st.Code())
			}
		})
	}
}

func TestSearchProductsEmptyResult(t *testing.T) {
	client := newTestClient(t, &memoryRepo{products: demoProducts()})

	resp, err := client.SearchProducts(context.Background(), &catalogv1.SearchProductsRequest{
		Query: "does-not-exist",
	})
	if err != nil {
		t.Fatalf("SearchProducts: %v", err)
	}
	if len(resp.GetItems()) != 0 || resp.GetTotalItems() != 0 || resp.GetTotalPages() != 0 {
		t.Fatalf("expected empty result, got %+v", resp)
	}
}

func TestGetProductAndSearchProductsExposeVersion(t *testing.T) {
	products := demoProducts()
	products[0].Version = 5
	client := newTestClient(t, &memoryRepo{products: products})

	detail, err := client.GetProduct(context.Background(), &catalogv1.GetProductRequest{Id: "dt-1"})
	if err != nil {
		t.Fatalf("GetProduct: %v", err)
	}
	if detail.GetVersion() != 5 {
		t.Fatalf("GetProduct.GetVersion() = %d, want 5", detail.GetVersion())
	}

	search, err := client.SearchProducts(context.Background(), &catalogv1.SearchProductsRequest{Query: "Nova"})
	if err != nil {
		t.Fatalf("SearchProducts: %v", err)
	}
	if len(search.GetItems()) != 1 || search.GetItems()[0].GetVersion() != 5 {
		t.Fatalf("SearchProducts item version = %+v, want 5", search.GetItems())
	}

	adminList, err := client.ListAdminProducts(context.Background(), &catalogv1.ListAdminProductsRequest{
		UserId: "admin-1",
		Query:  "dt-1",
	})
	if err != nil {
		t.Fatalf("ListAdminProducts: %v", err)
	}
	if len(adminList.GetItems()) != 1 || adminList.GetItems()[0].GetVersion() != 5 {
		t.Fatalf("ListAdminProducts item version = %+v, want 5", adminList.GetItems())
	}
}
