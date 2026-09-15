-- FlowSight schema v2: holder attribution + user scoping for history reads.
ALTER TABLE filings ADD COLUMN holder_name TEXT NOT NULL DEFAULT '';
ALTER TABLE routine_runs ADD COLUMN user_key TEXT NOT NULL DEFAULT '';
ALTER TABLE alert_events ADD COLUMN user_key TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_routine_runs_user ON routine_runs(user_key, routine_id);
CREATE INDEX IF NOT EXISTS idx_alert_events_user ON alert_events(user_key, ticker);
UPDATE routine_runs SET user_key=(SELECT user_key FROM routines WHERE routines.id=routine_runs.routine_id) WHERE user_key='';
UPDATE alert_events SET user_key=(SELECT user_key FROM alerts WHERE alerts.id=alert_events.alert_id) WHERE user_key='';
