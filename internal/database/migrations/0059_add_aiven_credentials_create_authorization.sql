-- +goose Up
INSERT INTO
    authorizations (name, description)
VALUES
    (
        'aiven:credentials:create',
        'Permission to create Aiven service credentials.'
    )
;

INSERT INTO
    role_authorizations (role_name, authorization_name)
VALUES
    ('Team member', 'aiven:credentials:create'),
    ('Team owner', 'aiven:credentials:create')
;

-- +goose Down
DELETE FROM
    role_authorizations
WHERE
    authorization_name = 'aiven:credentials:create'
;

DELETE FROM
    authorizations
WHERE
    name = 'aiven:credentials:create'
;
