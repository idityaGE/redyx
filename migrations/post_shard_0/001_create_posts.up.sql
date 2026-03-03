-- Post shard schema: posts + saved_posts tables with all indexes.

CREATE TABLE IF NOT EXISTS posts (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    title           TEXT        NOT NULL CHECK (char_length(title) <= 300),
    body            TEXT        NOT NULL DEFAULT '' CHECK (char_length(body) <= 40000),
    url             TEXT        NOT NULL DEFAULT '',
    post_type       SMALLINT    NOT NULL DEFAULT 1,  -- 1=text, 2=link, 3=media
    author_id       UUID        NOT NULL,
    author_username TEXT        NOT NULL,
    community_id    UUID        NOT NULL,
    community_name  TEXT        NOT NULL,
    vote_score      INTEGER     NOT NULL DEFAULT 0,
    upvotes         INTEGER     NOT NULL DEFAULT 0,
    downvotes       INTEGER     NOT NULL DEFAULT 0,
    comment_count   INTEGER     NOT NULL DEFAULT 0,
    hot_score       DOUBLE PRECISION NOT NULL DEFAULT 0,
    is_edited       BOOLEAN     NOT NULL DEFAULT false,
    is_deleted      BOOLEAN     NOT NULL DEFAULT false,
    is_pinned       BOOLEAN     NOT NULL DEFAULT false,
    is_anonymous    BOOLEAN     NOT NULL DEFAULT false,
    thumbnail_url   TEXT        NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    edited_at       TIMESTAMPTZ
);

-- Community feed indexes (single shard) — one per sort order
CREATE INDEX IF NOT EXISTS idx_posts_community_hot ON posts (community_id, hot_score DESC, id DESC)
    WHERE is_deleted = false;
CREATE INDEX IF NOT EXISTS idx_posts_community_new ON posts (community_id, created_at DESC, id DESC)
    WHERE is_deleted = false;
CREATE INDEX IF NOT EXISTS idx_posts_community_top ON posts (community_id, vote_score DESC, id DESC)
    WHERE is_deleted = false;

-- Home feed cross-shard queries need efficient community_id IN (...) filtering
CREATE INDEX IF NOT EXISTS idx_posts_community_id ON posts (community_id);

-- Author's posts (for profile page, cross-shard)
CREATE INDEX IF NOT EXISTS idx_posts_author ON posts (author_id, created_at DESC);

-- Saved posts (centralized on shard_0)
CREATE TABLE IF NOT EXISTS saved_posts (
    user_id    UUID        NOT NULL,
    post_id    UUID        NOT NULL,
    saved_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, post_id)
);

CREATE INDEX IF NOT EXISTS idx_saved_posts_user ON saved_posts (user_id, saved_at DESC);
