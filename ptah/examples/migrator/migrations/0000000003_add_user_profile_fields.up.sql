-- Add profile fields to users table

ALTER TABLE users 
ADD COLUMN bio TEXT,
ADD COLUMN avatar_url VARCHAR(500),
ADD COLUMN is_active BOOLEAN NOT NULL DEFAULT true,
ADD COLUMN last_login_at TIMESTAMP;
