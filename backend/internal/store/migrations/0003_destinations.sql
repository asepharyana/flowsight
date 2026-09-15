-- FlowSight schema v3: per-user notification destinations (CRUD, dynamic per login).
-- kind: telegram (bot_token+chat_id) or discord (webhook_url). Secrets live
-- server-side only; API responses never include them.
CREATE TABLE IF NOT EXISTS notification_destinations(
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_key TEXT NOT NULL,
  kind TEXT NOT NULL,
  label TEXT NOT NULL DEFAULT '',
  bot_token TEXT NOT NULL DEFAULT '',
  chat_id TEXT NOT NULL DEFAULT '',
  webhook_url TEXT NOT NULL DEFAULT '',
  enabled INTEGER NOT NULL DEFAULT 1,
  created_at TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_destinations_user ON notification_destinations(user_key, enabled);
