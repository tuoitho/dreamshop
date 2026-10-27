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

func (m *memoryRepo) Get(_ context.Context, productID string) (domain.Stock, error) {
	if m.err != nil {
		return domain.Stock{}, m.err
	}
	stock, ok := m.stocks[productID]
	if !ok {
		return domain.Stock{ProductID: productID}, nil
	}
	return stock, nil
}

func (m *memoryRepo) BatchGet(ctx context.Context, productIDs []string) ([]domain.Stock, error) {
	if m.err != nil {
		return nil, m.err
	}
	out := make([]domain.Stock, len(productIDs))
	for i, id := range productIDs {
		st, _ := m.Get(ctx, id)
		out[i] = st
	}
	return out, nil
}

func (m *memoryRepo) ListAll(_ context.Context) ([]domain.Stock, error) {
	if m.err != nil {
		return nil, m.err
	}
	out := make([]domain.Stock, 0, len(m.stocks))
	for _, st := range m.stocks {
		out = append(out, st)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ProductID < out[j].ProductID })
	return out, nil
}

func (m *memoryRepo) SetQuantity(_ context.Context, productID string, quantity int32) (domain.Stock, error) {
	if m.err != nil {
		return domain.Stock{}, m.err
	}
	cur := m.stocks[productID]
	if quantity < 0 || quantity < cur.Reserved {
		return domain.Stock{}, domain.ErrStockBelowReserved
	}
	cur.ProductID = productID
	cur.Quantity = quantity
	cur.UpdatedAt = time.Now().UTC()
	m.stocks[productID] = cur
	return cur, nil
}

func (m *memoryRepo) AdjustQuantity(_ context.Context, productID string, delta int32) (domain.Stock, error) {
	if m.err != nil {
		return domain.Stock{}, m.err
	}
	cur, ok := m.stocks[productID]
	if !ok && delta < 0 {
		return domain.Stock{}, domain.ErrStockBelowReserved
	}
	next := cur.Quantity + delta
	if next < cur.Reserved {
		return domain.Stock{}, domain.ErrStockBelowReserved
	}
	cur.ProductID = productID
	cur.Quantity = next
	cur.UpdatedAt = time.Now().UTC()
	m.stocks[productID] = cur
	return cur, nil
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
		"dt-1": {ProductID: "dt-1", Quantity: 7, Reserved: 2},
	}})

	resp, err := client.GetAvailability(context.Background(), &inventoryv1.GetAvailabilityRequest{ProductId: "dt-1"})
	if err != nil {
		t.Fatalf("GetAvailability: %v", err)
	}
	if resp.GetProductId() != "dt-1" || resp.GetAvailableQuantity() != 5 {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestGetAvailabilityReturnsZeroForMissingOrFullyReservedStock(t *testing.T) {
	client := newTestClient(t, &memoryRepo{stocks: map[string]domain.Stock{
		"dt-2": {ProductID: "dt-2", Quantity: 2, Reserved: 4},
	}})

	for _, productID := range []string{"dt-2", "missing"} {
		t.Run(productID, func(t *testing.T) {
			resp, err := client.GetAvailability(context.Background(), &inventoryv1.GetAvailabilityRequest{ProductId: productID})
			if err != nil {
				t.Fatalf("GetAvailability: %v", err)
			}
			if resp.GetProductId() != productID || resp.GetAvailableQuantity() != 0 {
				t.Fatalf("unexpected response: %+v", resp)
			}
		})
	}
}

func TestGetAvailabilityRejectsEmptyProductID(t *testing.T) {
	client := newTestClient(t, &memoryRepo{stocks: map[string]domain.Stock{}})

	_, err := client.GetAvailability(context.Background(), &inventoryv1.GetAvailabilityRequest{ProductId: "  "})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected codes.InvalidArgument, got %v", err)
	}
}

func TestGetAvailabilityMapsRepositoryErrorToInternal(t *testing.T) {
	client := newTestClient(t, &memoryRepo{err: errors.New("database unavailable")})

	_, err := client.GetAvailability(context.Background(), &inventoryv1.GetAvailabilityRequest{ProductId: "dt-1"})
	if status.Code(err) != codes.Internal {
		t.Fatalf("expected codes.Internal, got %v", err)
	}
}
