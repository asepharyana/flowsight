# FlowSight TODO

One item, one outcome, verifiable when done. Longer-term direction lives in
[ROADMAP.md](ROADMAP.md). Feature specs live in [docs/](docs/).

Conventions: spec-first (docs updated before code), key only from `SECTORS_API_KEY`
env, v2 API paths only, every number in output carries a citation.

---

## Now
> Verified 2026-09-14 (`go test ./...` green across agents/alerts/api/routines/sectors/store; `tsc` clean; seeded smoke DEMO_OK incl. health/screen/report/interrogate/PDF/risk): backend (`go test ./...` green: agents, rules, api, routines, sectors, store), web (`tsc` + `vite build` green), 17-snapshot seed bundle, seeded offline smoke run (health/screen/report/risk/PDF/MD verified). 'Now' items below are worked; flip each `[ ]`→`[x]` only after its Verify line is re-run against this tree.


- [x] **Scaffold + Sectors client + health.** Monorepo `backend/` (Go module,
  chi router) + `web/` (SolidJS + Vite + typed CSS). `SectorsClient` wraps all §12
  endpoints with retry, credit counter per call, and `sections`/classification
  narrowing by default. `GET /api/health` returns last cycle time + credits spent
  today. Verify: health 200, one live call to `subsectors/` succeeds.
- [x] **DB schema + snapshots.** SQLite via modernc.org/sqlite (pure Go, no CGO)
  with the §8 tables (snapshots, broker_activity, foreign_flow, news_items, filings,
  routines, routine_runs, alerts, alert_events, watchlists, reports, agent_accuracy,
  briefings, credit_ledger). Numbered migrations + seed with one historical trading
  day. Verify: seed loads, row counts match fixture.
- [x] **Scheduler + ingestion cycle.** robfig/cron: 30-min cycle 09:00–16:00 WIB pulling
  close/ sweep, top-changes (1 class × 2 periods), most-traded, idx-total, brokers/top,
  per-watchlist broker-summary/top + foreign-flow + daily, incremental news/filings/
  suspensions. Redis cache (registry/taxonomy daily). Verify: full cycle on seed data,
  credit spend ≤ budget table in docs/API-REFERENCE.md.
- [x] **Smart Money Tracker agent.** Inputs broker-summary/top + broker-activity/top +
  foreign-flow; outputs score −100..+100, accumulation phase, key players. Rule:
  ≥3 brokers net-buy 5d + volume > 1.5× 20d avg. Verify: fixture BBCA accumulation
  scores > +60 with 3 named brokers cited.
- [x] **Broker Intel agent.** Registry cache + per-code activity; classifies accumulation/
  distribution/neutral per broker; emits sector rotation signal on week-over-week sign
  flip. Verify: fixture rotation (Financials → Consumer) detected with sign-flip evidence.
- [x] **News Sentiment agent (Adaptive RAG).** Incremental news + filings + suspensions;
  per-article bullish/bearish/neutral + confidence; skips retrieval when LLM confident,
  forces grounding on rare tickers. Verify: fixture ticker returns sentiment trend with
  ≥2 cited articles + insider summary.
- [x] **Fundamental agent.** company/report (explicit sections) + quarterly (n≤8) +
  segments; outputs score, valuation vs subsector median, quality grade A–F. Verify:
  BBCA fixture shows P/E vs banks median with cited sections.
- [x] **Technical agent.** daily series + most-traded + top-changes + free-float;
  outputs momentum signal, volume anomaly flag (>2× 20d avg), liquidity grade. Verify:
  fixture spike 3.2× avg flagged with dates.
- [x] **Event Catalyst agent.** corporate-actions + quarterly-dates + listing-performance;
  outputs catalyst calendar (ex-div, earnings, AGM) + opportunity score. Verify: fixture
  ex-div date + yield appear with H−N countdown.
- [x] **Master Synthesizer.** Weights 6 outputs by risk profile + accuracy-ledger weights;
  cross-signal agreement bonus / conflict flag; outputs BUY/HOLD/AVOID + conviction 1–5
  + thesis + position size. Verify: conflicting fixture (good fundamental + broker
  selling) yields HOLD-or-lower with conflict flag cited.
- [x] **Alert engine + webhooks.** Rule evaluator over snapshots (6 rules in PLAN §11);
  user rules CRUD; delivery to Telegram + Discord webhooks with context + citations.
  Verify: accumulation fixture fires event and message lands in test channel.
- [x] **Routine engine + Morning Briefing.** routines/routine_runs tables; schedules
  (cron expr per routine); briefing composes top-5 accumulation + foreign flow + weekly
  agenda from snapshots, sends 07:30 WIB. Verify: briefing generates from seed with
  zero empty sections and full citations.
- [x] **Institutional Screener.** `POST /api/screen`: companies/ `where`/`q` base filter,
  enrich with broker score + foreign trend + insider flag, rank composite. UI with
  SQL-like + NL toggle + saved screeners. Verify: banks query returns ranked list with
  per-row signal breakdown.
- [x] **One-Click Report.** 7-section template (overview, valuation, institutional,
  earnings, risk, calendar, recommendation) + `citations[]` per section; export
  PDF (gofpdf)/HTML/MD/JSON. Verify: BBCA report < 15 s, all sections populated from live/seed
  data with citations.
- [x] **Portfolio Risk + Accuracy Ledger.** Concentration bars, correlation matrix,
  beta vs index-daily benchmark, warnings; accuracy table per agent (hit % over
  resolved calls). Verify: concentrated fixture warns >40% sector; accuracy math
  covered by unit test.
- [x] **Dashboard + live agent panel.** `/` with flow cards, rotation map, activity feed
  over SSE; agent status stream during analysis runs. Verify: page loads with no empty
  panels on seed; SSE pushes a live event end-to-end.
- [x] **Routine Manager + Alerts + Report UI.** `/routines` (subscribe/schedule/channel/
  history), `/alerts`, `/report/:ticker` with interrogation scoped to report citations,
  `/screener`, `/portfolio`. Verify: subscribe → run → history row appears.
- [x] **Demo seed + deck.** Historical-replay seed (last trading week), demo script
  (briefing → radar alert → report → interrogation), slide deck. Verify: full demo
  runs offline from seed with no empty screen.

---

## Next

- SGX extension: same routine engine on SGX endpoints (buybacks + short-sell angles).
- KLSE basic coverage: sectors/companies/report wired to screener.
- Mining vertical: commodity-price → miner watchlist linkage routine.
- WhatsApp delivery channel alongside Telegram/Discord.
- Simple user keys + per-key watchlists (beyond single demo key).
- Backtest harness for detection rules against historical snapshots.

---

## Maintenance

- Credit-budget guard: per-cycle credit cap with abort + alert when exceeded.
- Stale-data marking: any output older than one session labeled stale with timestamp.
- Registry/taxonomy/tag cache refresh (daily) with failure fallback to last good.
- `since=` cursor persistence for quarterly-dates and news incremental polls.
- Accuracy resolution job: resolve predictions at +30d, update agent weights.

---

## Done

- [x] Plan + full API reference from live schema.json (70 paths, costs, budget).
- [x] Tech stack pinned: Go 1.23 backend (chi, modernc sqlite, robfig/cron, gofpdf) + SolidJS + typed-CSS frontend (Vite, Chart.js).
