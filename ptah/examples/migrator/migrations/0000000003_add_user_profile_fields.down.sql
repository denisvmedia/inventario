-- Rollback user profile fields

ALTER TABLE users 
DROP COLUMN IF EXISTS last_login_at,
DROP COLUMN IF EXISTS is_active,
DROP COLUMN IF EXISTS avatar_url,
DROP COLUMN IF EXISTS bio;
