package grpc

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"ecommerce.local/cart-service/internal/domain"
	cartv1 "ecommerce.local/contracts/go/cart/v1"
	catalogv1 "ecommerce.local/contracts/go/catalog/v1"
	commonv1 "ecommerce.local/contracts/go/common/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func detailedFavoriteProduct(id string) *catalogv1.GetProductResponse {
	return &catalogv1.GetProductResponse{
		Id:            id,
		Name:          "iPhone 16 Pro",
		Brand:         "Apple",
		CategoryId:    "dien-thoai",
		CategoryName:  "Điện thoại",
		Price:         &commonv1.Money{Currency: "VND", AmountMinor: 26990000},
		OriginalPrice: &commonv1.Money{Currency: "VND", AmountMinor: 29990000},
		MaxPrice:      &commonv1.Money{Currency: "VND", AmountMinor: 31990000},
		Rating:        4.9,
		ReviewCount:   128,
		SoldCount:     540,
		Emoji:         "📱",
		ImageHue:      []uint32{210, 250},
		Images:        []string{"https://example.test/iphone.png"},
		Badge:         "hot",
		Variants: []*catalogv1.ProductVariant{
			{
				SkuId:         id + "-v1",
				Label:         "Đen Titan 256GB",
				ColorHex:      "#1c1c1e",
				Price:         &commonv1.Money{Currency: "VND", AmountMinor: 26990000},
				OriginalPrice: &commonv1.Money{Currency: "VND", AmountMinor: 29990000},
			},
			{
				SkuId:         id + "-v2",
				Label:         "Trắng Titan 512GB",
				ColorHex:      "#f5f5f7",
				Price:         &commonv1.Money{Currency: "VND", AmountMinor: 31990000},
				OriginalPrice: &commonv1.Money{Currency: "VND", AmountMinor: 34990000},
			},
		},
	}
}

func TestFavoriteMethodsValidateInputBeforeAuthorization(t *testing.T) {
	overlongProductID := strings.Repeat("p", 129)
	store := &fakeStore{}
	auth := &fakeAuthorization{allowed: true}
	client := newBufconnClient(t, NewServer(store, &fakeCatalog{}, &fakeInventory{}, auth))

	cases := []struct {
		name string
		call func() error
	}{
		{"GetFavorites invalid user_id", func() error {
			_, err := client.GetFavorites(context.Background(), &cartv1.GetFavoritesRequest{UserId: "invalid-uuid"})
			return err
		}},
		{"AddFavorite invalid user_id", func() error {
			_, err := client.AddFavorite(context.Background(), &cartv1.AddFavoriteRequest{UserId: " ", ProductId: "dt-1"})
			return err
		}},
		{"AddFavorite blank product_id", func() error {
			_, err := client.AddFavorite(context.Background(), &cartv1.AddFavoriteRequest{UserId: testUserID, ProductId: "   "})
			return err
		}},
		{"AddFavorite overlong product_id", func() error {
			_, err := client.AddFavorite(context.Background(), &cartv1.AddFavoriteRequest{UserId: testUserID, ProductId: overlongProductID})
			return err
		}},
		{"RemoveFavorite invalid user_id", func() error {
			_, err := client.RemoveFavorite(context.Background(), &cartv1.RemoveFavoriteRequest{UserId: "not-a-uuid", ProductId: "dt-1"})
			return err
		}},
		{"RemoveFavorite blank product_id", func() error {
			_, err := client.RemoveFavorite(context.Background(), &cartv1.RemoveFavoriteRequest{UserId: testUserID, ProductId: "\t\n"})
			return err
		}},
		{"RemoveFavorite overlong product_id", func() error {
			_, err := client.RemoveFavorite(context.Background(), &cartv1.RemoveFavoriteRequest{UserId: testUserID, ProductId: overlongProductID})
			return err
		}},
		{"ClearFavorites invalid user_id", func() error {
			_, err := client.ClearFavorites(context.Background(), &cartv1.ClearFavoritesRequest{UserId: "123"})
			return err
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if code := status.Code(tc.call()); code != codes.InvalidArgument {
				t.Fatalf("code = %v, want InvalidArgument", code)
			}
		})
	}
	if auth.calls != 0 {
		t.Fatalf("authorization calls = %d, want 0 when input validation fails", auth.calls)
	}
}

func TestFavoriteMethodsEnforceFavoriteManageAuthorization(t *testing.T) {
	rpcs := []struct {
		name string
		call func(cartv1.CartServiceClient) error
	}{
		{"GetFavorites", func(c cartv1.CartServiceClient) error {
			_, err := c.GetFavorites(context.Background(), &cartv1.GetFavoritesRequest{UserId: testUserID})
			return err
		}},
		{"AddFavorite", func(c cartv1.CartServiceClient) error {
			_, err := c.AddFavorite(context.Background(), &cartv1.AddFavoriteRequest{UserId: testUserID, ProductId: "dt-1"})
			return err
		}},
		{"RemoveFavorite", func(c cartv1.CartServiceClient) error {
			_, err := c.RemoveFavorite(context.Background(), &cartv1.RemoveFavoriteRequest{UserId: testUserID, ProductId: "dt-1"})
			return err
		}},
		{"ClearFavorites", func(c cartv1.CartServiceClient) error {
			_, err := c.ClearFavorites(context.Background(), &cartv1.ClearFavoritesRequest{UserId: testUserID})
			return err
		}},
	}

	for _, rpc := range rpcs {
		t.Run(rpc.name+" denied", func(t *testing.T) {
			store := &fakeStore{}
			catalog := &fakeCatalog{products: map[string]*catalogv1.GetProductResponse{"dt-1": detailedFavoriteProduct("dt-1")}, errors: map[string]error{}}
			inventory := &fakeInventory{}
			auth := &fakeAuthorization{allowed: false}
			client := newBufconnClient(t, NewServer(store, catalog, inventory, auth))

			err := rpc.call(client)
			if code := status.Code(err); code != codes.PermissionDenied {
				t.Fatalf("code = %v, want PermissionDenied", code)
			}
			assertErrorInfoReason(t, err, "FAVORITE_PERMISSION_DENIED")
			if auth.calls != 1 || auth.userID != testUserID || auth.permission != "favorite.manage" {
				t.Fatalf("auth = calls=%d user=%q perm=%q, want 1/%s/favorite.manage", auth.calls, auth.userID, auth.permission, testUserID)
			}
			if len(catalog.called) != 0 || len(inventory.batches) != 0 || len(store.addFavoriteArgs) != 0 || len(store.removeFavArgs) != 0 || store.clearFavUserID != "" {
				t.Fatalf("denied RPC caused side effects: catalog=%v inventory=%v add=%v remove=%v clear=%q",
					catalog.called, inventory.batches, store.addFavoriteArgs, store.removeFavArgs, store.clearFavUserID)
			}
		})

		t.Run(rpc.name+" authorization unavailable", func(t *testing.T) {
			auth := &fakeAuthorization{err: status.Error(codes.Unavailable, "identity down")}
			client := newBufconnClient(t, NewServer(&fakeStore{}, &fakeCatalog{}, &fakeInventory{}, auth))

			err := rpc.call(client)
			if code := status.Code(err); code != codes.Unavailable {
				t.Fatalf("code = %v, want Unavailable", code)
			}
			assertErrorInfoReason(t, err, "AUTHORIZATION_UNAVAILABLE")
		})

		t.Run(rpc.name+" nil authorization client fails closed", func(t *testing.T) {
			client := newBufconnClient(t, NewServer(&fakeStore{}, &fakeCatalog{}, &fakeInventory{}, nil))

			err := rpc.call(client)
			if code := status.Code(err); code != codes.Unavailable {
				t.Fatalf("code = %v, want Unavailable", code)
			}
			assertErrorInfoReason(t, err, "AUTHORIZATION_UNAVAILABLE")
		})
	}
}

type cancelingAuthorization struct{}

func (cancelingAuthorization) CheckPermission(ctx context.Context, _, _ string) (bool, error) {
	<-ctx.Done()
	return false, ctx.Err()
}

func TestGetFavoritesPropagatesValidatedTraceAndPreservesCallerCancellation(t *testing.T) {
	identityServer, authorizationClient := newTraceRecordingAuthorizationClient(t)
	client := newBufconnClient(t, NewServer(&fakeStore{}, &fakeCatalog{}, &fakeInventory{}, authorizationClient))

	ctx := metadata.AppendToOutgoingContext(context.Background(), "x-trace-id", "fav-trace-001")
	_, err := client.GetFavorites(ctx, &cartv1.GetFavoritesRequest{UserId: testUserID})
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("GetFavorites() code = %v, want Unavailable", status.Code(err))
	}
	assertErrorInfoReason(t, err, "AUTHORIZATION_UNAVAILABLE")
	if identityServer.traceID != "fav-trace-001" {
		t.Fatalf("propagated traceID = %q, want fav-trace-001", identityServer.traceID)
	}

	identityServer.traceID = ""
	unsafeCtx := metadata.AppendToOutgoingContext(context.Background(), "x-trace-id", "bad trace with spaces!")
	_, _ = client.GetFavorites(unsafeCtx, &cartv1.GetFavoritesRequest{UserId: testUserID})
	if identityServer.traceID != "" {
		t.Fatalf("unsafe traceID propagated = %q, want empty", identityServer.traceID)
	}

	server := NewServer(&fakeStore{}, &fakeCatalog{}, &fakeInventory{}, cancelingAuthorization{})
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = server.GetFavorites(canceledCtx, &cartv1.GetFavoritesRequest{UserId: testUserID})
	if status.Code(err) != codes.Canceled {
		t.Fatalf("canceled caller GetFavorites() code = %v, want Canceled", status.Code(err))
	}
}

func TestAddFavoriteVerifiesCatalogAndAllowsOutOfStockWhileBlockingDiscontinued(t *testing.T) {
	t.Run("not found product returns NotFound without persisting", func(t *testing.T) {
		store := &fakeStore{}
		catalog := &fakeCatalog{products: map[string]*catalogv1.GetProductResponse{}, errors: map[string]error{}}
		client := newBufconnClient(t, NewServer(store, catalog, &fakeInventory{}, &fakeAuthorization{allowed: true}))

		_, err := client.AddFavorite(context.Background(), &cartv1.AddFavoriteRequest{UserId: testUserID, ProductId: "missing-id"})
		if status.Code(err) != codes.NotFound {
			t.Fatalf("AddFavorite() code = %v, want NotFound", status.Code(err))
		}
		if len(store.addFavoriteArgs) != 0 {
			t.Fatalf("AddFavorite persisted missing product: %v", store.addFavoriteArgs)
		}
	})

	t.Run("catalog outage returns Unavailable without persisting", func(t *testing.T) {
		store := &fakeStore{}
		catalog := &fakeCatalog{
			products: map[string]*catalogv1.GetProductResponse{},
			errors:   map[string]error{"dt-down": status.Error(codes.Unavailable, "catalog down")},
		}
		client := newBufconnClient(t, NewServer(store, catalog, &fakeInventory{}, &fakeAuthorization{allowed: true}))

		_, err := client.AddFavorite(context.Background(), &cartv1.AddFavoriteRequest{UserId: testUserID, ProductId: "dt-down"})
		if status.Code(err) != codes.Unavailable {
			t.Fatalf("AddFavorite() code = %v, want Unavailable", status.Code(err))
		}
		if len(store.addFavoriteArgs) != 0 {
			t.Fatalf("AddFavorite persisted when catalog was unavailable: %v", store.addFavoriteArgs)
		}
	})

	t.Run("soft deleted product returns FailedPrecondition FAVORITE_PRODUCT_UNAVAILABLE", func(t *testing.T) {
		store := &fakeStore{}
		deleted := detailedFavoriteProduct("dt-deleted")
		deleted.Deleted = true
		catalog := &fakeCatalog{products: map[string]*catalogv1.GetProductResponse{"dt-deleted": deleted}, errors: map[string]error{}}
		client := newBufconnClient(t, NewServer(store, catalog, &fakeInventory{}, &fakeAuthorization{allowed: true}))

		_, err := client.AddFavorite(context.Background(), &cartv1.AddFavoriteRequest{UserId: testUserID, ProductId: "dt-deleted"})
		if status.Code(err) != codes.FailedPrecondition {
			t.Fatalf("AddFavorite() code = %v, want FailedPrecondition", status.Code(err))
		}
		assertErrorInfoReason(t, err, "FAVORITE_PRODUCT_UNAVAILABLE")
		if len(store.addFavoriteArgs) != 0 {
			t.Fatalf("AddFavorite persisted soft-deleted product: %v", store.addFavoriteArgs)
		}
	})

	t.Run("variantless product returns FailedPrecondition FAVORITE_PRODUCT_UNAVAILABLE", func(t *testing.T) {
		store := &fakeStore{}
		noVariants := detailedFavoriteProduct("dt-novar")
		noVariants.Variants = nil
		catalog := &fakeCatalog{products: map[string]*catalogv1.GetProductResponse{"dt-novar": noVariants}, errors: map[string]error{}}
		client := newBufconnClient(t, NewServer(store, catalog, &fakeInventory{}, &fakeAuthorization{allowed: true}))

		_, err := client.AddFavorite(context.Background(), &cartv1.AddFavoriteRequest{UserId: testUserID, ProductId: "dt-novar"})
		if status.Code(err) != codes.FailedPrecondition {
			t.Fatalf("AddFavorite() code = %v, want FailedPrecondition", status.Code(err))
		}
		assertErrorInfoReason(t, err, "FAVORITE_PRODUCT_UNAVAILABLE")
		if len(store.addFavoriteArgs) != 0 {
			t.Fatalf("AddFavorite persisted variantless product: %v", store.addFavoriteArgs)
		}
	})

	t.Run("out of stock active product is allowed and returned as OUT_OF_STOCK", func(t *testing.T) {
		store := &fakeStore{}
		catalog := &fakeCatalog{products: map[string]*catalogv1.GetProductResponse{"dt-oos": detailedFavoriteProduct("dt-oos")}, errors: map[string]error{}}
		inventory := &fakeInventory{
			availability: map[string]int32{"dt-oos-v1": 0, "dt-oos-v2": 0},
		}
		client := newBufconnClient(t, NewServer(store, catalog, inventory, &fakeAuthorization{allowed: true}))

		resp, err := client.AddFavorite(context.Background(), &cartv1.AddFavoriteRequest{UserId: testUserID, ProductId: "  dt-oos  "})
		if err != nil {
			t.Fatalf("AddFavorite() error = %v", err)
		}
		if len(store.addFavoriteArgs) != 2 || store.addFavoriteArgs[0] != testUserID || store.addFavoriteArgs[1] != "dt-oos" {
			t.Fatalf("store.AddFavorite args = %v, want [%s dt-oos]", store.addFavoriteArgs, testUserID)
		}
		items := resp.GetFavorites().GetItems()
		if len(items) != 1 || items[0].GetStatus() != cartv1.FavoriteItemStatus_FAVORITE_ITEM_STATUS_OUT_OF_STOCK {
			t.Fatalf("AddFavorite OOS items = %+v", items)
		}
	})
}

func TestGetFavoritesEmptyListShortCircuitsDownstreamCalls(t *testing.T) {
	store := &fakeStore{favorites: nil}
	catalog := &fakeCatalog{products: map[string]*catalogv1.GetProductResponse{}, errors: map[string]error{}}
	inventory := &fakeInventory{}
	client := newBufconnClient(t, NewServer(store, catalog, inventory, &fakeAuthorization{allowed: true}))

	resp, err := client.GetFavorites(context.Background(), &cartv1.GetFavoritesRequest{UserId: testUserID})
	if err != nil {
		t.Fatalf("GetFavorites() error = %v", err)
	}
	if resp.GetFavorites() == nil || len(resp.GetFavorites().GetItems()) != 0 || resp.GetFavorites().GetDegraded() {
		t.Fatalf("empty GetFavorites() = %+v", resp.GetFavorites())
	}
	if len(catalog.called) != 0 || len(inventory.batches) != 0 {
		t.Fatalf("downstream calls on empty favorites: catalog=%v inventory=%v", catalog.called, inventory.batches)
	}
}

func TestGetFavoritesEnrichesMetadataStatusesAndGracefulDegradation(t *testing.T) {
	deletedProd := detailedFavoriteProduct("p-deleted")
	deletedProd.Deleted = true

	store := &fakeStore{
		favorites: []domain.FavoriteRecord{
			{ID: "fav-avail", ProductID: "p-avail"},
			{ID: "fav-limited", ProductID: "p-limited"},
			{ID: "fav-oos", ProductID: "p-oos"},
			{ID: "fav-deleted", ProductID: "p-deleted"},
			{ID: "fav-orphan", ProductID: "p-orphan"},
		},
	}
	catalog := &fakeCatalog{
		products: map[string]*catalogv1.GetProductResponse{
			"p-avail":   detailedFavoriteProduct("p-avail"),
			"p-limited": detailedFavoriteProduct("p-limited"),
			"p-oos":     detailedFavoriteProduct("p-oos"),
			"p-deleted": deletedProd,
		},
		errors: map[string]error{},
	}
	inventory := &fakeInventory{
		availability: map[string]int32{
			"p-avail-v1":   2,
			"p-avail-v2":   4,
			"p-limited-v1": 5,
			"p-limited-v2": 0,
			"p-oos-v1":     0,
			"p-oos-v2":     0,
		},
	}

	client := newBufconnClient(t, NewServer(store, catalog, inventory, &fakeAuthorization{allowed: true}))
	resp, err := client.GetFavorites(context.Background(), &cartv1.GetFavoritesRequest{UserId: testUserID})
	if err != nil {
		t.Fatalf("GetFavorites() error = %v", err)
	}
	if resp.GetFavorites().GetDegraded() {
		t.Fatal("GetFavorites() degraded = true, want false when only soft/hard deleted products exist")
	}

	items := resp.GetFavorites().GetItems()
	if len(items) != 5 {
		t.Fatalf("items len = %d, want 5", len(items))
	}

	first := items[0]
	if first.GetItemId() != "fav-avail" ||
		first.GetProductId() != "p-avail" ||
		first.GetName() != "iPhone 16 Pro" ||
		first.GetBrand() != "Apple" ||
		first.GetCategoryId() != "dien-thoai" ||
		first.GetCategoryName() != "Điện thoại" ||
		first.GetPrice().GetAmountMinor() != 26990000 ||
		first.GetOriginalPrice().GetAmountMinor() != 29990000 ||
		first.GetMaxPrice().GetAmountMinor() != 31990000 ||
		first.GetRating() != 4.9 ||
		first.GetReviewCount() != 128 ||
		first.GetSoldCount() != 540 ||
		first.GetImageUrl() != "https://example.test/iphone.png" ||
		first.GetImageEmoji() != "📱" ||
		len(first.GetImageHue()) != 2 ||
		first.GetBadge() != "hot" ||
		len(first.GetVariants()) != 2 ||
		first.GetVariants()[0].GetSkuId() != "p-avail-v1" ||
		first.GetVariants()[0].GetLabel() != "Đen Titan 256GB" ||
		first.GetVariants()[0].GetColorHex() != "#1c1c1e" ||
		first.GetVariants()[0].GetPrice().GetAmountMinor() != 26990000 ||
		first.GetVariants()[0].GetOriginalPrice().GetAmountMinor() != 29990000 ||
		first.GetVariants()[1].GetSkuId() != "p-avail-v2" ||
		first.GetVariants()[1].GetPrice().GetAmountMinor() != 31990000 ||
		first.GetStatus() != cartv1.FavoriteItemStatus_FAVORITE_ITEM_STATUS_AVAILABLE ||
		first.GetAvailableQuantity() != 6 {
		t.Fatalf("enriched first item mismatch: %+v", first)
	}

	if items[1].GetStatus() != cartv1.FavoriteItemStatus_FAVORITE_ITEM_STATUS_LIMITED || items[1].GetAvailableQuantity() != 5 {
		t.Fatalf("limited item = %+v", items[1])
	}
	if items[2].GetStatus() != cartv1.FavoriteItemStatus_FAVORITE_ITEM_STATUS_OUT_OF_STOCK || items[2].GetAvailableQuantity() != 0 {
		t.Fatalf("out-of-stock item = %+v", items[2])
	}
	if items[3].GetStatus() != cartv1.FavoriteItemStatus_FAVORITE_ITEM_STATUS_UNAVAILABLE || items[3].GetName() != "iPhone 16 Pro" {
		t.Fatalf("soft-deleted item = %+v", items[3])
	}
	if items[4].GetStatus() != cartv1.FavoriteItemStatus_FAVORITE_ITEM_STATUS_UNAVAILABLE || items[4].GetProductId() != "p-orphan" || items[4].GetName() != "" {
		t.Fatalf("orphan hard-deleted item = %+v", items[4])
	}
	if len(inventory.batches) != 1 || len(inventory.batches[0]) != 6 {
		t.Fatalf("inventory batches = %v, want 1 batch across 6 active variant SKUs", inventory.batches)
	}
}

func TestGetFavoritesMarksDegradedOnCatalogOrInventoryOutage(t *testing.T) {
	store := &fakeStore{
		favorites: []domain.FavoriteRecord{
			{ID: "fav-inv-down", ProductID: "p-inv-down"},
			{ID: "fav-cat-down", ProductID: "p-cat-down"},
		},
	}
	catalog := &fakeCatalog{
		products: map[string]*catalogv1.GetProductResponse{"p-inv-down": detailedFavoriteProduct("p-inv-down")},
		errors:   map[string]error{"p-cat-down": status.Error(codes.Unavailable, "catalog down")},
	}
	inventory := &fakeInventory{
		batchErr: status.Error(codes.Unavailable, "inventory down"),
	}

	client := newBufconnClient(t, NewServer(store, catalog, inventory, &fakeAuthorization{allowed: true}))
	resp, err := client.GetFavorites(context.Background(), &cartv1.GetFavoritesRequest{UserId: testUserID})
	if err != nil {
		t.Fatalf("GetFavorites() error = %v", err)
	}
	if !resp.GetFavorites().GetDegraded() {
		t.Fatal("GetFavorites() degraded = false, want true")
	}
	items := resp.GetFavorites().GetItems()
	if items[0].GetStatus() != cartv1.FavoriteItemStatus_FAVORITE_ITEM_STATUS_UNKNOWN || items[0].GetName() != "iPhone 16 Pro" || items[0].GetAvailableQuantity() != 0 {
		t.Fatalf("inventory-degraded item = %+v, want UNKNOWN with catalog fields preserved", items[0])
	}
	if items[1].GetStatus() != cartv1.FavoriteItemStatus_FAVORITE_ITEM_STATUS_UNAVAILABLE || items[1].GetProductId() != "p-cat-down" {
		t.Fatalf("catalog-degraded item = %+v, want UNAVAILABLE", items[1])
	}
}

func TestGetFavoritesBoundedConcurrencyAndRemoveClearMutations(t *testing.T) {
	records := make([]domain.FavoriteRecord, 10)
	for i := range records {
		records[i] = domain.FavoriteRecord{ID: "fav-" + string(rune('a'+i)), ProductID: "prod-" + string(rune('a'+i))}
	}
	entered := make(chan struct{}, len(records))
	release := make(chan struct{})
	catalog := &blockingCatalog{entered: entered, release: release}
	inventory := &fakeInventory{availability: map[string]int32{}}
	store := &fakeStore{favorites: records}

	done := make(chan struct{})
	var resp *cartv1.GetFavoritesResponse
	var err error
	go func() {
		resp, err = NewServer(store, catalog, inventory, &fakeAuthorization{allowed: true}).GetFavorites(
			context.Background(),
			&cartv1.GetFavoritesRequest{UserId: testUserID},
		)
		close(done)
	}()

	for i := 0; i < 8; i++ {
		select {
		case <-entered:
		case <-time.After(time.Second):
			close(release)
			<-done
			t.Fatalf("only %d catalog calls entered bounded fan-out", i)
		}
	}
	close(release)
	<-done
	if err != nil {
		t.Fatalf("GetFavorites() error = %v", err)
	}
	if got := catalog.max.Load(); got != 8 {
		t.Fatalf("max concurrent catalog calls = %d, want 8", got)
	}
	if len(resp.GetFavorites().GetItems()) != 10 {
		t.Fatalf("items len = %d, want 10", len(resp.GetFavorites().GetItems()))
	}

	mutationStore := &fakeStore{
		favorites: []domain.FavoriteRecord{{ID: "fav-1", ProductID: "dt-1"}},
	}
	mutCatalog := &fakeCatalog{products: map[string]*catalogv1.GetProductResponse{"dt-1": detailedFavoriteProduct("dt-1")}, errors: map[string]error{}}
	mutInv := &fakeInventory{availability: map[string]int32{"dt-1-v1": 6, "dt-1-v2": 4}}
	client := newBufconnClient(t, NewServer(mutationStore, mutCatalog, mutInv, &fakeAuthorization{allowed: true}))

	// Removing an absent product succeeds idempotently and returns current list.
	remAbsent, err := client.RemoveFavorite(context.Background(), &cartv1.RemoveFavoriteRequest{UserId: testUserID, ProductId: "absent-id"})
	if err != nil {
		t.Fatalf("RemoveFavorite(absent) error = %v", err)
	}
	if len(remAbsent.GetFavorites().GetItems()) != 1 {
		t.Fatalf("RemoveFavorite(absent) items = %+v, want 1 remaining item", remAbsent.GetFavorites().GetItems())
	}

	clearResp, err := client.ClearFavorites(context.Background(), &cartv1.ClearFavoritesRequest{UserId: testUserID})
	if err != nil {
		t.Fatalf("ClearFavorites() error = %v", err)
	}
	if len(clearResp.GetFavorites().GetItems()) != 0 {
		t.Fatalf("ClearFavorites() items = %+v, want empty", clearResp.GetFavorites().GetItems())
	}

	errStore := &fakeStore{listFavoritesErr: errors.New("db down")}
	errClient := newBufconnClient(t, NewServer(errStore, mutCatalog, mutInv, &fakeAuthorization{allowed: true}))
	if _, err := errClient.GetFavorites(context.Background(), &cartv1.GetFavoritesRequest{UserId: testUserID}); status.Code(err) != codes.Internal {
		t.Fatalf("GetFavorites store error code = %v, want Internal", status.Code(err))
	}
}
