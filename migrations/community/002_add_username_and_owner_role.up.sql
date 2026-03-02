-- Add username column to community_members for denormalized access
-- and update role CHECK constraint to include 'owner' role.

ALTER TABLE community_members ADD COLUMN IF NOT EXISTS username TEXT NOT NULL DEFAULT '';

-- Drop old check constraint and add new one with 'owner' role
ALTER TABLE community_members DROP CONSTRAINT IF EXISTS community_members_role_check;
ALTER TABLE community_members ADD CONSTRAINT community_members_role_check
    CHECK (role IN ('member', 'moderator', 'owner'));
