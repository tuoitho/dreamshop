package store

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"ecommerce.local/inventory-service/internal/domain"
	"github.com/jackc/pgx/v5"
)

const (
	reservationStatusActive    = "ACTIVE"
	reservationStatusConfirmed = "CONFIRMED"
	reservationStatusReleased  = "RELEASED"
	reservationStatusRestocked = "RESTOCKED"
)

// ReserveStock atomically reserves stock across all requested SKUs for orderID.
// Duplicate SKUs in items are aggregated, and stock rows are updated in ascending
// sku_id order to prevent cross-order deadlocks.
func (s *Store) ReserveStock(ctx context.Context, orderID string, items []domain.ReservationLine, expiresAt time.Time) (string, error) {
	aggregated := aggregateReservationLines(items)
	if len(aggregated) == 0 {
		return "", errors.New("at least one reservation line is required")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("begin reserve stock tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx, `
		INSERT INTO stock_reservations (order_id, status, expires_at, created_at, updated_at)
		VALUES ($1, $2, $3, now(), now())
		ON CONFLICT (order_id) DO NOTHING
	`, orderID, reservationStatusActive, expiresAt.UTC())
	if err != nil {
		return "", fmt.Errorf("insert stock reservation %q: %w", orderID, err)
	}

	if tag.RowsAffected() == 0 {
		var existingStatus string
		if err := tx.QueryRow(ctx, `
			SELECT status
			FROM stock_reservations
			WHERE order_id = $1
			FOR UPDATE
		`, orderID).Scan(&existingStatus); err != nil {
			return "", fmt.Errorf("lock existing reservation %q: %w", orderID, err)
		}
		if existingStatus == reservationStatusReleased {
			return "", domain.ErrReservationAlreadyReleased
		}
		if err := tx.Commit(ctx); err != nil {
			return "", fmt.Errorf("commit replay reservation %q: %w", orderID, err)
		}
		return existingStatus, nil
	}

	for _, line := range aggregated {
		var qty, reserved int32
		err := tx.QueryRow(ctx, `
			UPDATE stock
			SET reserved = reserved + $2,
			    updated_at = now()
			WHERE sku_id = $1
			  AND (quantity - reserved) >= $2
			RETURNING quantity, reserved
		`, line.SkuID, line.Quantity).Scan(&qty, &reserved)
		if errors.Is(err, pgx.ErrNoRows) {
			_ = tx.Rollback(ctx)
			cur, getErr := s.Get(ctx, line.SkuID)
			avail := int32(0)
			prodID := line.ProductID
			if getErr == nil {
				avail = cur.AvailableQuantity()
				if prodID == "" {
					prodID = cur.ProductID
				}
			}
			return "", &domain.InsufficientStockError{
				SkuID:     line.SkuID,
				ProductID: prodID,
				Available: avail,
				Requested: line.Quantity,
			}
		}
		if err != nil {
			return "", fmt.Errorf("reserve sku %q: %w", line.SkuID, err)
		}

		if _, err := tx.Exec(ctx, `
			INSERT INTO stock_reservation_items (order_id, sku_id, product_id, quantity)
			VALUES ($1, $2, $3, $4)
		`, orderID, line.SkuID, line.ProductID, line.Quantity); err != nil {
			return "", fmt.Errorf("insert reservation item (%q, %q): %w", orderID, line.SkuID, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit reserve stock %q: %w", orderID, err)
	}
	return reservationStatusActive, nil
}

// ConfirmReservation deducts physical quantity and reserved units for an ACTIVE
// reservation in ascending sku_id order. Replays on CONFIRMED, RELEASED or RESTOCKED
// are no-ops.
func (s *Store) ConfirmReservation(ctx context.Context, orderID string) (string, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("begin confirm reservation tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var currentStatus string
	err = tx.QueryRow(ctx, `
		SELECT status
		FROM stock_reservations
		WHERE order_id = $1
		FOR UPDATE
	`, orderID).Scan(&currentStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("lock reservation %q for confirm: %w", orderID, err)
	}

	if currentStatus != reservationStatusActive {
		if err := tx.Commit(ctx); err != nil {
			return "", fmt.Errorf("commit idempotent confirm %q: %w", orderID, err)
		}
		return currentStatus, nil
	}

	items, err := loadReservationItemsTx(ctx, tx, orderID)
	if err != nil {
		return "", err
	}
	for _, item := range items {
		if _, err := tx.Exec(ctx, `
			UPDATE stock
			SET quantity = quantity - $2,
			    reserved = reserved - $2,
			    updated_at = now()
			WHERE sku_id = $1
		`, item.SkuID, item.Quantity); err != nil {
			return "", fmt.Errorf("confirm stock deduction %q: %w", item.SkuID, err)
		}
	}

	if _, err := tx.Exec(ctx, `
		UPDATE stock_reservations
		SET status = $2,
		    updated_at = now()
		WHERE order_id = $1
	`, orderID, reservationStatusConfirmed); err != nil {
		return "", fmt.Errorf("mark reservation confirmed %q: %w", orderID, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit confirm reservation %q: %w", orderID, err)
	}
	return reservationStatusConfirmed, nil
}

// ReleaseReservation either inserts a RELEASED tombstone when no reservation exists
// yet (blocking late ReserveStock calls without unique-constraint errors) or releases
// reserved units for an ACTIVE reservation in ascending sku_id order.
func (s *Store) ReleaseReservation(ctx context.Context, orderID, reason string) (string, error) {
	cleanReason := strings.TrimSpace(reason)
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("begin release reservation tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx, `
		INSERT INTO stock_reservations (order_id, status, expires_at, release_reason, created_at, updated_at)
		VALUES ($1, $2, now(), $3, now(), now())
		ON CONFLICT (order_id) DO NOTHING
	`, orderID, reservationStatusReleased, cleanReason)
	if err != nil {
		return "", fmt.Errorf("insert release tombstone %q: %w", orderID, err)
	}
	if tag.RowsAffected() == 1 {
		if err := tx.Commit(ctx); err != nil {
			return "", fmt.Errorf("commit release tombstone %q: %w", orderID, err)
		}
		return reservationStatusReleased, nil
	}

	var currentStatus string
	if err := tx.QueryRow(ctx, `
		SELECT status
		FROM stock_reservations
		WHERE order_id = $1
		FOR UPDATE
	`, orderID).Scan(&currentStatus); err != nil {
		return "", fmt.Errorf("lock reservation %q for release: %w", orderID, err)
	}

	if currentStatus != reservationStatusActive {
		if err := tx.Commit(ctx); err != nil {
			return "", fmt.Errorf("commit idempotent release %q: %w", orderID, err)
		}
		return currentStatus, nil
	}

	items, err := loadReservationItemsTx(ctx, tx, orderID)
	if err != nil {
		return "", err
	}
	for _, item := range items {
		if _, err := tx.Exec(ctx, `
			UPDATE stock
			SET reserved = reserved - $2,
			    updated_at = now()
			WHERE sku_id = $1
		`, item.SkuID, item.Quantity); err != nil {
			return "", fmt.Errorf("release reserved stock %q: %w", item.SkuID, err)
		}
	}

	if _, err := tx.Exec(ctx, `
		UPDATE stock_reservations
		SET status = $2,
		    release_reason = $3,
		    updated_at = now()
		WHERE order_id = $1
	`, orderID, reservationStatusReleased, cleanReason); err != nil {
		return "", fmt.Errorf("mark reservation released %q: %w", orderID, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit release reservation %q: %w", orderID, err)
	}
	return reservationStatusReleased, nil
}

// RestockReservation returns stock for an order cancelled after confirmation. A
// CONFIRMED reservation adds its quantities back and becomes RESTOCKED. An ACTIVE
// reservation (the confirmation event has not been applied yet) is released
// instead, so a confirmation arriving later finds nothing to deduct. A missing
// reservation gets a RELEASED tombstone; other states are idempotent no-ops.
func (s *Store) RestockReservation(ctx context.Context, orderID, reason string) (string, error) {
	cleanReason := strings.TrimSpace(reason)
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("begin restock reservation tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx, `
		INSERT INTO stock_reservations (order_id, status, expires_at, release_reason, created_at, updated_at)
		VALUES ($1, $2, now(), $3, now(), now())
		ON CONFLICT (order_id) DO NOTHING
	`, orderID, reservationStatusReleased, cleanReason)
	if err != nil {
		return "", fmt.Errorf("insert restock tombstone %q: %w", orderID, err)
	}
	if tag.RowsAffected() == 1 {
		if err := tx.Commit(ctx); err != nil {
			return "", fmt.Errorf("commit restock tombstone %q: %w", orderID, err)
		}
		return reservationStatusReleased, nil
	}

	var currentStatus string
	if err := tx.QueryRow(ctx, `
		SELECT status
		FROM stock_reservations
		WHERE order_id = $1
		FOR UPDATE
	`, orderID).Scan(&currentStatus); err != nil {
		return "", fmt.Errorf("lock reservation %q for restock: %w", orderID, err)
	}

	var stockUpdate, nextStatus string
	switch currentStatus {
	case reservationStatusConfirmed:
		stockUpdate = `UPDATE stock SET quantity = quantity + $2, updated_at = now() WHERE sku_id = $1`
		nextStatus = reservationStatusRestocked
	case reservationStatusActive:
		stockUpdate = `UPDATE stock SET reserved = reserved - $2, updated_at = now() WHERE sku_id = $1`
		nextStatus = reservationStatusReleased
	default:
		if err := tx.Commit(ctx); err != nil {
			return "", fmt.Errorf("commit idempotent restock %q: %w", orderID, err)
		}
		return currentStatus, nil
	}

	items, err := loadReservationItemsTx(ctx, tx, orderID)
	if err != nil {
		return "", err
	}
	for _, item := range items {
		if _, err := tx.Exec(ctx, stockUpdate, item.SkuID, item.Quantity); err != nil {
			return "", fmt.Errorf("restock sku %q: %w", item.SkuID, err)
		}
	}

	if _, err := tx.Exec(ctx, `
		UPDATE stock_reservations
		SET status = $2,
		    release_reason = $3,
		    updated_at = now()
		WHERE order_id = $1
	`, orderID, nextStatus, cleanReason); err != nil {
		return "", fmt.Errorf("mark reservation %s %q: %w", nextStatus, orderID, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit restock reservation %q: %w", orderID, err)
	}
	return nextStatus, nil
}

func loadReservationItemsTx(ctx context.Context, tx pgx.Tx, orderID string) ([]domain.ReservationLine, error) {
	rows, err := tx.Query(ctx, `
		SELECT sku_id, product_id, quantity
		FROM stock_reservation_items
		WHERE order_id = $1
		ORDER BY sku_id ASC
	`, orderID)
	if err != nil {
		return nil, fmt.Errorf("query reservation items %q: %w", orderID, err)
	}
	defer rows.Close()

	var items []domain.ReservationLine
	for rows.Next() {
		var line domain.ReservationLine
		if err := rows.Scan(&line.SkuID, &line.ProductID, &line.Quantity); err != nil {
			return nil, fmt.Errorf("scan reservation item %q: %w", orderID, err)
		}
		items = append(items, line)
	}
	return items, rows.Err()
}

func aggregateReservationLines(items []domain.ReservationLine) []domain.ReservationLine {
	bySku := make(map[string]domain.ReservationLine, len(items))
	for _, raw := range items {
		skuID := strings.TrimSpace(raw.SkuID)
		if skuID == "" || raw.Quantity <= 0 {
			continue
		}
		cur, exists := bySku[skuID]
		if !exists {
			cur = domain.ReservationLine{
				SkuID:     skuID,
				ProductID: strings.TrimSpace(raw.ProductID),
			}
		} else if cur.ProductID == "" {
			cur.ProductID = strings.TrimSpace(raw.ProductID)
		}
		cur.Quantity += raw.Quantity
		bySku[skuID] = cur
	}

	out := make([]domain.ReservationLine, 0, len(bySku))
	for _, line := range bySku {
		out = append(out, line)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].SkuID < out[j].SkuID
	})
	return out
}
