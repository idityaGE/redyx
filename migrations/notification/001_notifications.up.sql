CREATE TABLE IF NOT EXISTS notifications (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL,
  type VARCHAR(30) NOT NULL,  -- 'post_reply', 'comment_reply', 'mention'
  actor_id UUID NOT NULL,
  actor_username VARCHAR(50) NOT NULL,
  target_id VARCHAR(255) NOT NULL,  -- post_id or comment_id
  target_type VARCHAR(20) NOT NULL, -- 'post' or 'comment'
  post_id VARCHAR(255) NOT NULL,    -- for navigation context
  community_name VARCHAR(50) NOT NULL,
  message TEXT NOT NULL,
  is_read BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_notifications_user_unread ON notifications(user_id, is_read, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_notifications_user_created ON notifications(user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS notification_preferences (
  user_id UUID PRIMARY KEY,
  post_replies BOOLEAN NOT NULL DEFAULT TRUE,
  comment_replies BOOLEAN NOT NULL DEFAULT TRUE,
  mentions BOOLEAN NOT NULL DEFAULT TRUE,
  muted_communities TEXT[] NOT NULL DEFAULT '{}',
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
