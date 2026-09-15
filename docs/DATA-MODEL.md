# Data model

SQLite for the hackathon; schema kept Postgres-compatible (serial → integer PK,
JSON → TEXT with JSON1, no SQLite-only DDL). Migrations numbered in
`backend/internal/store/migrations/`.

## Tables

- `snapshots(id, ticker, date, source, payload_json, fetched_at)` — raw API rows.
  Index (ticker, date, source). Retention: 180d, then compact to weekly.
- `broker_activity(broker_code, ticker, date, buy, sell, net, lots, freq, avg_price)`
  Index (ticker, date), (broker_code, date).
- `foreign_flow(ticker, date, net_inflow)` — PK (ticker, date).
- `news_items(id, ticker, date, source, sentiment, confidence, url, title)` —
  Index (ticker, date).
- `filings(id, ticker, date, holder_type, txn_type, volume, price)` — Index (ticker, date).
- `routines(id, user_key, type, schedule_cron, channels_json, enabled)` — 7 types (R1–R7).
- `routine_runs(id, routine_id, started_at, status, payload_json, credits_used)`.
- `alerts(id, user_key, name, rule_json, channels_json, last_fired)`.
- `notification_destinations(id, user_key, kind, label, bot_token, chat_id, webhook_url, enabled, created_at)` — per-user push targets. `kind` telegram needs bot_token+chat_id, discord needs webhook_url (https). Secrets never leave the server in API responses.
- `alert_events(id, alert_id, ticker, date, message, context_json, citations_json)`.
- `watchlists(user_key, ticker, added_at)` — PK (user_key, ticker).
- `reports(id, ticker, generated_at, payload_json, citations_json)`.
- `agent_accuracy(id, agent, ticker, prediction, predict_date, resolved, hit, actual_return)`.
- `briefings(date, payload_json, citations_json)` — PK date.
- `credit_ledger(date, endpoint, calls, credits)` — daily spend audit.

## Seed strategy

`seed.py` loads one historical trading week into snapshots + derived tables so the
full demo (briefing → radar → report → interrogation) runs offline. Fixtures live in
`tests/fixtures/` as JSON exports of real API shapes (field names match schema.json).

## Cursors

- `meta(key, value)`: `quarterly_since` (universe poll cursor), `news_since`,
  `filings_since` — persisted so restarts resume incrementally.
