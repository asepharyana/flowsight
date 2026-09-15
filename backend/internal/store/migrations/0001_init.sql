-- FlowSight schema v1. SQLite for hackathon; kept Postgres-compatible
-- (integer PKs, TEXT JSON payloads, no SQLite-only DDL).
CREATE TABLE IF NOT EXISTS snapshots(
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  ticker TEXT NOT NULL, date TEXT NOT NULL, source TEXT NOT NULL,
  payload_json TEXT NOT NULL, fetched_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_snapshots_ticker_date_source ON snapshots(ticker, date, source);
CREATE TABLE IF NOT EXISTS broker_activity(
  broker_code TEXT NOT NULL, ticker TEXT NOT NULL, date TEXT NOT NULL,
  buy REAL NOT NULL DEFAULT 0, sell REAL NOT NULL DEFAULT 0, net REAL NOT NULL DEFAULT 0,
  lots REAL NOT NULL DEFAULT 0, freq INTEGER NOT NULL DEFAULT 0, avg_price REAL NOT NULL DEFAULT 0,
  PRIMARY KEY(broker_code, ticker, date)
);
CREATE INDEX IF NOT EXISTS idx_broker_activity_ticker_date ON broker_activity(ticker, date);
CREATE INDEX IF NOT EXISTS idx_broker_activity_broker_date ON broker_activity(broker_code, date);
CREATE TABLE IF NOT EXISTS foreign_flow(
  ticker TEXT NOT NULL, date TEXT NOT NULL, net_inflow REAL NOT NULL DEFAULT 0,
  PRIMARY KEY(ticker, date)
);
CREATE TABLE IF NOT EXISTS news_items(
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  ticker TEXT NOT NULL, date TEXT NOT NULL, source TEXT NOT NULL DEFAULT '',
  sentiment TEXT NOT NULL DEFAULT 'neutral', confidence REAL NOT NULL DEFAULT 0,
  url TEXT NOT NULL DEFAULT '', title TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_news_ticker_date ON news_items(ticker, date);
CREATE TABLE IF NOT EXISTS filings(
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  ticker TEXT NOT NULL, date TEXT NOT NULL, holder_type TEXT NOT NULL DEFAULT '',
  txn_type TEXT NOT NULL DEFAULT '', volume REAL NOT NULL DEFAULT 0, price REAL NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_filings_ticker_date ON filings(ticker, date);
CREATE TABLE IF NOT EXISTS routines(
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_key TEXT NOT NULL, type TEXT NOT NULL, schedule_cron TEXT NOT NULL DEFAULT '',
  channels_json TEXT NOT NULL DEFAULT '[]', enabled INTEGER NOT NULL DEFAULT 1
);
CREATE TABLE IF NOT EXISTS routine_runs(
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  routine_id INTEGER NOT NULL, started_at TEXT NOT NULL, status TEXT NOT NULL DEFAULT 'ok',
  payload_json TEXT NOT NULL DEFAULT '{}', credits_used INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_routine_runs_routine ON routine_runs(routine_id, started_at);
CREATE TABLE IF NOT EXISTS alerts(
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_key TEXT NOT NULL, name TEXT NOT NULL, rule_json TEXT NOT NULL,
  channels_json TEXT NOT NULL DEFAULT '[]', last_fired TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS alert_events(
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  alert_id INTEGER NOT NULL, ticker TEXT NOT NULL, date TEXT NOT NULL,
  message TEXT NOT NULL, context_json TEXT NOT NULL DEFAULT '{}', citations_json TEXT NOT NULL DEFAULT '[]'
);
CREATE INDEX IF NOT EXISTS idx_alert_events_ticker_date ON alert_events(ticker, date);
CREATE TABLE IF NOT EXISTS watchlists(
  user_key TEXT NOT NULL, ticker TEXT NOT NULL, added_at TEXT NOT NULL,
  PRIMARY KEY(user_key, ticker)
);
CREATE TABLE IF NOT EXISTS reports(
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  ticker TEXT NOT NULL, generated_at TEXT NOT NULL,
  payload_json TEXT NOT NULL, citations_json TEXT NOT NULL DEFAULT '[]'
);
CREATE TABLE IF NOT EXISTS agent_accuracy(
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  agent TEXT NOT NULL, ticker TEXT NOT NULL, prediction TEXT NOT NULL,
  predict_date TEXT NOT NULL, resolved INTEGER NOT NULL DEFAULT 0,
  hit INTEGER NOT NULL DEFAULT 0, actual_return REAL NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_accuracy_agent ON agent_accuracy(agent, predict_date);
CREATE TABLE IF NOT EXISTS briefings(
  date TEXT PRIMARY KEY, payload_json TEXT NOT NULL, citations_json TEXT NOT NULL DEFAULT '[]'
);
CREATE TABLE IF NOT EXISTS credit_ledger(
  date TEXT NOT NULL, endpoint TEXT NOT NULL, calls INTEGER NOT NULL DEFAULT 0,
  credits INTEGER NOT NULL DEFAULT 0, PRIMARY KEY(date, endpoint)
);
CREATE TABLE IF NOT EXISTS meta(key TEXT PRIMARY KEY, value TEXT NOT NULL DEFAULT '');
