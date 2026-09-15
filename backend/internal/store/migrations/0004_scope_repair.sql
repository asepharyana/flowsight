-- FlowSight schema v4: repair marker for DBs poisoned by the pre-journal
-- 0002 run (v2 columns applied but never recorded in schema_migrations).
-- Backfills user scoping data dropped from 0002 when it was made idempotent.
CREATE INDEX IF NOT EXISTS idx_routine_runs_user ON routine_runs(user_key, routine_id);
CREATE INDEX IF NOT EXISTS idx_alert_events_user ON alert_events(user_key, ticker);
UPDATE routine_runs SET user_key=(SELECT user_key FROM routines WHERE routines.id=routine_runs.routine_id) WHERE user_key='';
UPDATE alert_events SET user_key=(SELECT user_key FROM alerts WHERE alerts.id=alert_events.alert_id) WHERE user_key='';
