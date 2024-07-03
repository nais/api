-- name: Count :one
SELECT COUNT(*) FROM teams;

-- name: List :many
SELECT * FROM teams
ORDER BY
    CASE WHEN @order_by::TEXT = 'slug:asc' THEN slug END ASC,
    CASE WHEN @order_by::TEXT = 'slug:desc' THEN slug END DESC,
    slug ASC
LIMIT sqlc.arg('limit')
OFFSET sqlc.arg('offset');

-- name: Get :one
SELECT * FROM teams
WHERE slug = @slug;

-- name: GetBySlugs :many
SELECT * FROM teams
WHERE slug = ANY(@slugs::slug[])
ORDER BY slug ASC;