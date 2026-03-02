-- User service: profiles table
-- Stores public profile information separate from auth credentials.

CREATE TABLE profiles (
    user_id      UUID        PRIMARY KEY,
    username     TEXT        NOT NULL,
    display_name TEXT        NOT NULL DEFAULT '',
    bio          TEXT        NOT NULL DEFAULT '' CHECK (length(bio) <= 500),
    avatar_url   TEXT        NOT NULL DEFAULT '',
    karma        INTEGER     NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at   TIMESTAMPTZ  -- nullable soft delete
);

CREATE UNIQUE INDEX idx_profiles_username ON profiles (username);
