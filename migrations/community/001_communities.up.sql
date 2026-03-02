-- Community service: communities and members tables
-- Stores community metadata and membership records.

CREATE TABLE communities (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    name         TEXT        NOT NULL CHECK (name ~ '^[a-zA-Z0-9_]{3,25}$'),
    description  TEXT        NOT NULL DEFAULT '',
    rules        JSONB       NOT NULL DEFAULT '[]',
    banner_url   TEXT        NOT NULL DEFAULT '',
    icon_url     TEXT        NOT NULL DEFAULT '',
    visibility   SMALLINT    NOT NULL DEFAULT 1,  -- 1=public, 2=restricted, 3=private
    member_count INTEGER     NOT NULL DEFAULT 0,
    owner_id     UUID        NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_communities_name ON communities (name);

CREATE TABLE community_members (
    community_id UUID        NOT NULL REFERENCES communities(id) ON DELETE CASCADE,
    user_id      UUID        NOT NULL,
    role         TEXT        NOT NULL DEFAULT 'member' CHECK (role IN ('member', 'moderator', 'admin')),
    joined_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (community_id, user_id)
);

CREATE INDEX idx_community_members_user_id ON community_members (user_id);
