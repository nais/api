-- name: Create :one
INSERT INTO
	service_accounts (name, description, team_slug)
VALUES
	(@name, @description, @team_slug)
ON CONFLICT (name, team_slug) DO UPDATE
SET
	name = EXCLUDED.name
RETURNING
	id,
	(xmax = 0) AS created
;

-- name: AssignRole :exec
INSERT INTO
	service_account_roles (service_account_id, role_name)
VALUES
	(@service_account_id, @role_name)
ON CONFLICT DO NOTHING
;

-- name: CreateToken :one
INSERT INTO
	service_account_tokens (
		expires_at,
		name,
		description,
		token,
		service_account_id
	)
VALUES
	(
		@expires_at,
		@name,
		@description,
		@token,
		@service_account_id
	)
RETURNING
	id
;
