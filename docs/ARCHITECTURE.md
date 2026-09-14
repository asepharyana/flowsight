# Architecture

## Layout

```
flowsight/
  TODO.md / ROADMAP.md        # work tracking (shiro-neko style)
  docs/                       # specs (this folder) — change before code
  backend/                    # Go 1.23 module (see TECH-STACK.md)
    cmd/server/main.go        # entrypoint: HTTP server + scheduler in one process
    internal/
      config/                 # env (SECTORS_API_KEY), credit budget, schedules
      sectors/                # SectorsClient + endpoint packages per category
        client.go             # retry, credit counter, param narrowing defaults
        screener.go           # companies/, free-float/, taxonomy
        company.go            # report, segments, quarterly, actions, shareholders
        market.go             # close/, daily, idx-total, index-daily, movers
        brokers.go            # registry cache, activity, summary, foreign-flow
        events.go             # news, filings, suspensions
      store/
        db.go                 # database/sql connect + numbered migrations
        seed.go               # historical-replay fixture loader
        cache.go              # Redis wrapper + in-memory TTL fallback
      agents/                 # 7 specialists + synthesizer (see AGENT-SPECS.md)
        agent.go              # Agent contract: Analyze() -> AgentResult + citations
        smart_money.go broker_intel.go sentiment.go fundamental.go
        technical.go catalyst.go synthesizer.go
      routines/               # 7 routines (see ROUTINES.md)
        engine.go             # cron dispatch, run recording, delivery
        briefing.go radar.go reversal.go insider.go earnings.go dividend.go weekly.go
      alerts/
        rules.go              # 6 detection rules over snapshots
        evaluate.go           # per-cycle evaluation
        notify.go             # Telegram/Discord webhooks
      reports/
        builder.go            # 7-section assembly + citations[]
        render.go             # PDF/HTML/MD/JSON exporters
      api/                    # chi route handlers (see API.md)
        flow.go screen.go routines.go briefing.go alerts.go
        report.go watchlist.go portfolio.go accuracy.go chat.go health.go stream.go
      scheduler/              # robfig/cron wiring (ingestion + routines)
  web/                        # SolidJS 1.9 + Vite 6 + StyleX
    src/
      pages/                  # Dashboard, Routines, Alerts, Screener, Portfolio, Report
      components/             # cards, tables, rotation map, correlation matrix
      lib/api.ts              # typed backend client + SSE hooks
      styles/                 # StyleX tokens + themes
  tests/
    fixtures/                 # historical snapshots (one trading week)
    agents_*_test.go          # per-agent fixture tests
    rules_test.go report_test.go budget_test.go
```

## Conventions

- Spec-first: docs/ updated before code; a PR without a doc touch needs a reason.
- Every outbound Sectors call goes through `SectorsClient` (credit counted, sections
  explicit, classification combos minimal). No raw HTTP to the API elsewhere.
- Every number in user-visible output carries `{endpoint, snapshot_at}` citation.
  Builders that emit numbers without citations fail review.
- Tests: one test file per agent/rule + budget test asserting per-cycle credits ≤ cap
  on fixtures. `go vet` + `gofmt` and `tsc` + `vite build` before commit.

## Scheduler

- Ingestion cycle every 30 min, 09:00–16:00 WIB (market hours). Steps in order:
  reference cache check → universe sweep → market context → per-watchlist depth →
  incremental events → quarterly freshness → rule evaluation → routine dispatch.
- Routine schedules are cron exprs stored per routine row; engine records each run
  (started_at, status, payload) for the Routine Manager history view.

## Citation pipeline

1. Ingestion stores raw payload + `snapshot_at` in `snapshots`.
2. Agents/detectors read snapshots, emit values tagged with snapshot IDs.
3. Reports/alerts/briefings serialize `citations[]` alongside values.
4. Frontend renders citation chips (endpoint + time); stale (>1 session) chips are
   visually marked.

## SSE design

- `GET /api/stream` (EventSource): channels `agents` (status+scores during runs),
  `alerts` (new events), `activity` (feed rows). Heartbeat 15 s; reconnect resumes
  from last event ID. No WebSocket — one-directional push is all the UI needs.
