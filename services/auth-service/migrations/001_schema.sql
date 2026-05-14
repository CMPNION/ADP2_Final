CREATE TABLE IF NOT EXISTS users (
    username TEXT PRIMARY KEY,
    password_hash BYTEA NOT NULL,
    role TEXT NOT NULL DEFAULT 'user',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
