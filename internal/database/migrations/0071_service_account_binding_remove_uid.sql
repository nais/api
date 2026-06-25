-- +goose Up
ALTER TABLE service_account_workload_bindings
DROP COLUMN kubernetes_service_account_uid
;
