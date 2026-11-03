-- name: EnsureCart :one
INSERT INTO carts (user_id)
VALUES (sqlc.arg(user_id)::uuid)
ON CONFLICT (user_id) DO UPDATE SET updated_at = now()
RETURNING id;

-- name: FindCart :one
SELECT id
FROM carts
WHERE user_id = sqlc.arg(user_id)::uuid;

-- name: ListItems :many
SELECT id, product_id, sku_id, quantity
FROM cart_items
WHERE cart_id = sqlc.arg(cart_id)::uuid
ORDER BY created_at, id;

-- name: AddItem :exec
INSERT INTO cart_items (cart_id, product_id, sku_id, quantity)
VALUES (sqlc.arg(cart_id)::uuid, sqlc.arg(product_id), sqlc.arg(sku_id), sqlc.arg(quantity))
ON CONFLICT (cart_id, sku_id)
DO UPDATE SET product_id = EXCLUDED.product_id, quantity = cart_items.quantity + EXCLUDED.quantity, updated_at = now();

-- name: UpdateQuantity :execrows
UPDATE cart_items
SET quantity = sqlc.arg(quantity), updated_at = now()
WHERE cart_id = sqlc.arg(cart_id)::uuid AND id = sqlc.arg(item_id)::uuid;

-- name: RemoveItem :execrows
DELETE FROM cart_items
WHERE cart_id = sqlc.arg(cart_id)::uuid AND id = sqlc.arg(item_id)::uuid;

-- name: Clear :exec
DELETE FROM cart_items
WHERE cart_id = sqlc.arg(cart_id)::uuid;
