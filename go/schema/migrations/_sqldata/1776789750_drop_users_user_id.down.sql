-- Restore the legacy users.user_id self-FK column. Best-effort: the
-- original values (each user's id) can be reconstructed from users.id
-- because the column was always a self-reference.

ALTER TABLE users ADD COLUMN IF NOT EXISTS user_id TEXT;
UPDATE users SET user_id = id WHERE user_id IS NULL;
ALTER TABLE users ALTER COLUMN user_id SET NOT NULL;
ALTER TABLE users ADD CONSTRAINT fk_entity_user FOREIGN KEY (user_id) REFERENCES users(id);
