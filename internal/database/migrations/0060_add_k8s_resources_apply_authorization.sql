-- +goose Up
INSERT INTO
	authorizations (name, description)
VALUES
	(
		'k8s_resources:apply',
		'Permission to apply generic Kubernetes resources on behalf of a team.'
	)
;

INSERT INTO
	role_authorizations (role_name, authorization_name)
VALUES
	('Team member', 'k8s_resources:apply'),
	('Team owner', 'k8s_resources:apply'),
	('GitHub repository', 'k8s_resources:apply')
;

-- +goose Down
DELETE FROM role_authorizations
WHERE
	authorization_name = 'k8s_resources:apply'
;

DELETE FROM authorizations
WHERE
	name = 'k8s_resources:apply'
;
