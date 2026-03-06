-- Reports from users and spam detection
CREATE TABLE IF NOT EXISTS reports (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    community_id    UUID        NOT NULL,
    community_name  TEXT        NOT NULL,
    content_id      UUID        NOT NULL,
    content_type    SMALLINT    NOT NULL,  -- 1=post, 2=comment
    reporter_id     UUID        NOT NULL,
    reason          TEXT        NOT NULL,
    source          TEXT        NOT NULL DEFAULT 'user',  -- 'user' or 'spam-detection'
    status          TEXT        NOT NULL DEFAULT 'active', -- 'active', 'resolved'
    resolved_action TEXT,       -- 'removed', 'dismissed', 'banned'
    resolved_by     UUID,
    resolved_at     TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_reports_community_active
    ON reports (community_id, status, created_at DESC)
    WHERE status = 'active';
CREATE INDEX IF NOT EXISTS idx_reports_content
    ON reports (content_id, content_type);

-- User bans per community
CREATE TABLE IF NOT EXISTS bans (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    community_id    UUID        NOT NULL,
    community_name  TEXT        NOT NULL,
    user_id         UUID        NOT NULL,
    username        TEXT        NOT NULL,
    reason          TEXT        NOT NULL,
    banned_by       UUID        NOT NULL,
    banned_by_username TEXT     NOT NULL DEFAULT '',
    duration_seconds BIGINT     NOT NULL DEFAULT 0,  -- 0 = permanent
    expires_at      TIMESTAMPTZ,  -- NULL = permanent
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(community_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_bans_community_active
    ON bans (community_id, expires_at);
CREATE INDEX IF NOT EXISTS idx_bans_user
    ON bans (user_id);

-- Moderation action log
CREATE TABLE IF NOT EXISTS mod_log (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    community_id    UUID        NOT NULL,
    community_name  TEXT        NOT NULL,
    moderator_id    UUID        NOT NULL,
    moderator_username TEXT     NOT NULL,
    action          TEXT        NOT NULL,  -- 'remove_post', 'remove_comment', 'ban_user', 'unban_user', 'pin_post', 'unpin_post', 'dismiss_report', 'restore_content'
    target_id       TEXT        NOT NULL,
    target_type     TEXT        NOT NULL,  -- 'post', 'comment', 'user'
    reason          TEXT        NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_mod_log_community
    ON mod_log (community_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_mod_log_action
    ON mod_log (community_id, action, created_at DESC);
