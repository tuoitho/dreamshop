package store

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"ecommerce.local/cart-service/internal/domain"
	"ecommerce.local/cart-service/internal/store/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrItemNotFound = errors.New("cart item not found")

type Store struct {
	pool    *pgxpool.Pool
	queries *db.Queries
}

func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool, queries: db.New(pool)}
}

func (s *Store) EnsureCart(ctx context.Context, userID string) (string, error) {
	cartID, err := s.queries.EnsureCart(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("ensure cart: %w", err)
	}
	return cartID, nil
}

func (s *Store) FindCart(ctx context.Context, userID string) (cartID string, found bool, err error) {
	cartID, err = s.queries.FindCart(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("find cart: %w", err)
	}
	return cartID, true, nil
}

func (s *Store) ListItems(ctx context.Context, cartID string) ([]domain.Line, error) {
	items, err := s.queries.ListItems(ctx, cartID)
	if err != nil {
		return nil, fmt.Errorf("list cart items: %w", err)
	}

	lines := make([]domain.Line, 0, len(items))
	for _, item := range items {
		lines = append(lines, domain.Line{
			ID:        item.ID,
			ProductID: item.ProductID,
			SkuID:     item.SkuID,
			Quantity:  item.Quantity,
		})
	}
	return lines, nil
}

func (s *Store) AddItem(ctx context.Context, cartID, productID, skuID string, quantity int32) error {
	if err := s.queries.AddItem(ctx, db.AddItemParams{
		CartID:    cartID,
		ProductID: productID,
		SkuID:     skuID,
		Quantity:  quantity,
	}); err != nil {
		return fmt.Errorf("add cart item: %w", err)
	}
	return nil
}

func (s *Store) UpdateQuantity(ctx context.Context, cartID, itemID string, quantity int32) error {
	rowsAffected, err := s.queries.UpdateQuantity(ctx, db.UpdateQuantityParams{
		CartID:   cartID,
		ItemID:   itemID,
		Quantity: quantity,
	})
	if err != nil {
		return fmt.Errorf("update cart item: %w", err)
	}
	if rowsAffected == 0 {
		return ErrItemNotFound
	}
	return nil
}

func (s *Store) RemoveItem(ctx context.Context, cartID, itemID string) error {
	rowsAffected, err := s.queries.RemoveItem(ctx, db.RemoveItemParams{
		CartID: cartID,
		ItemID: itemID,
	})
	if err != nil {
		return fmt.Errorf("remove cart item: %w", err)
	}
	if rowsAffected == 0 {
		return ErrItemNotFound
	}
	return nil
}

func (s *Store) Clear(ctx context.Context, cartID string) error {
	if err := s.queries.Clear(ctx, cartID); err != nil {
		return fmt.Errorf("clear cart: %w", err)
	}
	return nil
}

func (s *Store) ListFavorites(ctx context.Context, userID string) ([]domain.FavoriteRecord, error) {
	rows, err := s.queries.ListFavorites(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list favorites: %w", err)
	}
	records := make([]domain.FavoriteRecord, 0, len(rows))
	for _, row := range rows {
		records = append(records, domain.FavoriteRecord{
			ID:        row.ID,
			ProductID: row.ProductID,
		})
	}
	return records, nil
}

func (s *Store) AddFavorite(ctx context.Context, userID, productID string) error {
	if err := s.queries.AddFavorite(ctx, db.AddFavoriteParams{
		UserID:    userID,
		ProductID: productID,
	}); err != nil {
		return fmt.Errorf("add favorite: %w", err)
	}
	return nil
}

func (s *Store) RemoveFavorite(ctx context.Context, userID, productID string) error {
	if err := s.queries.RemoveFavorite(ctx, db.RemoveFavoriteParams{
		UserID:    userID,
		ProductID: productID,
	}); err != nil {
		return fmt.Errorf("remove favorite: %w", err)
	}
	return nil
}

func (s *Store) ClearFavorites(ctx context.Context, userID string) error {
	if err := s.queries.ClearFavorites(ctx, userID); err != nil {
		return fmt.Errorf("clear favorites: %w", err)
	}
	return nil
}

// RemoveOrderedItems deletes only the ordered cart items for userID inside a single
// transaction deduplicated by processed_events(event_id).
func (s *Store) RemoveOrderedItems(ctx context.Context, eventID, userID string, items []domain.OrderedItemRef) error {
	cleanEventID := strings.TrimSpace(eventID)
	cleanUserID := strings.TrimSpace(userID)
	if cleanEventID == "" || cleanUserID == "" {
		return errors.New("eventID and userID are required")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin remove ordered items tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx, `
		INSERT INTO processed_events (event_id, event_type, processed_at)
		VALUES ($1, 'OrderConfirmed', now())
		ON CONFLICT (event_id) DO NOTHING
	`, cleanEventID)
	if err != nil {
		return fmt.Errorf("record processed event %q: %w", cleanEventID, err)
	}
	if tag.RowsAffected() == 0 {
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit duplicate event %q: %w", cleanEventID, err)
		}
		return nil
	}

	var cartID string
	err = tx.QueryRow(ctx, "SELECT id::text FROM carts WHERE user_id = $1::uuid", cleanUserID).Scan(&cartID)
	if errors.Is(err, pgx.ErrNoRows) {
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit missing-cart cleanup %q: %w", cleanEventID, err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("find cart for user %q: %w", cleanUserID, err)
	}

	for _, item := range items {
		itemID := strings.TrimSpace(item.CartItemID)
		skuID := strings.TrimSpace(item.SkuID)
		deletedByID := false
		if looksLikeUUID(itemID) {
			delTag, err := tx.Exec(ctx, "DELETE FROM cart_items WHERE cart_id = $1::uuid AND id = $2::uuid", cartID, itemID)
			if err != nil {
				return fmt.Errorf("delete cart item by id %q: %w", itemID, err)
			}
			deletedByID = delTag.RowsAffected() > 0
		}
		if !deletedByID && skuID != "" {
			if _, err := tx.Exec(ctx, "DELETE FROM cart_items WHERE cart_id = $1::uuid AND sku_id = $2", cartID, skuID); err != nil {
				return fmt.Errorf("delete cart item by sku %q: %w", skuID, err)
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit remove ordered items %q: %w", cleanEventID, err)
	}
	return nil
}

func looksLikeUUID(value string) bool {
	if len(value) != 36 {
		return false
	}
	for i := range len(value) {
		c := value[i]
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return false
			}
		default:
			if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
				return false
			}
		}
	}
	return true
}
