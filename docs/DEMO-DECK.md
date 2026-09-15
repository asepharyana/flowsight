# FlowSight — Demo Deck (offline, seeded)

Narrative: briefing → radar alert → report → interrogation. All numbers cite
snapshots; stale chips render marked, never hidden.

## Slide 1 — Problem
6M+ retail SID decide from price + rumor. Institutional data (broker flow,
foreign flow, insider filings) is raw across 70 Sectors endpoints.

## Slide 2 — Autopilot routines
Subscribe once; agents run on schedule; results land without opening the app.
R1 Morning Briefing 07:30 · R2 Radar 30-min · R3 Reversal · R4 Insider Tape ·
R5 Earnings H-7/H-3/H-1 · R6 Dividend H-14/H-3 · R7 Weekend Review.

## Slide 3 — Live: Morning Briefing
`GET /api/briefing/today` — top-5 accumulation, foreign table, 7d agenda,
mover of the day. ≤25 lines, every row cited.

## Slide 4 — Live: Accumulation Radar
BBCA: 3 brokers net-buy Rp1.2T, volume 3.2× 20d avg → alert card with score,
top-3 brokers, volume multiple, `/report/BBCA` link.

## Slide 5 — Live: One-Click Report
`POST /api/report/BBCA` — 7 sections with `citations[]` per section; export
PDF/HTML/MD/JSON. Thesis claims carry inline `[endpoint @ date]` markers.

## Slide 6 — Verifiable AI + Accuracy Ledger
Every number cites endpoint + snapshot timestamp. Recommendations resolve at
+30d (`ResolveDue` daily cron); agent weights follow hit-rate, shown in
`/portfolio` accuracy table.

## Slide 7 — Ask
Run `BASE=http://localhost:8080 sh scripts/demo.sh` offline from seed.
