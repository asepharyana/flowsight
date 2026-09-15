-- FlowSight schema v7: local username+password login (Google stays optional).
-- Local accounts store google_sub='local:<username>' so session scoping
-- (sessions join + UserKeyForSub) works unchanged.
ALTER TABLE users ADD COLUMN username TEXT;
ALTER TABLE users ADD COLUMN password_hash TEXT NOT NULL DEFAULT '';
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username ON users(username);
