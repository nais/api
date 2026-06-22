-- +goose Up
INSERT INTO
	role_authorizations (role_name, authorization_name)
VALUES
	('GitHub repository', 'teams:configs:create'),
	('GitHub repository', 'teams:configs:update')
;
