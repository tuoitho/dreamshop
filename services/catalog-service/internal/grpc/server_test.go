package grpc

import (
	"context"
	"net"
	"testing"

	"ecommerce.local/catalog-service/internal/domain"
	catalogv1 "ecommerce.local/contracts/go/catalog/v1"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

// memoryRepo is a domain.Repository backed by the pure in-memory search so
// handler tests stay free of infrastructure.
type memoryRepo struct {
	products []*domain.Product
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
				{Label: "Đen", ColorHex: "#1f2937"},
				{Label: "Xanh", ColorHex: "#1e40af"},
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
			Rating:             4.7,
			ReviewCount:        3521,
			SoldCount:          8920,
			Emoji:              "📲",
			ImageHue:           []uint32{150, 190},
		},
	}
}

func newTestClient(t *testing.T, repo domain.Repository) catalogv1.CatalogServiceClient {
	t.Helper()

	lis := bufconn.Listen(1024 * 1024)
	srv := grpc.NewServer()
	catalogv1.RegisterCatalogServiceServer(srv, NewServer(repo))
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
	if len(resp.GetVariants()) != 2 || resp.GetVariants()[0].GetColor() != "Đen" || resp.GetVariants()[0].GetColorHex() != "#1f2937" {
		t.Fatalf("unexpected variants: %v", resp.GetVariants())
	}
	if len(resp.GetDetails()) != 1 || resp.GetDetails()[0] != "Màn hình: 6.8\" AMOLED 144Hz" {
		t.Fatalf("unexpected details: %v", resp.GetDetails())
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
