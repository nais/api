-- +goose Up
ALTER TABLE environments
ADD COLUMN oidc_issuer_url TEXT
;
