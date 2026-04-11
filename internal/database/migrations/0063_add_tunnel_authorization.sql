-- +goose Up
INSERT INTO
	authorizations (name, description)
VALUES
	(
		'tunnels:create',
		'Permission to create WireGuard tunnels.'
	)
;

INSERT INTO
	role_authorizations (role_name, authorization_name)
VALUES
	('Team member', 'tunnels:create'),
	('Team owner', 'tunnels:create')
;

-- +goose Down
DELETE FROM role_authorizations WHERE authorization_name = 'tunnels:create';
DELETE FROM authorizations WHERE name = 'tunnels:create';
