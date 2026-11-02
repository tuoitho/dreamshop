package grpc

import (
	"context"
	"errors"
	"net"
	"sort"
	"testing"
	"time"

	inventoryv1 "ecommerce.local/contracts/go/inventory/v1"
	"ecommerce.local/inventory-service/internal/domain"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

type memoryRepo struct {
	stocks map[string]domain.Stock
	err    error
}

func (m *memoryRepo) Get(_ context.Context, skuID string) (domain.Stock, error) {
	if m.err != nil {
		return domain.Stock{}, m.err
	}
	stock, ok := m.stocks[skuID]
	if !ok {
		return domain.Stock{SkuID: skuID}, nil
	}
	return stock, nil
}

func (m *memoryRepo) BatchGet(ctx context.Context, skuIDs []string) ([]domain.Stock, error) {
	if m.err != nil {
		return nil, m.err
	}
	out := make([]domain.Stock, len(skuIDs))
	for i, id := range skuIDs {
		st, _ := m.Get(ctx, id)
		out[i] = st
	}
	return out, nil
}

func (m *memoryRepo) List(_ context.Context) ([]domain.Stock, error) {
	if m.err != nil {
		return nil, m.err
	}
	out := make([]domain.Stock, 0, len(m.stocks))
	for _, st := range m.stocks {
		out = append(out, st)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SkuID < out[j].SkuID })
	return out, nil
}

func (m *memoryRepo) SetQuantity(_ context.Context, skuID, productID string, quantity int32) (domain.Stock, error) {
	if m.err != nil {
		return domain.Stock{}, m.err
	}
	cur := m.stocks[skuID]
	if quantity < 0 || quantity < cur.Reserved {
		return domain.Stock{}, domain.ErrStockBelowReserved
	}
	cur.SkuID = skuID
	cur.ProductID = productID
	cur.Quantity = quantity
	cur.UpdatedAt = time.Now().UTC()
	m.stocks[skuID] = cur
	return cur, nil
}

func (m *memoryRepo) AdjustQuantity(_ context.Context, skuID, productID string, delta int32) (domain.Stock, error) {
	if m.err != nil {
		return domain.Stock{}, m.err
	}
	cur, ok := m.stocks[skuID]
	if !ok && delta < 0 {
		return domain.Stock{}, domain.ErrStockBelowReserved
	}
	next := cur.Quantity + delta
	if next < cur.Reserved {
		return domain.Stock{}, domain.ErrStockBelowReserved
	}
	cur.SkuID = skuID
	cur.ProductID = productID
	cur.Quantity = next
	cur.UpdatedAt = time.Now().UTC()
	m.stocks[skuID] = cur
	return cur, nil
}

func (m *memoryRepo) DeleteUnreserved(_ context.Context, skuIDs []string, dryRun bool) error {
	if m.err != nil {
		return m.err
	}
	sorted := append([]string(nil), skuIDs...)
	sort.Strings(sorted)
	for _, id := range sorted {
		if st, ok := m.stocks[id]; ok && st.Reserved > 0 {
			return &domain.VariantHasReservedStockError{SkuID: id}
		}
	}
	if !dryRun {
		for _, id := range sorted {
			delete(m.stocks, id)
		}
	}
	return nil
}

func newTestClient(t *testing.T, repo domain.Repository) inventoryv1.InventoryServiceClient {
	t.Helper()

	lis := bufconn.Listen(1024 * 1024)
	srv := grpc.NewServer()
	inventoryv1.RegisterInventoryServiceServer(srv, NewServer(repo))
	go func() {
		if err := srv.Serve(lis); err != nil {
			t.Errorf("grpc serve: %v", err)
		}
	}()
	t.Cleanup(func() {
		srv.Stop()
		_ = lis.Close()
	})

	conn, err := grpc.NewClient("passthrough:///inventory-service-test",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("grpc dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	return inventoryv1.NewInventoryServiceClient(conn)
}

func TestGetAvailabilityReturnsAvailableQuantity(t *testing.T) {
	client := newTestClient(t, &memoryRepo{stocks: map[string]domain.Stock{
		"dt-1-v1": {SkuID: "dt-1-v1", ProductID: "dt-1", Quantity: 7, Reserved: 2},
	}})

	resp, err := client.GetAvailability(context.Background(), &inventoryv1.GetAvailabilityRequest{SkuId: "dt-1-v1"})
	if err != nil {
		t.Fatalf("GetAvailability: %v", err)
	}
	if resp.GetSkuId() != "dt-1-v1" || resp.GetProductId() != "dt-1" || resp.GetAvailableQuantity() != 5 {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestBatchGetAvailabilityPreservesInputOrderWithoutAuth(t *testing.T) {
	client := newTestClient(t, &memoryRepo{stocks: map[string]domain.Stock{
		"dt-1-v1": {SkuID: "dt-1-v1", ProductID: "dt-1", Quantity: 7, Reserved: 2},
		"dt-1-v3": {SkuID: "dt-1-v3", ProductID: "dt-1", Quantity: 2, Reserved: 2},
	}})

	resp, err := client.BatchGetAvailability(context.Background(), &inventoryv1.BatchGetAvailabilityRequest{
		SkuIds: []string{"missing-sku", "dt-1-v1", "dt-1-v3"},
	})
	if err != nil {
		t.Fatalf("BatchGetAvailability: %v", err)
	}
	if len(resp.GetItems()) != 3 {
		t.Fatalf("expected 3 items, got %d", len(resp.GetItems()))
	}
	if resp.GetItems()[0].GetSkuId() != "missing-sku" || resp.GetItems()[0].GetAvailableQuantity() != 0 ||
		resp.GetItems()[1].GetSkuId() != "dt-1-v1" || resp.GetItems()[1].GetProductId() != "dt-1" || resp.GetItems()[1].GetAvailableQuantity() != 5 ||
		resp.GetItems()[2].GetSkuId() != "dt-1-v3" || resp.GetItems()[2].GetAvailableQuantity() != 0 {
		t.Fatalf("unexpected BatchGetAvailability items: %+v", resp.GetItems())
	}
}

func TestGetAvailabilityRejectsEmptySkuID(t *testing.T) {
	client := newTestClient(t, &memoryRepo{stocks: map[string]domain.Stock{}})

	_, err := client.GetAvailability(context.Background(), &inventoryv1.GetAvailabilityRequest{SkuId: "  "})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected codes.InvalidArgument, got %v", err)
	}
}

func TestGetAvailabilityMapsRepositoryErrorToInternal(t *testing.T) {
	client := newTestClient(t, &memoryRepo{err: errors.New("database unavailable")})

	_, err := client.GetAvailability(context.Background(), &inventoryv1.GetAvailabilityRequest{SkuId: "dt-1-v1"})
	if status.Code(err) != codes.Internal {
		t.Fatalf("expected codes.Internal, got %v", err)
	}
}
