package grpc

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"ecommerce.local/cart-service/internal/authorization"
	"ecommerce.local/cart-service/internal/domain"
	cartstore "ecommerce.local/cart-service/internal/store"
	cartv1 "ecommerce.local/contracts/go/cart/v1"
	catalogv1 "ecommerce.local/contracts/go/catalog/v1"
	commonv1 "ecommerce.local/contracts/go/common/v1"
	identityv1 "ecommerce.local/contracts/go/identity/v1"
	inventoryv1 "ecommerce.local/contracts/go/inventory/v1"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

const (
	testBufSize = 1024 * 1024
	testUserID  = "00000000-0000-0000-0000-000000000001"
	testItemID  = "00000000-0000-0000-0000-000000000011"
)

type fakeStore struct {
	cartID            string
	found             bool
	lines             []domain.Line
	favorites         []domain.FavoriteRecord
	ensureErr         error
	findErr           error
	listErr           error
	addErr            error
	updateErr         error
	removeErr         error
	clearErr          error
	listFavoritesErr  error
	addFavoriteErr    error
	removeFavoriteErr error
	clearFavoritesErr error
	ensureUserID      string
	findUserID        string
	addArgs           []any
	updateArgs        []any
	removeArgs        []any
	clearCartID       string
	addFavoriteArgs   []string
	removeFavArgs     []string
	clearFavUserID    string
	events            *[]string
}

func (f *fakeStore) EnsureCart(_ context.Context, userID string) (string, error) {
	if f.events != nil {
		*f.events = append(*f.events, "ensure-cart")
	}
	f.ensureUserID = userID
	return f.cartID, f.ensureErr
}

func (f *fakeStore) FindCart(_ context.Context, userID string) (string, bool, error) {
	if f.events != nil {
		*f.events = append(*f.events, "find-cart")
	}
	f.findUserID = userID
	return f.cartID, f.found, f.findErr
}

func (f *fakeStore) ListItems(_ context.Context, cartID string) ([]domain.Line, error) {
	if cartID != f.cartID {
		return nil, errors.New("unexpected cart id")
	}
	return f.lines, f.listErr
}

func (f *fakeStore) AddItem(_ context.Context, cartID, productID, skuID string, quantity int32) error {
	f.addArgs = []any{cartID, productID, skuID, quantity}
	return f.addErr
}

func (f *fakeStore) UpdateQuantity(_ context.Context, cartID, itemID string, quantity int32) error {
	f.updateArgs = []any{cartID, itemID, quantity}
	return f.updateErr
}

func (f *fakeStore) RemoveItem(_ context.Context, cartID, itemID string) error {
	f.removeArgs = []any{cartID, itemID}
	return f.removeErr
}

func (f *fakeStore) Clear(_ context.Context, cartID string) error {
	f.clearCartID = cartID
	return f.clearErr
}

func (f *fakeStore) ListFavorites(_ context.Context, _ string) ([]domain.FavoriteRecord, error) {
	return f.favorites, f.listFavoritesErr
}

func (f *fakeStore) AddFavorite(_ context.Context, userID, productID string) error {
	f.addFavoriteArgs = []string{userID, productID}
	if f.addFavoriteErr == nil {
		for _, rec := range f.favorites {
			if rec.ProductID == productID {
				return nil
			}
		}
		f.favorites = append([]domain.FavoriteRecord{{ID: testItemID, ProductID: productID}}, f.favorites...)
	}
	return f.addFavoriteErr
}

func (f *fakeStore) RemoveFavorite(_ context.Context, userID, productID string) error {
	f.removeFavArgs = []string{userID, productID}
	if f.removeFavoriteErr == nil {
		filtered := make([]domain.FavoriteRecord, 0, len(f.favorites))
		for _, rec := range f.favorites {
			if rec.ProductID != productID {
				filtered = append(filtered, rec)
			}
		}
		f.favorites = filtered
	}
	return f.removeFavoriteErr
}

func (f *fakeStore) ClearFavorites(_ context.Context, userID string) error {
	f.clearFavUserID = userID
	if f.clearFavoritesErr == nil {
		f.favorites = nil
	}
	return f.clearFavoritesErr
}

type fakeCatalog struct {
	mu       sync.Mutex
	products map[string]*catalogv1.GetProductResponse
	errors   map[string]error
	called   []string
}

func (f *fakeCatalog) GetProduct(_ context.Context, productID string) (*catalogv1.GetProductResponse, error) {
	f.mu.Lock()
	f.called = append(f.called, productID)
	f.mu.Unlock()
	if err := f.errors[productID]; err != nil {
		return nil, err
	}
	product, ok := f.products[productID]
	if !ok {
		return nil, status.Error(codes.NotFound, "product not found")
	}
	return product, nil
}

type fakeInventory struct {
	mu           sync.Mutex
	availability map[string]int32
	batchErr     error
	batches      [][]string
}

func (f *fakeInventory) BatchGetAvailability(_ context.Context, skuIDs []string) ([]*inventoryv1.AvailabilityItem, error) {
	f.mu.Lock()
	copied := append([]string(nil), skuIDs...)
	f.batches = append(f.batches, copied)
	f.mu.Unlock()
	if f.batchErr != nil {
		return nil, f.batchErr
	}
	items := make([]*inventoryv1.AvailabilityItem, 0, len(skuIDs))
	for _, skuID := range skuIDs {
		avail := int32(0)
		if f.availability != nil {
			avail = f.availability[skuID]
		}
		items = append(items, &inventoryv1.AvailabilityItem{
			SkuId:             skuID,
			AvailableQuantity: avail,
		})
	}
	return items, nil
}

type fakeAuthorization struct {
	allowed    bool
	err        error
	userID     string
	permission string
	calls      int
	events     *[]string
}

func (f *fakeAuthorization) CheckPermission(_ context.Context, userID, permission string) (bool, error) {
	if f.events != nil {
		*f.events = append(*f.events, "check-permission")
	}
	f.calls++
	f.userID = userID
	f.permission = permission
	return f.allowed, f.err
}

type blockingCatalog struct {
	entered chan<- struct{}
	release <-chan struct{}
	current atomic.Int32
	max     atomic.Int32
}

func (f *blockingCatalog) GetProduct(_ context.Context, productID string) (*catalogv1.GetProductResponse, error) {
	current := f.current.Add(1)
	for {
		max := f.max.Load()
		if current <= max || f.max.CompareAndSwap(max, current) {
			break
		}
	}
	f.entered <- struct{}{}
	<-f.release
	f.current.Add(-1)
	return validProduct(productID), nil
}

func TestGetCartEnrichesDistinctProductsInParallelWithBoundedConcurrencyAndLineOrder(t *testing.T) {
	lines := make([]domain.Line, 10)
	for i := range lines {
		pid := "product-" + string(rune('a'+i))
		lines[i] = domain.Line{ID: "item-" + string(rune('a'+i)), ProductID: pid, SkuID: pid + "-v1", Quantity: 1}
	}
	store := &fakeStore{cartID: "cart-1", lines: lines}
	entered := make(chan struct{}, len(lines))
	release := make(chan struct{})
	catalog := &blockingCatalog{entered: entered, release: release}
	inventory := &fakeInventory{availability: map[string]int32{}}

	done := make(chan struct{})
	var cart *cartv1.Cart
	var err error
	go func() {
		cart, err = NewServer(store, catalog, inventory, &fakeAuthorization{allowed: true}).getCart(context.Background(), "cart-1")
		close(done)
	}()

	for i := 0; i < 8; i++ {
		select {
		case <-entered:
		case <-time.After(time.Second):
			close(release)
			<-done
			t.Fatalf("only %d catalog calls entered the bounded fan-out", i)
		}
	}
	close(release)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("bounded fan-out did not complete after dependencies were released")
	}
	if err != nil {
		t.Fatalf("getCart() error = %v", err)
	}
	if got := catalog.max.Load(); got != 8 {
		t.Fatalf("maximum concurrent catalog calls = %d, want 8", got)
	}
	if len(inventory.batches) != 1 {
		t.Fatalf("inventory batch calls = %d, want 1", len(inventory.batches))
	}
	if len(cart.GetItems()) != len(lines) {
		t.Fatalf("got %d items, want %d", len(cart.GetItems()), len(lines))
	}
	for i, item := range cart.GetItems() {
		if item.GetItemId() != lines[i].ID || item.GetProductId() != lines[i].ProductID || item.GetSkuId() != lines[i].SkuID {
			t.Fatalf("item %d = %q/%q/%q, want %q/%q/%q", i, item.GetItemId(), item.GetProductId(), item.GetSkuId(), lines[i].ID, lines[i].ProductID, lines[i].SkuID)
		}
	}
}

func TestGetCartEnrichesDistinctVariantPricesAndAvailabilityViaSingleBatch(t *testing.T) {
	product := &catalogv1.GetProductResponse{
		Id:            "dt-1",
		Name:          "Apex Pro",
		Brand:         "Apex",
		CategoryName:  "Điện thoại",
		Price:         &commonv1.Money{Currency: "VND", AmountMinor: 18990000},
		OriginalPrice: &commonv1.Money{Currency: "VND", AmountMinor: 21990000},
		MaxPrice:      &commonv1.Money{Currency: "VND", AmountMinor: 23990000},
		Emoji:         "📱",
		ImageHue:      []uint32{10, 20},
		Images:        []string{"https://example.test/dt-1.png"},
		Variants: []*catalogv1.ProductVariant{
			{
				SkuId:         "dt-1-v1",
				Label:         "12GB/256GB",
				ColorHex:      "#1c1c1e",
				Price:         &commonv1.Money{Currency: "VND", AmountMinor: 18990000},
				OriginalPrice: &commonv1.Money{Currency: "VND", AmountMinor: 21990000},
			},
			{
				SkuId:         "dt-1-v3",
				Label:         "16GB/1TB",
				ColorHex:      "#f5f5f7",
				Price:         &commonv1.Money{Currency: "VND", AmountMinor: 23990000},
				OriginalPrice: &commonv1.Money{Currency: "VND", AmountMinor: 26990000},
			},
		},
	}
	store := &fakeStore{cartID: "cart-1", lines: []domain.Line{
		{ID: "item-1", ProductID: "dt-1", SkuID: "dt-1-v1", Quantity: 2},
		{ID: "item-2", ProductID: "dt-1", SkuID: "dt-1-v3", Quantity: 1},
		{ID: "item-3", ProductID: "dt-1", SkuID: "dt-1-removed", Quantity: 1},
	}}
	catalog := &fakeCatalog{products: map[string]*catalogv1.GetProductResponse{"dt-1": product}, errors: map[string]error{}}
	inventory := &fakeInventory{availability: map[string]int32{
		"dt-1-v1": 5,
		"dt-1-v3": 0,
	}}

	cart, err := NewServer(store, catalog, inventory, &fakeAuthorization{allowed: true}).getCart(context.Background(), "cart-1")
	if err != nil {
		t.Fatalf("getCart() error = %v", err)
	}
	if len(catalog.called) != 1 {
		t.Fatalf("catalog calls = %d, want 1 distinct product", len(catalog.called))
	}
	if len(inventory.batches) != 1 || len(inventory.batches[0]) != 2 {
		t.Fatalf("inventory batches = %v, want single batch of 2 active SKUs", inventory.batches)
	}

	first := cart.GetItems()[0]
	if first.GetSkuId() != "dt-1-v1" ||
		first.GetVariantLabel() != "12GB/256GB" ||
		first.GetVariantColorHex() != "#1c1c1e" ||
		first.GetUnitPrice().GetAmountMinor() != 18990000 ||
		first.GetOriginalPrice().GetAmountMinor() != 21990000 ||
		first.GetAvailableQuantity() != 5 ||
		first.GetStatus() != cartv1.CartItemStatus_CART_ITEM_STATUS_AVAILABLE {
		t.Fatalf("first variant line = %+v", first)
	}

	second := cart.GetItems()[1]
	if second.GetSkuId() != "dt-1-v3" ||
		second.GetVariantLabel() != "16GB/1TB" ||
		second.GetUnitPrice().GetAmountMinor() != 23990000 ||
		second.GetOriginalPrice().GetAmountMinor() != 26990000 ||
		second.GetAvailableQuantity() != 0 ||
		second.GetStatus() != cartv1.CartItemStatus_CART_ITEM_STATUS_OUT_OF_STOCK {
		t.Fatalf("second variant line = %+v", second)
	}

	removed := cart.GetItems()[2]
	if removed.GetSkuId() != "dt-1-removed" || removed.GetStatus() != cartv1.CartItemStatus_CART_ITEM_STATUS_UNAVAILABLE {
		t.Fatalf("removed variant line = %+v, want UNAVAILABLE with sku_id preserved", removed)
	}
}

func TestGetCartDeduplicatesCatalogFailureAndPreservesSkuIdOnDegradedLines(t *testing.T) {
	store := &fakeStore{cartID: "cart-1", lines: []domain.Line{
		{ID: "item-1", ProductID: "down", SkuID: "down-v1", Quantity: 1},
		{ID: "item-2", ProductID: "down", SkuID: "down-v2", Quantity: 2},
		{ID: "item-3", ProductID: "ok", SkuID: "ok-v1", Quantity: 1},
	}}
	catalog := &fakeCatalog{
		products: map[string]*catalogv1.GetProductResponse{"ok": validProduct("ok")},
		errors:   map[string]error{"down": status.Error(codes.Unavailable, "catalog unavailable")},
	}
	inventory := &fakeInventory{availability: map[string]int32{
		"ok-v1": 1,
	}}

	cart, err := NewServer(store, catalog, inventory, &fakeAuthorization{allowed: true}).getCart(context.Background(), "cart-1")
	if err != nil {
		t.Fatalf("getCart() error = %v", err)
	}
	if !cart.GetDegraded() {
		t.Fatal("getCart() degraded = false, want true")
	}
	if cart.GetItems()[0].GetSkuId() != "down-v1" || cart.GetItems()[1].GetSkuId() != "down-v2" {
		t.Fatalf("degraded lines lost sku_id: %+v, %+v", cart.GetItems()[0], cart.GetItems()[1])
	}
	for i := 0; i < 2; i++ {
		if cart.GetItems()[i].GetStatus() != cartv1.CartItemStatus_CART_ITEM_STATUS_UNAVAILABLE {
			t.Fatalf("failed product line %d status = %v, want unavailable", i, cart.GetItems()[i].GetStatus())
		}
	}
	if len(catalog.called) != 2 {
		t.Fatalf("catalog calls = %d, want one per product", len(catalog.called))
	}
	if len(inventory.batches) != 1 || len(inventory.batches[0]) != 1 || inventory.batches[0][0] != "ok-v1" {
		t.Fatalf("inventory batches = %v, want [[ok-v1]]", inventory.batches)
	}
}

func newBufconnClient(t *testing.T, server *Server) cartv1.CartServiceClient {
	t.Helper()
	listener := bufconn.Listen(testBufSize)
	grpcServer := grpc.NewServer()
	cartv1.RegisterCartServiceServer(grpcServer, server)
	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			_ = err
		}
	}()
	t.Cleanup(func() {
		grpcServer.Stop()
		_ = listener.Close()
	})

	conn, err := grpc.NewClient("passthrough:///bufconn", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}), grpc.WithInsecure())
	if err != nil {
		t.Fatalf("create bufconn client: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return cartv1.NewCartServiceClient(conn)
}

func validProduct(id string) *catalogv1.GetProductResponse {
	return &catalogv1.GetProductResponse{
		Id:            id,
		Name:          "Phone",
		Brand:         "Brand",
		CategoryName:  "Phones",
		Price:         &commonv1.Money{Currency: "VND", AmountMinor: 12345},
		OriginalPrice: &commonv1.Money{Currency: "VND", AmountMinor: 15000},
		MaxPrice:      &commonv1.Money{Currency: "VND", AmountMinor: 12345},
		Emoji:         "📱",
		ImageHue:      []uint32{10, 20},
		Images:        []string{"https://example.test/phone.png"},
		Variants: []*catalogv1.ProductVariant{{
			SkuId:         id + "-v1",
			Label:         "red",
			ColorHex:      "#ff0000",
			Price:         &commonv1.Money{Currency: "VND", AmountMinor: 12345},
			OriginalPrice: &commonv1.Money{Currency: "VND", AmountMinor: 15000},
		}},
	}
}

func TestGetCartEnrichesLinesAndMapsAllStatuses(t *testing.T) {
	store := &fakeStore{
		cartID: "cart-1",
		found:  true,
		lines: []domain.Line{
			{ID: "item-available", ProductID: "p-available", SkuID: "p-available-v1", Quantity: 2},
			{ID: "item-limited", ProductID: "p-limited", SkuID: "p-limited-v1", Quantity: 3},
			{ID: "item-empty", ProductID: "p-empty", SkuID: "p-empty-v1", Quantity: 1},
			{ID: "item-variant", ProductID: "p-variant", SkuID: "p-variant-missing", Quantity: 4},
		},
	}
	catalog := &fakeCatalog{products: map[string]*catalogv1.GetProductResponse{
		"p-available": validProduct("p-available"),
		"p-limited":   validProduct("p-limited"),
		"p-empty":     validProduct("p-empty"),
		"p-variant":   validProduct("p-variant"),
	}, errors: map[string]error{}}
	inventory := &fakeInventory{
		availability: map[string]int32{
			"p-available-v1": 5,
			"p-limited-v1":   1,
			"p-empty-v1":     0,
		},
	}

	client := newBufconnClient(t, NewServer(store, catalog, inventory, &fakeAuthorization{allowed: true}))
	response, err := client.GetCart(context.Background(), &cartv1.GetCartRequest{UserId: testUserID})
	if err != nil {
		t.Fatalf("GetCart() error = %v", err)
	}
	if response.GetCart().GetDegraded() {
		t.Fatal("GetCart() unexpectedly degraded")
	}
	items := response.GetCart().GetItems()
	if len(items) != 4 {
		t.Fatalf("got %d items, want 4", len(items))
	}
	wantStatuses := []cartv1.CartItemStatus{
		cartv1.CartItemStatus_CART_ITEM_STATUS_AVAILABLE,
		cartv1.CartItemStatus_CART_ITEM_STATUS_LIMITED,
		cartv1.CartItemStatus_CART_ITEM_STATUS_OUT_OF_STOCK,
		cartv1.CartItemStatus_CART_ITEM_STATUS_UNAVAILABLE,
	}
	for i, item := range items {
		if item.GetStatus() != wantStatuses[i] {
			t.Errorf("item %d status = %v, want %v", i, item.GetStatus(), wantStatuses[i])
		}
	}
	limited := items[1]
	if limited.GetQuantity() != 3 || limited.GetAvailableQuantity() != 1 {
		t.Fatalf("limited item quantity/availability = %d/%d, want 3/1", limited.GetQuantity(), limited.GetAvailableQuantity())
	}
	if limited.GetName() != "Phone" || limited.GetBrand() != "Brand" || limited.GetCategoryName() != "Phones" || limited.GetUnitPrice().GetAmountMinor() != 12345 || limited.GetOriginalPrice().GetAmountMinor() != 15000 {
		t.Fatalf("valid product enrichment missing: %+v", limited)
	}
	if limited.GetImageUrl() != "https://example.test/phone.png" || limited.GetVariantColorHex() != "#ff0000" || limited.GetVariantLabel() != "red" || limited.GetSkuId() != "p-limited-v1" {
		t.Fatalf("image/variant enrichment missing: %+v", limited)
	}
	if len(inventory.batches) != 1 || len(inventory.batches[0]) != 3 {
		t.Fatalf("inventory batches = %v, want 1 batch of 3 SKUs", inventory.batches)
	}
}

func TestGetCartCatalogOutageDegradesOnlyAffectedLine(t *testing.T) {
	store := &fakeStore{cartID: "cart-1", found: true, lines: []domain.Line{
		{ID: "item-ok", ProductID: "p-ok", SkuID: "p-ok-v1", Quantity: 1},
		{ID: "item-down", ProductID: "p-down", SkuID: "p-down-v1", Quantity: 2},
	}}
	catalog := &fakeCatalog{
		products: map[string]*catalogv1.GetProductResponse{"p-ok": validProduct("p-ok")},
		errors:   map[string]error{"p-down": status.Error(codes.Unavailable, "catalog unavailable")},
	}
	inventory := &fakeInventory{availability: map[string]int32{
		"p-ok-v1": 1,
	}}

	client := newBufconnClient(t, NewServer(store, catalog, inventory, &fakeAuthorization{allowed: true}))
	response, err := client.GetCart(context.Background(), &cartv1.GetCartRequest{UserId: testUserID})
	if err != nil {
		t.Fatalf("GetCart() error = %v", err)
	}
	items := response.GetCart().GetItems()
	if !response.GetCart().GetDegraded() {
		t.Fatal("GetCart() degraded = false, want true")
	}
	if items[1].GetStatus() != cartv1.CartItemStatus_CART_ITEM_STATUS_UNAVAILABLE || items[1].GetProductId() != "p-down" || items[1].GetSkuId() != "p-down-v1" || items[1].GetQuantity() != 2 {
		t.Fatalf("outage line was not preserved as unavailable: %+v", items[1])
	}
	if items[0].GetStatus() != cartv1.CartItemStatus_CART_ITEM_STATUS_AVAILABLE {
		t.Fatalf("healthy line status = %v, want available", items[0].GetStatus())
	}
}

func TestGetCartCatalogNotFoundIsUnavailableWithoutDegraded(t *testing.T) {
	store := &fakeStore{cartID: "cart-1", found: true, lines: []domain.Line{{ID: "item-1", ProductID: "missing", SkuID: "missing-v1", Quantity: 2}}}
	catalog := &fakeCatalog{products: map[string]*catalogv1.GetProductResponse{}, errors: map[string]error{}}
	inventory := &fakeInventory{availability: map[string]int32{}}

	client := newBufconnClient(t, NewServer(store, catalog, inventory, &fakeAuthorization{allowed: true}))
	response, err := client.GetCart(context.Background(), &cartv1.GetCartRequest{UserId: testUserID})
	if err != nil {
		t.Fatalf("GetCart() error = %v", err)
	}
	item := response.GetCart().GetItems()[0]
	if response.GetCart().GetDegraded() || item.GetStatus() != cartv1.CartItemStatus_CART_ITEM_STATUS_UNAVAILABLE {
		t.Fatalf("not-found mapping degraded/status = %t/%v, want false/unavailable", response.GetCart().GetDegraded(), item.GetStatus())
	}
	if len(inventory.batches) != 0 {
		t.Fatal("inventory called for missing catalog product")
	}
}

func TestGetCartDeletedProductPreservesLineAndSkipsInventory(t *testing.T) {
	store := &fakeStore{cartID: "cart-1", found: true, lines: []domain.Line{{
		ID: "item-deleted", ProductID: "p-deleted", SkuID: "p-deleted-v1", Quantity: 7,
	}}}
	deleted := validProduct("p-deleted")
	deleted.Deleted = true
	catalog := &fakeCatalog{products: map[string]*catalogv1.GetProductResponse{"p-deleted": deleted}, errors: map[string]error{}}
	inventory := &fakeInventory{availability: map[string]int32{}}

	client := newBufconnClient(t, NewServer(store, catalog, inventory, &fakeAuthorization{allowed: true}))
	response, err := client.GetCart(context.Background(), &cartv1.GetCartRequest{UserId: testUserID})
	if err != nil {
		t.Fatalf("GetCart() error = %v", err)
	}
	item := response.GetCart().GetItems()[0]
	if item.GetItemId() != "item-deleted" || item.GetProductId() != "p-deleted" || item.GetSkuId() != "p-deleted-v1" || item.GetVariantLabel() != "red" || item.GetQuantity() != 7 {
		t.Fatalf("deleted line identity/quantity = %+v", item)
	}
	if item.GetStatus() != cartv1.CartItemStatus_CART_ITEM_STATUS_UNAVAILABLE {
		t.Fatalf("deleted line status = %v, want unavailable", item.GetStatus())
	}
	if item.GetName() != "Phone" || item.GetUnitPrice().GetAmountMinor() != 12345 || item.GetImageEmoji() != "📱" || item.GetVariantColorHex() != "#ff0000" {
		t.Fatalf("deleted line display enrichment = %+v", item)
	}
	if len(inventory.batches) != 0 {
		t.Fatalf("inventory batches = %v, want none for deleted product", inventory.batches)
	}
}

func TestGetCartInventoryOutagePreservesCatalogEnrichment(t *testing.T) {
	store := &fakeStore{cartID: "cart-1", found: true, lines: []domain.Line{{ID: "item-1", ProductID: "p-1", SkuID: "p-1-v1", Quantity: 2}}}
	catalog := &fakeCatalog{products: map[string]*catalogv1.GetProductResponse{"p-1": validProduct("p-1")}, errors: map[string]error{}}
	inventory := &fakeInventory{
		batchErr: status.Error(codes.Unavailable, "inventory unavailable"),
	}

	client := newBufconnClient(t, NewServer(store, catalog, inventory, &fakeAuthorization{allowed: true}))
	response, err := client.GetCart(context.Background(), &cartv1.GetCartRequest{UserId: testUserID})
	if err != nil {
		t.Fatalf("GetCart() error = %v", err)
	}
	item := response.GetCart().GetItems()[0]
	if !response.GetCart().GetDegraded() || item.GetStatus() != cartv1.CartItemStatus_CART_ITEM_STATUS_UNAVAILABLE {
		t.Fatalf("inventory outage degraded/status = %t/%v, want true/unavailable", response.GetCart().GetDegraded(), item.GetStatus())
	}
	if item.GetName() != "Phone" || item.GetUnitPrice().GetAmountMinor() != 12345 || item.GetVariantColorHex() != "#ff0000" {
		t.Fatalf("catalog enrichment lost during inventory outage: %+v", item)
	}
}

func TestGetCartWithoutExistingCartReturnsEmptyCart(t *testing.T) {
	store := &fakeStore{found: false, lines: nil}
	client := newBufconnClient(t, NewServer(store, &fakeCatalog{}, &fakeInventory{}, &fakeAuthorization{allowed: true}))
	response, err := client.GetCart(context.Background(), &cartv1.GetCartRequest{UserId: testUserID})
	if err != nil {
		t.Fatalf("GetCart() error = %v", err)
	}
	if response.GetCart() == nil || len(response.GetCart().GetItems()) != 0 || response.GetCart().GetDegraded() {
		t.Fatalf("empty cart response = %+v", response.GetCart())
	}
	if store.findUserID != testUserID {
		t.Fatalf("FindCart user id = %q, want %s", store.findUserID, testUserID)
	}
}

func TestAddItemValidationSkipsAuthorization(t *testing.T) {
	store := &fakeStore{cartID: "cart-1"}
	authorization := &fakeAuthorization{allowed: true}
	client := newBufconnClient(t, NewServer(store, &fakeCatalog{}, &fakeInventory{}, authorization))

	_, err := client.AddItem(context.Background(), &cartv1.AddItemRequest{UserId: testUserID, ProductId: "p-1", SkuId: "p-1-v1", Quantity: 0})
	if code := status.Code(err); code != codes.InvalidArgument {
		t.Fatalf("AddItem() code = %v, want InvalidArgument", code)
	}
	if authorization.calls != 0 {
		t.Fatalf("authorization calls = %d, want 0 for invalid input", authorization.calls)
	}
}

func TestAddItemDeniedLeavesCartAndDependenciesUntouched(t *testing.T) {
	store := &fakeStore{cartID: "cart-1"}
	catalog := &fakeCatalog{products: map[string]*catalogv1.GetProductResponse{}, errors: map[string]error{}}
	inventory := &fakeInventory{}
	authorization := &fakeAuthorization{}
	client := newBufconnClient(t, NewServer(store, catalog, inventory, authorization))

	_, err := client.AddItem(context.Background(), &cartv1.AddItemRequest{UserId: testUserID, ProductId: "p-1", SkuId: "p-1-v1", Quantity: 1})
	if code := status.Code(err); code != codes.PermissionDenied {
		t.Fatalf("AddItem() code = %v, want PermissionDenied", code)
	}
	assertErrorInfoReason(t, err, "CART_PERMISSION_DENIED")
	if authorization.calls != 1 || authorization.userID != testUserID || authorization.permission != "cart.item.add" {
		t.Fatalf("authorization = calls=%d user=%q permission=%q, want 1/%s/cart.item.add", authorization.calls, authorization.userID, authorization.permission, testUserID)
	}
	if store.ensureUserID != "" || len(store.addArgs) != 0 || len(catalog.called) != 0 || len(inventory.batches) != 0 {
		t.Fatalf("denied AddItem side effects = ensure=%q add=%v catalog=%v inventory=%v, want none", store.ensureUserID, store.addArgs, catalog.called, inventory.batches)
	}
}

func TestAddItemAuthorizationOutageLeavesCartAndDependenciesUntouched(t *testing.T) {
	store := &fakeStore{cartID: "cart-1"}
	catalog := &fakeCatalog{products: map[string]*catalogv1.GetProductResponse{}, errors: map[string]error{}}
	inventory := &fakeInventory{}
	authorization := &fakeAuthorization{err: status.Error(codes.Unavailable, "identity unavailable")}
	client := newBufconnClient(t, NewServer(store, catalog, inventory, authorization))

	_, err := client.AddItem(context.Background(), &cartv1.AddItemRequest{UserId: testUserID, ProductId: "p-1", SkuId: "p-1-v1", Quantity: 1})
	if code := status.Code(err); code != codes.Unavailable {
		t.Fatalf("AddItem() code = %v, want Unavailable", code)
	}
	assertErrorInfoReason(t, err, "AUTHORIZATION_UNAVAILABLE")
	if store.ensureUserID != "" || len(store.addArgs) != 0 || len(catalog.called) != 0 || len(inventory.batches) != 0 {
		t.Fatalf("unavailable AddItem side effects = ensure=%q add=%v catalog=%v inventory=%v, want none", store.ensureUserID, store.addArgs, catalog.called, inventory.batches)
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

func newTraceRecordingAuthorizationClient(t *testing.T) (*traceRecordingAuthorizationServer, authorization.Client) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen for Identity authorization: %v", err)
	}
	server := &traceRecordingAuthorizationServer{}
	grpcServer := grpc.NewServer()
	identityv1.RegisterAuthorizationServiceServer(grpcServer, server)
	go func() { _ = grpcServer.Serve(listener) }()
	t.Cleanup(func() {
		grpcServer.Stop()
		_ = listener.Close()
	})

	client, err := authorization.NewClient(listener.Addr().String(), time.Second)
	if err != nil {
		t.Fatalf("create Identity authorization client: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return server, client
}

func TestAddItemForwardsIncomingTraceToIdentityAndLogsIt(t *testing.T) {
	previousLogger := slog.Default()
	var logs bytes.Buffer
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previousLogger) })

	identityServer, authorizationClient := newTraceRecordingAuthorizationClient(t)
	client := newBufconnClient(t, NewServer(&fakeStore{cartID: "cart-1"}, &fakeCatalog{}, &fakeInventory{}, authorizationClient))
	ctx := metadata.AppendToOutgoingContext(context.Background(), "x-trace-id", "trace-abc-123")

	_, err := client.AddItem(ctx, &cartv1.AddItemRequest{
		UserId: testUserID, ProductId: "p-1", SkuId: "p-1-v1", Quantity: 1,
	})

	if status.Code(err) != codes.Unavailable {
		t.Fatalf("AddItem() code = %v, want Unavailable", status.Code(err))
	}
	assertErrorInfoReason(t, err, "AUTHORIZATION_UNAVAILABLE")
	if identityServer.traceID != "trace-abc-123" {
		t.Fatalf("Identity metadata trace = %q, want trace-abc-123", identityServer.traceID)
	}
	if !strings.Contains(logs.String(), `"trace_id":"trace-abc-123"`) {
		t.Fatalf("authorization log = %s, want propagated trace_id", logs.String())
	}
}

func TestMutationsValidateInputForwardUserAndReturnMethodResponse(t *testing.T) {
	events := make([]string, 0, 2)
	store := &fakeStore{cartID: "cart-1", found: true, lines: nil, events: &events}
	authorization := &fakeAuthorization{allowed: true, events: &events}
	client := newBufconnClient(t, NewServer(store, &fakeCatalog{products: map[string]*catalogv1.GetProductResponse{}, errors: map[string]error{}}, &fakeInventory{}, authorization))

	if _, err := client.AddItem(context.Background(), &cartv1.AddItemRequest{UserId: " ", ProductId: "p-1", SkuId: "p-1-v1", Quantity: 1}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("blank user AddItem code = %v, want InvalidArgument", status.Code(err))
	}
	if _, err := client.AddItem(context.Background(), &cartv1.AddItemRequest{UserId: testUserID, ProductId: "p-1", SkuId: "p-1-v1", Quantity: 0}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("zero quantity AddItem code = %v, want InvalidArgument", status.Code(err))
	}
	response, err := client.AddItem(context.Background(), &cartv1.AddItemRequest{UserId: testUserID, ProductId: "p-1", SkuId: "p-1-v1", Quantity: 2})
	if err != nil {
		t.Fatalf("AddItem() error = %v", err)
	}
	if store.ensureUserID != testUserID || len(store.addArgs) != 4 || store.addArgs[2] != "p-1-v1" || response.GetCart() == nil {
		t.Fatalf("AddItem forwarding/response invalid: user=%q args=%v response=%+v", store.ensureUserID, store.addArgs, response.GetCart())
	}
	if authorization.calls != 1 || authorization.userID != testUserID || authorization.permission != "cart.item.add" {
		t.Fatalf("AddItem authorization = calls=%d user=%q permission=%q, want 1/%s/cart.item.add", authorization.calls, authorization.userID, authorization.permission, testUserID)
	}
	if len(events) < 2 || events[0] != "check-permission" || events[1] != "ensure-cart" {
		t.Fatalf("AddItem order = %v, want authorization before EnsureCart", events)
	}

	if _, err := client.UpdateItemQuantity(context.Background(), &cartv1.UpdateItemQuantityRequest{UserId: testUserID, ItemId: testItemID, Quantity: 0}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("zero quantity UpdateItemQuantity code = %v, want InvalidArgument", status.Code(err))
	}
	if _, err := client.RemoveItem(context.Background(), &cartv1.RemoveItemRequest{UserId: " ", ItemId: testItemID}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("blank user RemoveItem code = %v, want InvalidArgument", status.Code(err))
	}
	if _, err := client.ClearCart(context.Background(), &cartv1.ClearCartRequest{UserId: testUserID}); err != nil {
		t.Fatalf("ClearCart() error = %v", err)
	}
	if store.clearCartID != "cart-1" {
		t.Fatalf("ClearCart cart id = %q, want cart-1", store.clearCartID)
	}
}

func TestMutationsRejectBlankIDsAndNegativeQuantities(t *testing.T) {
	client := newBufconnClient(t, NewServer(&fakeStore{cartID: "cart-1", found: true}, &fakeCatalog{}, &fakeInventory{}, &fakeAuthorization{allowed: true}))
	tests := []struct {
		name string
		call func() error
	}{
		{"blank product id", func() error {
			_, err := client.AddItem(context.Background(), &cartv1.AddItemRequest{UserId: testUserID, SkuId: "p-1-v1", Quantity: 1})
			return err
		}},
		{"blank sku id", func() error {
			_, err := client.AddItem(context.Background(), &cartv1.AddItemRequest{UserId: testUserID, ProductId: "p-1", Quantity: 1})
			return err
		}},
		{"blank update item id", func() error {
			_, err := client.UpdateItemQuantity(context.Background(), &cartv1.UpdateItemQuantityRequest{UserId: testUserID, Quantity: 1})
			return err
		}},
		{"blank remove item id", func() error {
			_, err := client.RemoveItem(context.Background(), &cartv1.RemoveItemRequest{UserId: testUserID})
			return err
		}},
		{"negative add quantity", func() error {
			_, err := client.AddItem(context.Background(), &cartv1.AddItemRequest{UserId: testUserID, ProductId: "p-1", SkuId: "p-1-v1", Quantity: -1})
			return err
		}},
		{"negative update quantity", func() error {
			_, err := client.UpdateItemQuantity(context.Background(), &cartv1.UpdateItemQuantityRequest{UserId: testUserID, ItemId: testItemID, Quantity: -1})
			return err
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if code := status.Code(test.call()); code != codes.InvalidArgument {
				t.Fatalf("status code = %v, want InvalidArgument", code)
			}
		})
	}
}

func TestCartMethodsRejectInvalidUUIDsBeforeDependencies(t *testing.T) {
	invalidUUIDs := []string{
		"not-a-uuid",
		"123",
		"00000000000000000000000000000001",
		"00000000-0000-0000-0000-00000000000g",
		"{00000000-0000-0000-0000-000000000001}",
	}

	for _, invalid := range invalidUUIDs {
		t.Run(invalid, func(t *testing.T) {
			store := &fakeStore{cartID: "cart-1", found: true}
			auth := &fakeAuthorization{allowed: true}
			client := newBufconnClient(t, NewServer(store, &fakeCatalog{}, &fakeInventory{}, auth))

			calls := []struct {
				name string
				fn   func() error
			}{
				{"GetCart user_id", func() error {
					_, err := client.GetCart(context.Background(), &cartv1.GetCartRequest{UserId: invalid})
					return err
				}},
				{"AddItem user_id", func() error {
					_, err := client.AddItem(context.Background(), &cartv1.AddItemRequest{UserId: invalid, ProductId: "p-1", SkuId: "p-1-v1", Quantity: 1})
					return err
				}},
				{"UpdateItemQuantity user_id", func() error {
					_, err := client.UpdateItemQuantity(context.Background(), &cartv1.UpdateItemQuantityRequest{UserId: invalid, ItemId: testItemID, Quantity: 2})
					return err
				}},
				{"UpdateItemQuantity item_id", func() error {
					_, err := client.UpdateItemQuantity(context.Background(), &cartv1.UpdateItemQuantityRequest{UserId: testUserID, ItemId: invalid, Quantity: 2})
					return err
				}},
				{"RemoveItem user_id", func() error {
					_, err := client.RemoveItem(context.Background(), &cartv1.RemoveItemRequest{UserId: invalid, ItemId: testItemID})
					return err
				}},
				{"RemoveItem item_id", func() error {
					_, err := client.RemoveItem(context.Background(), &cartv1.RemoveItemRequest{UserId: testUserID, ItemId: invalid})
					return err
				}},
				{"ClearCart user_id", func() error {
					_, err := client.ClearCart(context.Background(), &cartv1.ClearCartRequest{UserId: invalid})
					return err
				}},
			}

			for _, call := range calls {
				if code := status.Code(call.fn()); code != codes.InvalidArgument {
					t.Fatalf("%s(%q) code = %v, want InvalidArgument", call.name, invalid, code)
				}
			}
			if auth.calls != 0 || store.findUserID != "" || store.ensureUserID != "" || len(store.updateArgs) != 0 || len(store.removeArgs) != 0 {
				t.Fatalf("invalid UUID triggered side effects: authCalls=%d findUser=%q ensureUser=%q updateArgs=%v removeArgs=%v",
					auth.calls, store.findUserID, store.ensureUserID, store.updateArgs, store.removeArgs)
			}
		})
	}
}

func TestMutationsMapStoreFailuresAndMissingItems(t *testing.T) {
	tests := []struct {
		name      string
		call      func(cartv1.CartServiceClient) error
		makeStore func() *fakeStore
	}{
		{"ensure failure", func(client cartv1.CartServiceClient) error {
			_, err := client.ClearCart(context.Background(), &cartv1.ClearCartRequest{UserId: testUserID})
			return err
		}, func() *fakeStore { return &fakeStore{cartID: "cart-1", ensureErr: errors.New("ensure failed")} }},
		{"add failure", func(client cartv1.CartServiceClient) error {
			_, err := client.AddItem(context.Background(), &cartv1.AddItemRequest{UserId: testUserID, ProductId: "p-1", SkuId: "p-1-v1", Quantity: 1})
			return err
		}, func() *fakeStore { return &fakeStore{cartID: "cart-1", addErr: errors.New("add failed")} }},
		{"update failure", func(client cartv1.CartServiceClient) error {
			_, err := client.UpdateItemQuantity(context.Background(), &cartv1.UpdateItemQuantityRequest{UserId: testUserID, ItemId: testItemID, Quantity: 2})
			return err
		}, func() *fakeStore { return &fakeStore{cartID: "cart-1", updateErr: errors.New("update failed")} }},
		{"remove failure", func(client cartv1.CartServiceClient) error {
			_, err := client.RemoveItem(context.Background(), &cartv1.RemoveItemRequest{UserId: testUserID, ItemId: testItemID})
			return err
		}, func() *fakeStore { return &fakeStore{cartID: "cart-1", removeErr: errors.New("remove failed")} }},
		{"clear failure", func(client cartv1.CartServiceClient) error {
			_, err := client.ClearCart(context.Background(), &cartv1.ClearCartRequest{UserId: testUserID})
			return err
		}, func() *fakeStore { return &fakeStore{cartID: "cart-1", clearErr: errors.New("clear failed")} }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := newBufconnClient(t, NewServer(test.makeStore(), &fakeCatalog{}, &fakeInventory{}, &fakeAuthorization{allowed: true}))
			if code := status.Code(test.call(client)); code != codes.Internal {
				t.Fatalf("status code = %v, want Internal", code)
			}
		})
	}

	for _, method := range []string{"update", "remove"} {
		t.Run("missing "+method, func(t *testing.T) {
			store := &fakeStore{cartID: "cart-1"}
			if method == "update" {
				store.updateErr = cartstore.ErrItemNotFound
			} else {
				store.removeErr = cartstore.ErrItemNotFound
			}
			client := newBufconnClient(t, NewServer(store, &fakeCatalog{}, &fakeInventory{}, &fakeAuthorization{allowed: true}))
			var err error
			if method == "update" {
				_, err = client.UpdateItemQuantity(context.Background(), &cartv1.UpdateItemQuantityRequest{UserId: testUserID, ItemId: testItemID, Quantity: 2})
			} else {
				_, err = client.RemoveItem(context.Background(), &cartv1.RemoveItemRequest{UserId: testUserID, ItemId: testItemID})
			}
			if code := status.Code(err); code != codes.NotFound {
				t.Fatalf("status code = %v, want NotFound", code)
			}
		})
	}
}

func assertErrorInfoReason(t *testing.T, err error, wantReason string) {
	t.Helper()
	for _, detail := range status.Convert(err).Details() {
		info, ok := detail.(*errdetails.ErrorInfo)
		if ok && info.GetReason() == wantReason {
			return
		}
	}
	t.Fatalf("status details = %v, want ErrorInfo reason %q", status.Convert(err).Details(), wantReason)
}

func TestUpdateAndRemoveReturnRefreshedEnrichedCartWrappers(t *testing.T) {
	line := domain.Line{ID: testItemID, ProductID: "p-1", SkuID: "p-1-v1", Quantity: 4}
	newServer := func() (*fakeStore, cartv1.CartServiceClient) {
		store := &fakeStore{cartID: "cart-1", lines: []domain.Line{line}}
		catalog := &fakeCatalog{products: map[string]*catalogv1.GetProductResponse{"p-1": validProduct("p-1")}, errors: map[string]error{}}
		inventory := &fakeInventory{availability: map[string]int32{"p-1-v1": 4}}
		return store, newBufconnClient(t, NewServer(store, catalog, inventory, &fakeAuthorization{allowed: true}))
	}

	_, client := newServer()
	updated, err := client.UpdateItemQuantity(context.Background(), &cartv1.UpdateItemQuantityRequest{UserId: testUserID, ItemId: testItemID, Quantity: 4})
	if err != nil {
		t.Fatalf("UpdateItemQuantity() error = %v", err)
	}
	if updated.GetCart().GetItems()[0].GetName() != "Phone" || updated.GetCart().GetItems()[0].GetStatus() != cartv1.CartItemStatus_CART_ITEM_STATUS_AVAILABLE {
		t.Fatalf("updated wrapper cart = %+v", updated.GetCart())
	}

	_, client = newServer()
	removed, err := client.RemoveItem(context.Background(), &cartv1.RemoveItemRequest{UserId: testUserID, ItemId: testItemID})
	if err != nil {
		t.Fatalf("RemoveItem() error = %v", err)
	}
	if removed.GetCart() == nil || len(removed.GetCart().GetItems()) != 1 || removed.GetCart().GetItems()[0].GetImageUrl() == "" {
		t.Fatalf("removed wrapper cart was not refreshed/enriched: %+v", removed.GetCart())
	}
}

func TestAddItemFailsClosedWhenAuthorizationClientIsNil(t *testing.T) {
	store := &fakeStore{cartID: "cart-1"}
	server := NewServer(store, &fakeCatalog{}, &fakeInventory{}, nil)

	_, err := server.AddItem(context.Background(), &cartv1.AddItemRequest{
		UserId:    testUserID,
		ProductId: "p-1",
		SkuId:     "p-1-v1",
		Quantity:  1,
	})
	if code := status.Code(err); code != codes.Unavailable {
		t.Fatalf("AddItem() status code = %v, want Unavailable", code)
	}
	assertErrorInfoReason(t, err, reasonAuthorizationUnavailable)
	if store.ensureUserID != "" || len(store.addArgs) != 0 {
		t.Fatalf("AddItem() with nil authorization mutated cart state: ensureUserID=%q addArgs=%v", store.ensureUserID, store.addArgs)
	}
}

func TestCartOperationsEnforceGranularPermissionsBeforeStoreAndDependencies(t *testing.T) {
	cases := []struct {
		name           string
		wantPermission string
		wantNextEvent  string
		invoke         func(cartv1.CartServiceClient) error
	}{
		{
			name:           "GetCart",
			wantPermission: "cart.view",
			wantNextEvent:  "find-cart",
			invoke: func(client cartv1.CartServiceClient) error {
				_, err := client.GetCart(context.Background(), &cartv1.GetCartRequest{UserId: testUserID})
				return err
			},
		},
		{
			name:           "UpdateItemQuantity",
			wantPermission: "cart.item.update",
			wantNextEvent:  "ensure-cart",
			invoke: func(client cartv1.CartServiceClient) error {
				_, err := client.UpdateItemQuantity(context.Background(), &cartv1.UpdateItemQuantityRequest{
					UserId: testUserID, ItemId: testItemID, Quantity: 2,
				})
				return err
			},
		},
		{
			name:           "RemoveItem",
			wantPermission: "cart.item.remove",
			wantNextEvent:  "ensure-cart",
			invoke: func(client cartv1.CartServiceClient) error {
				_, err := client.RemoveItem(context.Background(), &cartv1.RemoveItemRequest{
					UserId: testUserID, ItemId: testItemID,
				})
				return err
			},
		},
		{
			name:           "ClearCart",
			wantPermission: "cart.clear",
			wantNextEvent:  "ensure-cart",
			invoke: func(client cartv1.CartServiceClient) error {
				_, err := client.ClearCart(context.Background(), &cartv1.ClearCartRequest{UserId: testUserID})
				return err
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name+" checks permission before store", func(t *testing.T) {
			events := make([]string, 0, 2)
			store := &fakeStore{cartID: "cart-1", found: true, events: &events}
			auth := &fakeAuthorization{allowed: true, events: &events}
			client := newBufconnClient(t, NewServer(store, &fakeCatalog{}, &fakeInventory{}, auth))

			if err := tc.invoke(client); err != nil {
				t.Fatalf("%s() error = %v", tc.name, err)
			}
			if auth.calls != 1 || auth.userID != testUserID || auth.permission != tc.wantPermission {
				t.Fatalf("%s() auth = calls=%d user=%q permission=%q, want 1/%s/%s",
					tc.name, auth.calls, auth.userID, auth.permission, testUserID, tc.wantPermission)
			}
			if len(events) < 2 || events[0] != "check-permission" || events[1] != tc.wantNextEvent {
				t.Fatalf("%s() event order = %v, want [check-permission %s ...]", tc.name, events, tc.wantNextEvent)
			}
		})

		t.Run(tc.name+" denied leaves store and dependencies untouched", func(t *testing.T) {
			store := &fakeStore{
				cartID: "cart-1",
				found:  true,
				lines:  []domain.Line{{ID: testItemID, ProductID: "p-1", SkuID: "p-1-v1", Quantity: 1}},
			}
			catalog := &fakeCatalog{products: map[string]*catalogv1.GetProductResponse{"p-1": validProduct("p-1")}, errors: map[string]error{}}
			inventory := &fakeInventory{availability: map[string]int32{"p-1-v1": 5}}
			auth := &fakeAuthorization{allowed: false}
			client := newBufconnClient(t, NewServer(store, catalog, inventory, auth))

			err := tc.invoke(client)
			if code := status.Code(err); code != codes.PermissionDenied {
				t.Fatalf("%s() code = %v, want PermissionDenied", tc.name, code)
			}
			assertErrorInfoReason(t, err, "CART_PERMISSION_DENIED")
			if auth.calls != 1 || auth.userID != testUserID || auth.permission != tc.wantPermission {
				t.Fatalf("%s() auth = calls=%d user=%q permission=%q, want 1/%s/%s",
					tc.name, auth.calls, auth.userID, auth.permission, testUserID, tc.wantPermission)
			}
			if store.findUserID != "" || store.ensureUserID != "" || len(store.updateArgs) != 0 ||
				len(store.removeArgs) != 0 || store.clearCartID != "" || len(catalog.called) != 0 || len(inventory.batches) != 0 {
				t.Fatalf("%s() denied triggered side effects: find=%q ensure=%q update=%v remove=%v clear=%q catalog=%v inventory=%v",
					tc.name, store.findUserID, store.ensureUserID, store.updateArgs, store.removeArgs, store.clearCartID, catalog.called, inventory.batches)
			}
		})

		t.Run(tc.name+" authorization outage fails closed", func(t *testing.T) {
			store := &fakeStore{cartID: "cart-1", found: true}
			catalog := &fakeCatalog{products: map[string]*catalogv1.GetProductResponse{}, errors: map[string]error{}}
			inventory := &fakeInventory{}
			auth := &fakeAuthorization{err: status.Error(codes.Unavailable, "identity unavailable")}
			client := newBufconnClient(t, NewServer(store, catalog, inventory, auth))

			err := tc.invoke(client)
			if code := status.Code(err); code != codes.Unavailable {
				t.Fatalf("%s() code = %v, want Unavailable", tc.name, code)
			}
			assertErrorInfoReason(t, err, "AUTHORIZATION_UNAVAILABLE")
			if store.findUserID != "" || store.ensureUserID != "" || len(store.updateArgs) != 0 ||
				len(store.removeArgs) != 0 || store.clearCartID != "" || len(catalog.called) != 0 || len(inventory.batches) != 0 {
				t.Fatalf("%s() outage triggered side effects: find=%q ensure=%q update=%v remove=%v clear=%q catalog=%v inventory=%v",
					tc.name, store.findUserID, store.ensureUserID, store.updateArgs, store.removeArgs, store.clearCartID, catalog.called, inventory.batches)
			}
		})

		t.Run(tc.name+" nil authorization client fails closed", func(t *testing.T) {
			store := &fakeStore{cartID: "cart-1", found: true}
			client := newBufconnClient(t, NewServer(store, &fakeCatalog{}, &fakeInventory{}, nil))

			err := tc.invoke(client)
			if code := status.Code(err); code != codes.Unavailable {
				t.Fatalf("%s() nil auth code = %v, want Unavailable", tc.name, code)
			}
			assertErrorInfoReason(t, err, "AUTHORIZATION_UNAVAILABLE")
			if store.findUserID != "" || store.ensureUserID != "" {
				t.Fatalf("%s() nil auth touched store: find=%q ensure=%q", tc.name, store.findUserID, store.ensureUserID)
			}
		})
	}
}

func TestUpdateItemQuantityValidationSkipsAuthorization(t *testing.T) {
	store := &fakeStore{cartID: "cart-1"}
	auth := &fakeAuthorization{allowed: true}
	client := newBufconnClient(t, NewServer(store, &fakeCatalog{}, &fakeInventory{}, auth))

	_, err := client.UpdateItemQuantity(context.Background(), &cartv1.UpdateItemQuantityRequest{
		UserId: testUserID, ItemId: testItemID, Quantity: 0,
	})
	if code := status.Code(err); code != codes.InvalidArgument {
		t.Fatalf("UpdateItemQuantity() code = %v, want InvalidArgument", code)
	}
	if auth.calls != 0 {
		t.Fatalf("authorization calls = %d, want 0 for invalid quantity", auth.calls)
	}
}

func TestGetCartMapsCatalogProductVersion(t *testing.T) {
	store := &fakeStore{
		cartID: "cart-ver",
		found:  true,
		lines: []domain.Line{
			{ID: "item-1", ProductID: "p-ver", SkuID: "p-ver-v1", Quantity: 1},
		},
	}
	prod := validProduct("p-ver")
	prod.Version = 7
	catalog := &fakeCatalog{
		products: map[string]*catalogv1.GetProductResponse{"p-ver": prod},
		errors:   map[string]error{},
	}
	inventory := &fakeInventory{availability: map[string]int32{"p-ver-v1": 5}}

	client := newBufconnClient(t, NewServer(store, catalog, inventory, &fakeAuthorization{allowed: true}))
	resp, err := client.GetCart(context.Background(), &cartv1.GetCartRequest{UserId: testUserID})
	if err != nil {
		t.Fatalf("GetCart() error = %v", err)
	}
	if len(resp.GetCart().GetItems()) != 1 {
		t.Fatalf("got %d items, want 1", len(resp.GetCart().GetItems()))
	}
	if got := resp.GetCart().GetItems()[0].GetProductVersion(); got != 7 {
		t.Fatalf("CartItem.GetProductVersion() = %d, want 7", got)
	}
}
