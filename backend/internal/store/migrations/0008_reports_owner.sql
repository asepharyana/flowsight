-- 0008_reports_owner.sql: reports now scoped to the owning user.
-- Existing rows are assigned to the shared demo key so historic reports
-- remain readable by the demo/seed pipeline; new reports carry user_key.
CREATE TABLE IF NOT EXISTS reports_new(
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  ticker TEXT NOT NULL, generated_at TEXT NOT NULL,
  payload_json TEXT NOT NULL, citations_json TEXT NOT NULL DEFAULT '[]',
  user_key TEXT NOT NULL DEFAULT 'demo'
);
INSERT INTO reports_new(id, ticker, generated_at, payload_json, citations_json, user_key)
  SELECT id, ticker, generated_at, payload_json, citations_json, 'demo' FROM reports;
DROP TABLE reports;
ALTER TABLE reports_new RENAME TO reports;
CREATE INDEX IF NOT EXISTS idx_reports_ticker ON reports(ticker, id);

-- SQLite ignores IF NOT EXISTS on column add; guard on table existence.