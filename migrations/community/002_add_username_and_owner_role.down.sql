-- Revert username column and restore original role CHECK constraint.

ALTER TABLE community_members DROP COLUMN IF EXISTS username;

ALTER TABLE community_members DROP CONSTRAINT IF EXISTS community_members_role_check;
ALTER TABLE community_members ADD CONSTRAINT community_members_role_check
    CHECK (role IN ('member', 'moderator', 'admin'));
