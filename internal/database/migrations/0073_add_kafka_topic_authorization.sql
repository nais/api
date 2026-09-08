-- +goose Up
INSERT INTO
	authorizations (name, description)
VALUES
	(
		'kafka:update',
		'Permission to update a Kafka Topic.'
	)
;

INSERT INTO
	role_authorizations (role_name, authorization_name)
VALUES
	('Team member', 'kafka:update'),
	('Team owner', 'kafka:update')
;
