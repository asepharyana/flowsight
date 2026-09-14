# FlowSight Roadmap

What is built, what is next, and what has been deliberately declined. Reordered when
evidence says the order is wrong.

Nothing here is a date. Items move to [TODO.md](TODO.md) when they are next up.

---

## Shipped

- **Plan + API reference from live schema.** docs/PLAN.md (concept, routines,
  verifiable AI, accuracy ledger) + docs/API-REFERENCE.md (all 70 v2 paths with
  params, costs, per-cycle credit budget). Source: schema.json + docs site.

---

## Next

### Hackathon core (maps to TODO Now, in order)

**Foundation — data in, health out.** Scaffold (FastAPI + Next.js), Sectors client
with credit counting + param narrowing, SQLite schema (§8 tables), historical seed.
Health endpoint proves the pipeline breathes.

**Ingestion — the 30-minute heartbeat.** Scheduler pulls universe sweep (close/),
market context (top-changes minimal combos, most-traded, idx-total, brokers/top),
per-watchlist depth (broker-summary/top, foreign-flow, daily), incremental events
(news/filings/suspensions), quarterly freshness (`since=`). Redis caches slow-moving
reference data. Credit spend stays inside the §12 budget.

**Agents — seven specialists, one synthesis.** Smart Money Tracker and Broker Intel
are built first (the moat: no competitor fuses broker × foreign flow). Sentiment
(Adaptive RAG), Fundamental, Technical, and Event Catalyst follow the same
input→detect→score contract. Master Synthesizer correlates cross-signal agreement
into conviction, flags conflicts, and sizes positions by risk profile.

**Routines — data becomes habit.** Routine engine with cron schedules turns agent
output into deliveries: Morning Briefing (07:30), Accumulation Radar (30-min),
Foreign Reversal Watch, Insider Tape, Earnings Countdown, Dividend Calendar,
Weekend Review. Each run is recorded; each delivery carries citations.

**Trust — verify, then track.** Every number cites endpoint + snapshot timestamp;
stale data is labeled, never hidden. Accuracy Ledger records each recommendation
and resolves it at +30d against actual return; agent weights follow the ledger.

**Surface — see it, run it, export it.** Dashboard with live agent panel (SSE),
Routine Manager, Institutional Screener, Alert Engine UI, One-Click Report with
interrogation scoped to citations, Portfolio Risk + accuracy table. Demo seed +
script + deck close the loop for judges.

### Post-hackathon

- SGX extension (buybacks + short-sell have no IDX equivalent — new angles).
- KLSE basic coverage through the same screener.
- Mining vertical: commodity price → miner watchlist linkage.
- WhatsApp delivery; per-user keys and watchlists; backtest harness for rules.

---

## Later

- **Push-first mobile.** Native-feel PWA with push for radar alerts; web stays primary.
- **Community routines.** Shareable routine templates (e.g. "dividend hunter",
  "foreign follower") with fork counts.
- **Multi-market synthesis.** One briefing spanning IDX + SGX + KLSE positions.
- **LLM cost control.** Spend ceiling per routine run; cheaper model for sentiment
  triage, strong model only for synthesis.

---

## Declined

- **Order execution / trading.** Read-only intelligence; executing trades adds
  regulatory surface no hackathon needs.
- **Price prediction models.** Directional forecasting competes on accuracy claims no
  48h build can defend; the ledger tracks recommendations, not price targets.
- **Real-time tick streaming.** Sectors data is end-of-day granularity; pretending
  otherwise would fake the product. 30-min cycles match the source.
- **A general chatbot.** Conversational UI exists as a scoped sidebar only; the
  product is routines that run without being asked.
- **v1 API support.** v1 returns 410 Gone; no compat layer will be built.
