-- +goose Up
DELETE FROM service_accounts
WHERE
	name LIKE 'nais-%'
;
