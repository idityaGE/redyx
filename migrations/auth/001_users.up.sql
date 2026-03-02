-- Auth service: users table
-- Stores authentication credentials and identity.

CREATE TABLE users (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    email         TEXT        NOT NULL,
    username      TEXT        NOT NULL,
    password_hash TEXT,  -- nullable for OAuth-only accounts
    auth_method   TEXT        NOT NULL DEFAULT 'email' CHECK (auth_method IN ('email', 'google')),
    google_id     TEXT,
    is_verified   BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_users_email ON users (email);
CREATE UNIQUE INDEX idx_users_username ON users (username);
CREATE UNIQUE INDEX idx_users_google_id ON users (google_id) WHERE google_id IS NOT NULL;
