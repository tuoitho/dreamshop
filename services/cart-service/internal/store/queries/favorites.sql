-- name: ListFavorites :many
SELECT id, product_id
FROM favorite_items
WHERE user_id = sqlc.arg(user_id)::uuid
ORDER BY created_at DESC, id DESC;

-- name: AddFavorite :exec
INSERT INTO favorite_items (user_id, product_id)
VALUES (sqlc.arg(user_id)::uuid, sqlc.arg(product_id))
ON CONFLICT (user_id, product_id) DO NOTHING;

-- name: RemoveFavorite :exec
DELETE FROM favorite_items
WHERE user_id = sqlc.arg(user_id)::uuid AND product_id = sqlc.arg(product_id);

-- name: ClearFavorites :exec
DELETE FROM favorite_items
WHERE user_id = sqlc.arg(user_id)::uuid;
