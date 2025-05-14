-- +goose Up
INSERT INTO
	authorizations (name, description)
VALUES
	(
		'service_maintenance:update:start',
		'Permission to start ServiceMaintenance Update(s).'
	)
;

INSERT INTO
	role_authorizations (role_name, authorization_name)
VALUES
	('Team member', 'service_maintenance:update:start'),
	('Team owner', 'service_maintenance:update:start')
;
