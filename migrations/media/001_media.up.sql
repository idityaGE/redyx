CREATE TABLE IF NOT EXISTS media_items (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL,
  filename VARCHAR(255) NOT NULL,
  content_type VARCHAR(100) NOT NULL,
  size_bytes BIGINT NOT NULL,
  media_type VARCHAR(20) NOT NULL,  -- 'image', 'video', 'gif'
  status VARCHAR(20) NOT NULL DEFAULT 'pending',  -- pending, processing, ready, failed
  s3_key VARCHAR(500) NOT NULL,
  url TEXT,
  thumbnail_url TEXT,
  error_message TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_media_items_user ON media_items(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_media_items_status ON media_items(status);
