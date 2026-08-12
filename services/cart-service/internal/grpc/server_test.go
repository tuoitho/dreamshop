package grpc

import (
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"ecommerce.local/cart-service/internal/domain"
	cartstore "ecommerce.local/cart-service/internal/store"
	cartv1 "ecommerce.local/contracts/go/cart/v1"
	catalogv1 "ecommerce.local/contracts/go/catalog/v1"
	commonv1 "ecommerce.local/contracts/go/common/v1"
	inventoryv1 "ecommerce.local/contracts/go/inventory/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

const testBufSize = 1024 * 1024

type fakeStore struct {
	cartID       string
	found        bool
	lines        []domain.Line
	ensureErr    error
	findErr      error
	listErr      error
	addErr       error
	updateErr    error
	removeErr    error
	clearErr     error
	ensureUserID string
	findUserID   string
	addArgs      []any
	updateArgs   []any
	removeArgs   []any
	clearCartID  string
}

func (f *fakeStore) EnsureCart(_ context.Context, userID string) (string, error) {
	f.ensureUserID = userID
	return f.cartID, f.ensureErr
}

func (f *fakeStore) FindCart(_ context.Context, userID string) (string, bool, error) {
	f.findUserID = userID
	return f.cartID, f.found, f.findErr
}

func (f *fakeStore) ListItems(_ context.Context, cartID string) ([]domain.Line, error) {
	if cartID != f.cartID {
		return nil, errors.New("unexpected cart id")
	}
	return f.lines, f.listErr
}

func (f *fakeStore) AddItem(_ context.Context, cartID, productID, variantLabel string, quantity int32) error {
	f.addArgs = []any{cartID, productID, variantLabel, quantity}
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
	availability map[string]*inventoryv1.GetAvailabilityResponse
	errors       map[string]error
	called       []string
}

func (f *fakeInventory) GetAvailability(_ context.Context, productID string) (*inventoryv1.GetAvailabilityResponse, error) {
	f.mu.Lock()
	f.called = append(f.called, productID)
	f.mu.Unlock()
	if err := f.errors[productID]; err != nil {
		return nil, err
	}
	return f.availability[productID], nil
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
		lines[i] = domain.Line{ID: "item-" + string(rune('a'+i)), ProductID: "product-" + string(rune('a'+i)), VariantLabel: "red", Quantity: 1}
	}
	store := &fakeStore{cartID: "cart-1", lines: lines}
	entered := make(chan struct{}, len(lines))
	release := make(chan struct{})
	catalog := &blockingCatalog{entered: entered, release: release}
	inventory := &fakeInventory{availability: map[string]*inventoryv1.GetAvailabilityResponse{}}

	done := make(chan struct{})
	var cart *cartv1.Cart
	var err error
	go func() {
		cart, err = NewServer(store, catalog, inventory).getCart(context.Background(), "cart-1")
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
	if len(cart.GetItems()) != len(lines) {
		t.Fatalf("got %d items, want %d", len(cart.GetItems()), len(lines))
	}
	for i, item := range cart.GetItems() {
		if item.GetItemId() != lines[i].ID || item.GetProductId() != lines[i].ProductID {
			t.Fatalf("item %d = %q/%q, want %q/%q", i, item.GetItemId(), item.GetProductId(), lines[i].ID, lines[i].ProductID)
		}
	}
}

func TestGetCartDeduplicatesCatalogAndInventoryAcrossProductVariants(t *testing.T) {
	product := validProduct("p-1")
	product.Variants = append(product.Variants, &catalogv1.ProductVariant{Label: "blue", ColorHex: "#0000ff"})
	store := &fakeStore{cartID: "cart-1", lines: []domain.Line{
		{ID: "item-1", ProductID: "p-1", VariantLabel: "red", Quantity: 2},
		{ID: "item-2", ProductID: "p-1", VariantLabel: "blue", Quantity: 1},
		{ID: "item-3", ProductID: "p-1", VariantLabel: "red", Quantity: 4},
		{ID: "item-4", ProductID: "p-2", VariantLabel: "red", Quantity: 1},
	}}
	catalog := &fakeCatalog{products: map[string]*catalogv1.GetProductResponse{
		"p-1": product,
		"p-2": validProduct("p-2"),
	}, errors: map[string]error{}}
	inventory := &fakeInventory{availability: map[string]*inventoryv1.GetAvailabilityResponse{
		"p-1": {ProductId: "p-1", AvailableQuantity: 1},
		"p-2": {ProductId: "p-2", AvailableQuantity: 1},
	}, errors: map[string]error{}}

	cart, err := NewServer(store, catalog, inventory).getCart(context.Background(), "cart-1")
	if err != nil {
		t.Fatalf("getCart() error = %v", err)
	}
	if len(catalog.called) != 2 {
		t.Fatalf("catalog calls = %d, want 2 distinct products", len(catalog.called))
	}
	if len(inventory.called) != 2 {
		t.Fatalf("inventory calls = %d, want 2 distinct products", len(inventory.called))
	}
	if cart.GetItems()[0].GetStatus() != cartv1.CartItemStatus_CART_ITEM_STATUS_LIMITED || cart.GetItems()[1].GetStatus() != cartv1.CartItemStatus_CART_ITEM_STATUS_AVAILABLE {
		t.Fatalf("same-product variant statuses = %v, %v", cart.GetItems()[0].GetStatus(), cart.GetItems()[1].GetStatus())
	}
	if cart.GetItems()[2].GetStatus() != cartv1.CartItemStatus_CART_ITEM_STATUS_LIMITED {
		t.Fatalf("repeated product line status = %v, want limited", cart.GetItems()[2].GetStatus())
	}
}

func TestGetCartDeduplicatesCatalogFailureAndAggregatesDegradedStatus(t *testing.T) {
	store := &fakeStore{cartID: "cart-1", lines: []domain.Line{
		{ID: "item-1", ProductID: "down", VariantLabel: "red", Quantity: 1},
		{ID: "item-2", ProductID: "down", VariantLabel: "blue", Quantity: 2},
		{ID: "item-3", ProductID: "ok", VariantLabel: "red", Quantity: 1},
	}}
	catalog := &fakeCatalog{
		products: map[string]*catalogv1.GetProductResponse{"ok": validProduct("ok")},
		errors:   map[string]error{"down": status.Error(codes.Unavailable, "catalog unavailable")},
	}
	inventory := &fakeInventory{availability: map[string]*inventoryv1.GetAvailabilityResponse{
		"ok": {ProductId: "ok", AvailableQuantity: 1},
	}, errors: map[string]error{}}

	cart, err := NewServer(store, catalog, inventory).getCart(context.Background(), "cart-1")
	if err != nil {
		t.Fatalf("getCart() error = %v", err)
	}
	if !cart.GetDegraded() {
		t.Fatal("getCart() degraded = false, want true")
	}
	for i := 0; i < 2; i++ {
		if cart.GetItems()[i].GetStatus() != cartv1.CartItemStatus_CART_ITEM_STATUS_UNAVAILABLE {
			t.Fatalf("failed product line %d status = %v, want unavailable", i, cart.GetItems()[i].GetStatus())
		}
	}
	if len(catalog.called) != 2 {
		t.Fatalf("catalog calls = %d, want one per product", len(catalog.called))
	}
	if len(inventory.called) != 1 || inventory.called[0] != "ok" {
		t.Fatalf("inventory calls = %v, want only healthy product", inventory.called)
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
		Id:           id,
		Name:         "Phone",
		Brand:        "Brand",
		CategoryName: "Phones",
		Price:        &commonv1.Money{Currency: "VND", AmountMinor: 12345},
		Emoji:        "📱",
		ImageHue:     []uint32{10, 20},
		Images:       []string{"https://example.test/phone.png"},
		Variants:     []*catalogv1.ProductVariant{{Label: "red", ColorHex: "#ff0000"}},
	}
}

func TestGetCartEnrichesLinesAndMapsAllStatuses(t *testing.T) {
	store := &fakeStore{
		cartID: "cart-1",
		found:  true,
		lines: []domain.Line{
			{ID: "item-available", ProductID: "p-available", VariantLabel: "red", Quantity: 2},
			{ID: "item-limited", ProductID: "p-limited", VariantLabel: "red", Quantity: 3},
			{ID: "item-empty", ProductID: "p-empty", VariantLabel: "red", Quantity: 1},
			{ID: "item-variant", ProductID: "p-variant", VariantLabel: "blue", Quantity: 4},
		},
	}
	catalog := &fakeCatalog{products: map[string]*catalogv1.GetProductResponse{
		"p-available": validProduct("p-available"),
		"p-limited":   validProduct("p-limited"),
		"p-empty":     validProduct("p-empty"),
		"p-variant":   validProduct("p-variant"),
	}, errors: map[string]error{}}
	inventory := &fakeInventory{
		availability: map[string]*inventoryv1.GetAvailabilityResponse{
			"p-available": {ProductId: "p-available", AvailableQuantity: 5},
			"p-limited":   {ProductId: "p-limited", AvailableQuantity: 1},
			"p-empty":     {ProductId: "p-empty", AvailableQuantity: 0},
		},
		errors: map[string]error{},
	}

	client := newBufconnClient(t, NewServer(store, catalog, inventory))
	response, err := client.GetCart(context.Background(), &cartv1.GetCartRequest{UserId: "user-1"})
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
	if limited.GetName() != "Phone" || limited.GetBrand() != "Brand" || limited.GetCategoryName() != "Phones" || limited.GetUnitPrice().GetAmountMinor() != 12345 {
		t.Fatalf("valid product enrichment missing: %+v", limited)
	}
	if limited.GetImageUrl() != "https://example.test/phone.png" || limited.GetVariantColorHex() != "#ff0000" {
		t.Fatalf("image/variant enrichment missing: %+v", limited)
	}
	if len(inventory.called) != 3 {
		t.Fatalf("inventory called for %d products, want 3", len(inventory.called))
	}
}

func TestGetCartCatalogOutageDegradesOnlyAffectedLine(t *testing.T) {
	store := &fakeStore{cartID: "cart-1", found: true, lines: []domain.Line{
		{ID: "item-ok", ProductID: "p-ok", VariantLabel: "red", Quantity: 1},
		{ID: "item-down", ProductID: "p-down", VariantLabel: "red", Quantity: 2},
	}}
	catalog := &fakeCatalog{
		products: map[string]*catalogv1.GetProductResponse{"p-ok": validProduct("p-ok")},
		errors:   map[string]error{"p-down": status.Error(codes.Unavailable, "catalog unavailable")},
	}
	inventory := &fakeInventory{availability: map[string]*inventoryv1.GetAvailabilityResponse{
		"p-ok": {ProductId: "p-ok", AvailableQuantity: 1},
	}, errors: map[string]error{}}

	client := newBufconnClient(t, NewServer(store, catalog, inventory))
	response, err := client.GetCart(context.Background(), &cartv1.GetCartRequest{UserId: "user-1"})
	if err != nil {
		t.Fatalf("GetCart() error = %v", err)
	}
	items := response.GetCart().GetItems()
	if !response.GetCart().GetDegraded() {
		t.Fatal("GetCart() degraded = false, want true")
	}
	if items[1].GetStatus() != cartv1.CartItemStatus_CART_ITEM_STATUS_UNAVAILABLE || items[1].GetProductId() != "p-down" || items[1].GetQuantity() != 2 {
		t.Fatalf("outage line was not preserved as unavailable: %+v", items[1])
	}
	if items[0].GetStatus() != cartv1.CartItemStatus_CART_ITEM_STATUS_AVAILABLE {
		t.Fatalf("healthy line status = %v, want available", items[0].GetStatus())
	}
}

func TestGetCartCatalogNotFoundIsUnavailableWithoutDegraded(t *testing.T) {
	store := &fakeStore{cartID: "cart-1", found: true, lines: []domain.Line{{ID: "item-1", ProductID: "missing", VariantLabel: "red", Quantity: 2}}}
	catalog := &fakeCatalog{products: map[string]*catalogv1.GetProductResponse{}, errors: map[string]error{}}
	inventory := &fakeInventory{availability: map[string]*inventoryv1.GetAvailabilityResponse{}, errors: map[string]error{}}

	client := newBufconnClient(t, NewServer(store, catalog, inventory))
	response, err := client.GetCart(context.Background(), &cartv1.GetCartRequest{UserId: "user-1"})
	if err != nil {
		t.Fatalf("GetCart() error = %v", err)
	}
	item := response.GetCart().GetItems()[0]
	if response.GetCart().GetDegraded() || item.GetStatus() != cartv1.CartItemStatus_CART_ITEM_STATUS_UNAVAILABLE {
		t.Fatalf("not-found mapping degraded/status = %t/%v, want false/unavailable", response.GetCart().GetDegraded(), item.GetStatus())
	}
	if len(inventory.called) != 0 {
		t.Fatal("inventory called for missing catalog product")
	}
}

func TestGetCartDeletedProductPreservesLineAndSkipsInventory(t *testing.T) {
	store := &fakeStore{cartID: "cart-1", found: true, lines: []domain.Line{{
		ID: "item-deleted", ProductID: "p-deleted", VariantLabel: "red", Quantity: 7,
	}}}
	deleted := validProduct("p-deleted")
	deleted.Deleted = true
	catalog := &fakeCatalog{products: map[string]*catalogv1.GetProductResponse{"p-deleted": deleted}, errors: map[string]error{}}
	inventory := &fakeInventory{availability: map[string]*inventoryv1.GetAvailabilityResponse{}, errors: map[string]error{}}

	client := newBufconnClient(t, NewServer(store, catalog, inventory))
	response, err := client.GetCart(context.Background(), &cartv1.GetCartRequest{UserId: "user-1"})
	if err != nil {
		t.Fatalf("GetCart() error = %v", err)
	}
	item := response.GetCart().GetItems()[0]
	if item.GetItemId() != "item-deleted" || item.GetProductId() != "p-deleted" || item.GetVariantLabel() != "red" || item.GetQuantity() != 7 {
		t.Fatalf("deleted line identity/quantity = %+v", item)
	}
	if item.GetStatus() != cartv1.CartItemStatus_CART_ITEM_STATUS_UNAVAILABLE {
		t.Fatalf("deleted line status = %v, want unavailable", item.GetStatus())
	}
	if len(inventory.called) != 0 {
		t.Fatalf("inventory calls = %v, want none for deleted product", inventory.called)
	}
}

func TestGetCartInventoryOutagePreservesCatalogEnrichment(t *testing.T) {
	store := &fakeStore{cartID: "cart-1", found: true, lines: []domain.Line{{ID: "item-1", ProductID: "p-1", VariantLabel: "red", Quantity: 2}}}
	catalog := &fakeCatalog{products: map[string]*catalogv1.GetProductResponse{"p-1": validProduct("p-1")}, errors: map[string]error{}}
	inventory := &fakeInventory{
		availability: map[string]*inventoryv1.GetAvailabilityResponse{},
		errors:       map[string]error{"p-1": status.Error(codes.Unavailable, "inventory unavailable")},
	}

	client := newBufconnClient(t, NewServer(store, catalog, inventory))
	response, err := client.GetCart(context.Background(), &cartv1.GetCartRequest{UserId: "user-1"})
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
	client := newBufconnClient(t, NewServer(store, &fakeCatalog{}, &fakeInventory{}))
	response, err := client.GetCart(context.Background(), &cartv1.GetCartRequest{UserId: "user-1"})
	if err != nil {
		t.Fatalf("GetCart() error = %v", err)
	}
	if response.GetCart() == nil || len(response.GetCart().GetItems()) != 0 || response.GetCart().GetDegraded() {
		t.Fatalf("empty cart response = %+v", response.GetCart())
	}
	if store.findUserID != "user-1" {
		t.Fatalf("FindCart user id = %q, want user-1", store.findUserID)
	}
}

func TestMutationsValidateInputForwardUserAndReturnMethodResponse(t *testing.T) {
	store := &fakeStore{cartID: "cart-1", found: true, lines: nil}
	client := newBufconnClient(t, NewServer(store, &fakeCatalog{products: map[string]*catalogv1.GetProductResponse{}, errors: map[string]error{}}, &fakeInventory{availability: map[string]*inventoryv1.GetAvailabilityResponse{}, errors: map[string]error{}}))

	if _, err := client.AddItem(context.Background(), &cartv1.AddItemRequest{UserId: " ", ProductId: "p-1", VariantLabel: "red", Quantity: 1}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("blank user AddItem code = %v, want InvalidArgument", status.Code(err))
	}
	if _, err := client.AddItem(context.Background(), &cartv1.AddItemRequest{UserId: "user-1", ProductId: "p-1", VariantLabel: "red", Quantity: 0}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("zero quantity AddItem code = %v, want InvalidArgument", status.Code(err))
	}
	response, err := client.AddItem(context.Background(), &cartv1.AddItemRequest{UserId: "user-1", ProductId: "p-1", VariantLabel: "red", Quantity: 2})
	if err != nil {
		t.Fatalf("AddItem() error = %v", err)
	}
	if store.ensureUserID != "user-1" || len(store.addArgs) != 4 || response.GetCart() == nil {
		t.Fatalf("AddItem forwarding/response invalid: user=%q args=%v response=%+v", store.ensureUserID, store.addArgs, response.GetCart())
	}

	if _, err := client.UpdateItemQuantity(context.Background(), &cartv1.UpdateItemQuantityRequest{UserId: "user-1", ItemId: "item-1", Quantity: 0}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("zero quantity UpdateItemQuantity code = %v, want InvalidArgument", status.Code(err))
	}
	if _, err := client.RemoveItem(context.Background(), &cartv1.RemoveItemRequest{UserId: " ", ItemId: "item-1"}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("blank user RemoveItem code = %v, want InvalidArgument", status.Code(err))
	}
	if _, err := client.ClearCart(context.Background(), &cartv1.ClearCartRequest{UserId: "user-1"}); err != nil {
		t.Fatalf("ClearCart() error = %v", err)
	}
	if store.clearCartID != "cart-1" {
		t.Fatalf("ClearCart cart id = %q, want cart-1", store.clearCartID)
	}
}

func TestMutationsRejectBlankIDsAndNegativeQuantities(t *testing.T) {
	client := newBufconnClient(t, NewServer(&fakeStore{cartID: "cart-1", found: true}, &fakeCatalog{}, &fakeInventory{}))
	tests := []struct {
		name string
		call func() error
	}{
		{"blank product id", func() error {
			_, err := client.AddItem(context.Background(), &cartv1.AddItemRequest{UserId: "user-1", VariantLabel: "red", Quantity: 1})
			return err
		}},
		{"blank variant label", func() error {
			_, err := client.AddItem(context.Background(), &cartv1.AddItemRequest{UserId: "user-1", ProductId: "p-1", Quantity: 1})
			return err
		}},
		{"blank update item id", func() error {
			_, err := client.UpdateItemQuantity(context.Background(), &cartv1.UpdateItemQuantityRequest{UserId: "user-1", Quantity: 1})
			return err
		}},
		{"blank remove item id", func() error {
			_, err := client.RemoveItem(context.Background(), &cartv1.RemoveItemRequest{UserId: "user-1"})
			return err
		}},
		{"negative add quantity", func() error {
			_, err := client.AddItem(context.Background(), &cartv1.AddItemRequest{UserId: "user-1", ProductId: "p-1", VariantLabel: "red", Quantity: -1})
			return err
		}},
		{"negative update quantity", func() error {
			_, err := client.UpdateItemQuantity(context.Background(), &cartv1.UpdateItemQuantityRequest{UserId: "user-1", ItemId: "item-1", Quantity: -1})
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

func TestMutationsMapStoreFailuresAndMissingItems(t *testing.T) {
	tests := []struct {
		name      string
		call      func(cartv1.CartServiceClient) error
		makeStore func() *fakeStore
	}{
		{"ensure failure", func(client cartv1.CartServiceClient) error {
			_, err := client.ClearCart(context.Background(), &cartv1.ClearCartRequest{UserId: "user-1"})
			return err
		}, func() *fakeStore { return &fakeStore{cartID: "cart-1", ensureErr: errors.New("ensure failed")} }},
		{"add failure", func(client cartv1.CartServiceClient) error {
			_, err := client.AddItem(context.Background(), &cartv1.AddItemRequest{UserId: "user-1", ProductId: "p-1", VariantLabel: "red", Quantity: 1})
			return err
		}, func() *fakeStore { return &fakeStore{cartID: "cart-1", addErr: errors.New("add failed")} }},
		{"update failure", func(client cartv1.CartServiceClient) error {
			_, err := client.UpdateItemQuantity(context.Background(), &cartv1.UpdateItemQuantityRequest{UserId: "user-1", ItemId: "item-1", Quantity: 2})
			return err
		}, func() *fakeStore { return &fakeStore{cartID: "cart-1", updateErr: errors.New("update failed")} }},
		{"remove failure", func(client cartv1.CartServiceClient) error {
			_, err := client.RemoveItem(context.Background(), &cartv1.RemoveItemRequest{UserId: "user-1", ItemId: "item-1"})
			return err
		}, func() *fakeStore { return &fakeStore{cartID: "cart-1", removeErr: errors.New("remove failed")} }},
		{"clear failure", func(client cartv1.CartServiceClient) error {
			_, err := client.ClearCart(context.Background(), &cartv1.ClearCartRequest{UserId: "user-1"})
			return err
		}, func() *fakeStore { return &fakeStore{cartID: "cart-1", clearErr: errors.New("clear failed")} }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := newBufconnClient(t, NewServer(test.makeStore(), &fakeCatalog{}, &fakeInventory{}))
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
			client := newBufconnClient(t, NewServer(store, &fakeCatalog{}, &fakeInventory{}))
			var err error
			if method == "update" {
				_, err = client.UpdateItemQuantity(context.Background(), &cartv1.UpdateItemQuantityRequest{UserId: "user-1", ItemId: "item-1", Quantity: 2})
			} else {
				_, err = client.RemoveItem(context.Background(), &cartv1.RemoveItemRequest{UserId: "user-1", ItemId: "item-1"})
			}
			if code := status.Code(err); code != codes.NotFound {
				t.Fatalf("status code = %v, want NotFound", code)
			}
		})
	}
}

func TestUpdateAndRemoveReturnRefreshedEnrichedCartWrappers(t *testing.T) {
	line := domain.Line{ID: "item-1", ProductID: "p-1", VariantLabel: "red", Quantity: 4}
	newServer := func() (*fakeStore, cartv1.CartServiceClient) {
		store := &fakeStore{cartID: "cart-1", lines: []domain.Line{line}}
		catalog := &fakeCatalog{products: map[string]*catalogv1.GetProductResponse{"p-1": validProduct("p-1")}, errors: map[string]error{}}
		inventory := &fakeInventory{availability: map[string]*inventoryv1.GetAvailabilityResponse{"p-1": {ProductId: "p-1", AvailableQuantity: 4}}, errors: map[string]error{}}
		return store, newBufconnClient(t, NewServer(store, catalog, inventory))
	}

	_, client := newServer()
	updated, err := client.UpdateItemQuantity(context.Background(), &cartv1.UpdateItemQuantityRequest{UserId: "user-1", ItemId: "item-1", Quantity: 4})
	if err != nil {
		t.Fatalf("UpdateItemQuantity() error = %v", err)
	}
	if updated.GetCart().GetItems()[0].GetName() != "Phone" || updated.GetCart().GetItems()[0].GetStatus() != cartv1.CartItemStatus_CART_ITEM_STATUS_AVAILABLE {
		t.Fatalf("updated wrapper cart = %+v", updated.GetCart())
	}

	_, client = newServer()
	removed, err := client.RemoveItem(context.Background(), &cartv1.RemoveItemRequest{UserId: "user-1", ItemId: "item-1"})
	if err != nil {
		t.Fatalf("RemoveItem() error = %v", err)
	}
	if removed.GetCart() == nil || len(removed.GetCart().GetItems()) != 1 || removed.GetCart().GetItems()[0].GetImageUrl() == "" {
		t.Fatalf("removed wrapper cart was not refreshed/enriched: %+v", removed.GetCart())
	}
}
