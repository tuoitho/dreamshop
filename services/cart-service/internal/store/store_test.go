package store

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set")
	}

	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		t.Fatalf("connect pool: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(context.Background()); err != nil {
		t.Fatalf("ping database: %v", err)
	}
	return pool
}

func TestStoreIntegration(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	store := New(pool)
	userID := "00000000-0000-0000-0000-000000000001"
	otherUserID := "00000000-0000-0000-0000-000000000002"
	t.Cleanup(func() {
		for _, testUserID := range []string{userID, otherUserID} {
			cartID, found, err := store.FindCart(ctx, testUserID)
			if err != nil {
				t.Errorf("find test cart for cleanup: %v", err)
				continue
			}
			if found {
				if err := store.Clear(ctx, cartID); err != nil {
					t.Errorf("clear test cart: %v", err)
				}
			}
		}
	})
	cartID, err := store.EnsureCart(ctx, userID)
	if err != nil {
		t.Fatalf("ensure cart: %v", err)
	}
	secondCartID, err := store.EnsureCart(ctx, userID)
	if err != nil {
		t.Fatalf("ensure same cart: %v", err)
	}
	if cartID != secondCartID {
		t.Fatalf("same user received different cart IDs: %s and %s", cartID, secondCartID)
	}

	otherCartID, err := store.EnsureCart(ctx, otherUserID)
	if err != nil {
		t.Fatalf("ensure other cart: %v", err)
	}
	if err := store.AddItem(ctx, cartID, "product-1", "red", 2); err != nil {
		t.Fatalf("add item: %v", err)
	}
	if err := store.AddItem(ctx, cartID, "product-1", "red", 3); err != nil {
		t.Fatalf("upsert duplicate item: %v", err)
	}
	if err := store.AddItem(ctx, otherCartID, "product-1", "red", 7); err != nil {
		t.Fatalf("add other-cart item: %v", err)
	}

	items, err := store.ListItems(ctx, cartID)
	if err != nil {
		t.Fatalf("list items: %v", err)
	}
	if len(items) != 1 || items[0].Quantity != 5 {
		t.Fatalf("expected one item with quantity 5, got %+v", items)
	}
	itemID := items[0].ID

	if err := store.UpdateQuantity(ctx, otherCartID, itemID, 9); !errors.Is(err, ErrItemNotFound) {
		t.Fatalf("cross-cart update error = %v, want ErrItemNotFound", err)
	}
	if err := store.RemoveItem(ctx, otherCartID, itemID); !errors.Is(err, ErrItemNotFound) {
		t.Fatalf("cross-cart remove error = %v, want ErrItemNotFound", err)
	}
	if err := store.UpdateQuantity(ctx, cartID, itemID, 4); err != nil {
		t.Fatalf("update quantity: %v", err)
	}

	if err := store.Clear(ctx, cartID); err != nil {
		t.Fatalf("clear cart: %v", err)
	}
	items, err = store.ListItems(ctx, cartID)
	if err != nil {
		t.Fatalf("list cleared cart: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected clear to remove all items, got %+v", items)
	}

	missingID, found, err := store.FindCart(ctx, "00000000-0000-0000-0000-000000000099")
	if err != nil {
		t.Fatalf("find missing cart: %v", err)
	}
	if found || missingID != "" {
		t.Fatalf("expected absent cart, got id=%q found=%t", missingID, found)
	}
}

func TestStoreMutationMapsMissingItemToSentinel(t *testing.T) {
	pool := testPool(t)
	if err := Migrate(context.Background(), pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	store := New(pool)
	ctx := context.Background()
	cartID := "00000000-0000-0000-0000-000000000099"
	itemID := "00000000-0000-0000-0000-000000000098"

	if err := store.UpdateQuantity(ctx, cartID, itemID, 1); !errors.Is(err, ErrItemNotFound) {
		t.Fatalf("UpdateQuantity() = %v, want ErrItemNotFound", err)
	}
	if err := store.RemoveItem(ctx, cartID, itemID); !errors.Is(err, ErrItemNotFound) {
		t.Fatalf("RemoveItem() = %v, want ErrItemNotFound", err)
	}
}
