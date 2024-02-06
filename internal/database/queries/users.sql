-- name: CreateUser :one
INSERT INTO users (name, email, external_id)
VALUES (@name, LOWER(@email), @external_id)
RETURNING *;

-- name: GetUsersCount :one
SELECT COUNT(*) FROM users;

-- name: GetUsers :many
SELECT * FROM users
ORDER BY name, email ASC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: GetUserByID :one
SELECT * FROM users
WHERE id = @id;

-- name: GetUsersByIDs :many
SELECT * FROM users
WHERE id = ANY(@ids::uuid[])
ORDER BY name, email ASC;

-- name: GetUserByExternalID :one
SELECT * FROM users
WHERE external_id = @external_id;

-- name: GetUserByEmail :one
SELECT * FROM users
WHERE email = LOWER(@email);

-- name: GetUserTeams :many
SELECT sqlc.embed(teams), user_roles.role_name FROM user_roles
JOIN teams ON teams.slug = user_roles.target_team_slug
WHERE user_roles.user_id = @user_id
ORDER BY teams.slug ASC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: GetUserTeamsCount :one
SELECT COUNT(*) FROM user_roles
WHERE user_roles.user_id = @user_id
AND target_team_slug IS NOT NULL;

-- name: UpdateUser :one
UPDATE users
SET name = @name, email = LOWER(@email), external_id = @external_id
WHERE id = @id
RETURNING *;

-- name: DeleteUser :exec
DELETE FROM users
WHERE id = @id;
