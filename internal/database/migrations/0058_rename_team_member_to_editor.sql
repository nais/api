-- +goose Up
-- Rename "Team member" role to "Team editor"
UPDATE roles
SET
	name = 'Team editor'
WHERE
	name = 'Team member'
;

-- Update all user_roles references
UPDATE user_roles
SET
	role_name = 'Team editor'
WHERE
	role_name = 'Team member'
;

-- Update all role_authorizations references
UPDATE role_authorizations
SET
	role_name = 'Team editor'
WHERE
	role_name = 'Team member'
;

-- +goose Down
UPDATE role_authorizations
SET
	role_name = 'Team member'
WHERE
	role_name = 'Team editor'
;

UPDATE user_roles
SET
	role_name = 'Team member'
WHERE
	role_name = 'Team editor'
;

UPDATE roles
SET
	name = 'Team member'
WHERE
	name = 'Team editor'
;
