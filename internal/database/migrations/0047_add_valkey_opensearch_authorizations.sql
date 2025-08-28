-- +goose Up
INSERT INTO
	authorizations (name, description)
VALUES
	(
		'valkeys:create',
		'Permission to create Valkey instances.'
	),
	(
		'valkeys:delete',
		'Permission to delete Valkey instances.'
	),
	(
		'valkeys:update',
		'Permission to update Valkey instances.'
	),
	(
		'opensearches:create',
		'Permission to create OpenSearch instances.'
	),
	(
		'opensearches:delete',
		'Permission to delete OpenSearch instances.'
	),
	(
		'opensearches:update',
		'Permission to update OpenSearch instances.'
	)
;

INSERT INTO
	role_authorizations (role_name, authorization_name)
VALUES
	('Team member', 'valkeys:create'),
	('Team member', 'valkeys:delete'),
	('Team member', 'valkeys:update'),
	('Team member', 'opensearches:create'),
	('Team member', 'opensearches:delete'),
	('Team member', 'opensearches:update'),
	('Team owner', 'valkeys:create'),
	('Team owner', 'valkeys:delete'),
	('Team owner', 'valkeys:update'),
	('Team owner', 'opensearches:create'),
	('Team owner', 'opensearches:delete'),
	('Team owner', 'opensearches:update')
;
