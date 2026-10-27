package store

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

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
	otherItems, err := store.ListItems(ctx, otherCartID)
	if err != nil {
		t.Fatalf("list separate cart after clear: %v", err)
	}
	if len(otherItems) != 1 || otherItems[0].Quantity != 7 {
		t.Fatalf("expected clear to preserve separate cart line, got %+v", otherItems)
	}

	missingID, found, err := store.FindCart(ctx, "00000000-0000-0000-0000-000000000099")
	if err != nil {
		t.Fatalf("find missing cart: %v", err)
	}
	if found || missingID != "" {
		t.Fatalf("expected absent cart, got id=%q found=%t", missingID, found)
	}
}

func TestMigrateSerializesConcurrentStartup(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	blocker, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire migration blocker: %v", err)
	}

	const testMigrationLockKey int64 = 724918263
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = blocker.Exec(cleanupCtx, "SELECT pg_advisory_unlock($1)", testMigrationLockKey)
		blocker.Release()
	}()
	if _, err := blocker.Exec(ctx, "SELECT pg_advisory_lock($1)", testMigrationLockKey); err != nil {
		t.Fatalf("lock migration advisory key: %v", err)
	}

	done := make(chan error, 1)
	go func() { done <- Migrate(ctx, pool) }()

	select {
	case err := <-done:
		t.Fatalf("Migrate returned while another startup held the lock: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	var unlocked bool
	if err := blocker.QueryRow(ctx,
		"SELECT pg_advisory_unlock($1)", testMigrationLockKey,
	).Scan(&unlocked); err != nil {
		t.Fatalf("unlock migration advisory key: %v", err)
	}
	if !unlocked {
		t.Fatal("migration advisory key was not held by blocker")
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Migrate() = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Migrate did not finish after migration lock was released")
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

func TestStoreFavoritesIntegration(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	store := New(pool)
	userID := "00000000-0000-0000-0000-000000000101"
	otherUserID := "00000000-0000-0000-0000-000000000102"
	t.Cleanup(func() {
		_ = store.ClearFavorites(ctx, userID)
		_ = store.ClearFavorites(ctx, otherUserID)
	})
	_ = store.ClearFavorites(ctx, userID)
	_ = store.ClearFavorites(ctx, otherUserID)

	if err := store.AddFavorite(ctx, userID, "dt-1"); err != nil {
		t.Fatalf("add first favorite: %v", err)
	}
	time.Sleep(5 * time.Millisecond)
	if err := store.AddFavorite(ctx, userID, "lt-1"); err != nil {
		t.Fatalf("add second favorite: %v", err)
	}
	time.Sleep(5 * time.Millisecond)
	if err := store.AddFavorite(ctx, userID, "dt-1"); err != nil {
		t.Fatalf("duplicate add favorite should be idempotent: %v", err)
	}
	if err := store.AddFavorite(ctx, otherUserID, "tn-1"); err != nil {
		t.Fatalf("add other user favorite: %v", err)
	}

	favorites, err := store.ListFavorites(ctx, userID)
	if err != nil {
		t.Fatalf("list favorites: %v", err)
	}
	if len(favorites) != 2 {
		t.Fatalf("expected 2 favorites, got %+v", favorites)
	}
	if favorites[0].ProductID != "lt-1" || favorites[1].ProductID != "dt-1" {
		t.Fatalf("favorites order = [%s, %s], want [lt-1, dt-1] (newest first, duplicate add keeps original created_at)", favorites[0].ProductID, favorites[1].ProductID)
	}

	if err := store.RemoveFavorite(ctx, userID, "absent-product"); err != nil {
		t.Fatalf("removing absent favorite should succeed idempotently: %v", err)
	}
	if err := store.RemoveFavorite(ctx, userID, "lt-1"); err != nil {
		t.Fatalf("remove favorite: %v", err)
	}
	afterRemove, err := store.ListFavorites(ctx, userID)
	if err != nil {
		t.Fatalf("list after remove: %v", err)
	}
	if len(afterRemove) != 1 || afterRemove[0].ProductID != "dt-1" {
		t.Fatalf("after remove = %+v, want only dt-1", afterRemove)
	}

	otherFavorites, err := store.ListFavorites(ctx, otherUserID)
	if err != nil {
		t.Fatalf("list other user favorites: %v", err)
	}
	if len(otherFavorites) != 1 || otherFavorites[0].ProductID != "tn-1" {
		t.Fatalf("other user favorites = %+v, want only tn-1", otherFavorites)
	}

	if err := store.ClearFavorites(ctx, userID); err != nil {
		t.Fatalf("clear favorites: %v", err)
	}
	cleared, err := store.ListFavorites(ctx, userID)
	if err != nil {
		t.Fatalf("list after clear: %v", err)
	}
	if len(cleared) != 0 {
		t.Fatalf("expected empty favorites after clear, got %+v", cleared)
	}
}
