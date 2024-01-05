-- +goose Up
UPDATE cost SET app = '' WHERE app IS NULL;
ALTER TABLE cost ALTER COLUMN app SET NOT NULL;

-- +goose Down
ALTER TABLE cost ALTER COLUMN app DROP NOT NULL;
UPDATE cost SET app = NULL WHERE app = '';
