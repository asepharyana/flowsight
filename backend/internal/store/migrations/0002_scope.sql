-- FlowSight schema v2: holder attribution + user scoping for history reads.
-- Idempotent: safe to re-run on DBs created before schema_migrations existed.
ALTER TABLE filings ADD COLUMN holder_name TEXT NOT NULL DEFAULT '';
ALTER TABLE routine_runs ADD COLUMN user_key TEXT NOT NULL DEFAULT '';
ALTER TABLE alert_events ADD COLUMN user_key TEXT NOT NULL DEFAULT '';
