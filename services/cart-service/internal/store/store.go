package store

import (
	"context"
	"errors"
	"fmt"

	"ecommerce.local/cart-service/internal/domain"
	"ecommerce.local/cart-service/internal/store/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrItemNotFound = errors.New("cart item not found")

type Store struct {
	queries *db.Queries
}

func New(pool *pgxpool.Pool) *Store {
	return &Store{queries: db.New(pool)}
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
			ID:           item.ID,
			ProductID:    item.ProductID,
			VariantLabel: item.VariantLabel,
			Quantity:     item.Quantity,
		})
	}
	return lines, nil
}

func (s *Store) AddItem(ctx context.Context, cartID, productID, variantLabel string, quantity int32) error {
	if err := s.queries.AddItem(ctx, db.AddItemParams{
		CartID:       cartID,
		ProductID:    productID,
		VariantLabel: variantLabel,
		Quantity:     quantity,
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
