-- +goose Up

 CREATE TABLE refresh_tokens (
     user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
     token TEXT PRIMARY KEY  NOT NULL UNIQUE,
     expires_at TIMESTAMPTZ DEFAULT NOW() NOT NULL, 
     created_at TIMESTAMPTZ DEFAULT NOW() NOT NULL,
      revoked_at TIMESTAMPTZ DEFAULT NULL 
 );

-- +goose Down
DROP TABLE refresh_tokens;


