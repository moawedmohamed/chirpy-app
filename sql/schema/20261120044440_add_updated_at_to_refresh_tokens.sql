-- +goose Up

ALTER TABLE refresh_tokens
ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

-- +goose Down

ALTER TABLE refresh_tokens
DROP COLUMN updated_at;
