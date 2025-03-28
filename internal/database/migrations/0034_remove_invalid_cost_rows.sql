-- +goose Up
DELETE FROM cost
WHERE
	service = 'valkey'
;
