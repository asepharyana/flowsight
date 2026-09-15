# Backend API

Base `/api`. Demo auth: `X-User-Key` header (single demo key for hackathon).
Errors: `{error: {code, message}}` with HTTP 400/404/422/502 (502 = upstream Sectors).

## Flow

- `GET /api/flow/summary?date=` → foreign net total, top-5 accumulation rows,
  rotation signal, mover of day. Each value with citations.
- `GET /api/flow/broker?ticker=&start=&end=` → buyers/sellers + 5d net series.
- `GET /api/flow/foreign?ticker=&start=&end=` → inflow series + reversal flag.
- `GET /api/stream` → SSE (channels: agents, alerts, activity; heartbeat 15 s).

## Screen

- `POST /api/screen` body `{where?, q?, institutional?: {broker_score_min, foreign_trend, insider_buying, volume_anomaly}, limit?}` → ranked rows
  `{symbol, name, composite, breakdown: {broker, foreign, insider, fundamental}, citations}`.

## Routines & briefing

- `GET /api/routines` → list with enabled + last run status.
- `POST /api/routines` body `{type, schedule_cron?, channels[]}` → created.
- `PATCH /api/routines/:id` body `{enabled?, schedule_cron?, channels?}`.
- `GET /api/routine-runs?routine_id=&limit=` → run history.
- `GET /api/briefing/today` → latest briefing payload + citations.

## Alerts

- `GET /api/alerts`, `POST /api/alerts` body `{name, rule, channels[]}`,
  `DELETE /api/alerts/:id`, `GET /api/alert-events?since=&ticker=`.

## Destinations (push targets per user)

- `GET /api/destinations` → `[{id, kind, label, enabled, configured}]`. Secrets are never returned.
- `POST /api/destinations` body `{kind: telegram|discord, label?, bot_token?, chat_id?, webhook_url?}` → created (422 on wrong kind or missing secret).
- `PATCH /api/destinations/:id` body `{label?, enabled?, bot_token?, chat_id?, webhook_url?}` → patched (kind immutable; omitted secret = keep).
- `DELETE /api/destinations/:id` → removed.

Delivery: routines and alerts push to the owner's **enabled** destinations. When the owner has none, the server-level env webhook (`TELEGRAM_*` / `DISCORD_*`) is used as fallback; when neither exists delivery is record-only.

## Report

- `POST /api/report/:ticker` query `?format=json|html|pdf|md&profile=moderate|conservative|aggressive`
  → 7-section payload with `citations[]` per section. PDF rendered server-side.
- `POST /api/report/:ticker/ask` body `{question, report_id?}` → follow-up answer
  grounded ONLY in that report's citations (explicit id or latest for the ticker;
  cross-ticker ids are rejected).

## Watchlist / portfolio / accuracy / chat / health

- `GET /api/watchlist`, `POST /api/watchlist` `{ticker}`, `DELETE /api/watchlist/:ticker`.
- `GET /api/portfolio/risk` → concentration[], correlation[][], beta, warnings[].
- `GET /api/accuracy` → per-agent `{calls, resolved, hits, hit_rate}`.
- `POST /api/chat` body `{message, scope?: {report_id}}` → cited answer (report scope
  restricts grounding to that report's citations).
- `GET /api/health` (`?force=1` runs one synchronous probe cycle) →
  `{last_cycle_at, credits_today, scheduler_ok, stale_flags}`.
