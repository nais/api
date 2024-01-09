-- name: CreateUser :one
INSERT INTO users (name, email, external_id)
VALUES ($1, LOWER(sqlc.arg(email)), sqlc.arg(external_id))
RETURNING *;

-- name: GetUsersCount :one
SELECT count (*) FROM users;

-- name: GetUsers :many
SELECT * FROM users
ORDER BY name, email ASC LIMIT $1 OFFSET $2;

-- name: GetAllUsers :many
SELECT * FROM users
ORDER BY name, email ASC;

-- name: GetUserByID :one
SELECT * FROM users
WHERE id = $1;

-- name: GetUserByExternalID :one
SELECT * FROM users
WHERE external_id = $1;

-- name: GetUserByEmail :one
SELECT * FROM users
WHERE email = LOWER(sqlc.arg(email));

-- name: GetUserTeams :many
SELECT sqlc.embed(teams), user_roles.role_name FROM user_roles
JOIN teams ON teams.slug = user_roles.target_team_slug
WHERE user_roles.user_id = $1
ORDER BY teams.slug ASC
LIMIT $2 OFFSET $3;

-- name: GetUserTeamsCount :one
SELECT COUNT (*) FROM user_roles
WHERE user_roles.user_id = $1
AND target_team_slug IS NOT NULL;
;

-- name: UpdateUser :one
UPDATE users
SET name = $1, email = LOWER(sqlc.arg(email)), external_id = $2
WHERE id = $3
RETURNING *;

-- name: DeleteUser :exec
DELETE FROM users
WHERE id = $1;
