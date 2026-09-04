-- +goose Up
INSERT INTO
	authorizations (name, description)
VALUES
	(
		'kafka:credentials:create',
		'Permission to create Kafka credentials.'
	),
	(
		'opensearch:credentials:create',
		'Permission to create OpenSearch credentials.'
	),
	(
		'valkey:credentials:create',
		'Permission to create Valkey credentials.'
	)
;

INSERT INTO
	role_authorizations (role_name, authorization_name)
VALUES
	('Team member', 'kafka:credentials:create'),
	('Team owner', 'kafka:credentials:create'),
	('Team member', 'opensearch:credentials:create'),
	('Team owner', 'opensearch:credentials:create'),
	('Team member', 'valkey:credentials:create'),
	('Team owner', 'valkey:credentials:create')
;

DELETE FROM role_authorizations
WHERE
	authorization_name = 'aiven:credentials:create'
;

DELETE FROM authorizations
WHERE
	name = 'aiven:credentials:create'
;
