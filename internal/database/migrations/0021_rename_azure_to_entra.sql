-- +goose Up
ALTER TABLE teams
RENAME COLUMN azure_group_id TO entra_id_group_id
;
