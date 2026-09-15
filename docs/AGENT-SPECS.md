# Agent specs

Contract: `analyze(ticker_or_scope, snapshots) -> AgentResult(values[], score, citations[])`.
Agents never fetch live; they read snapshots. Fixtures in `tests/fixtures/` prove each.

## A1 — Smart Money Tracker

- Reads: broker-summary/top, broker-activity/top, foreign-flow.
- Steps: (1) top buyers/sellers per ticker; (2) per-broker accumulation ranks;
  (3) foreign inflow 90d trend; (4) correlate broker net direction vs foreign
  direction; (5) classify phase: accumulation / distribution / neutral / conflict.
- Output: score −100..+100, phase, top-3 players with net Rp, direction-agreement flag.
- Fixture: BBCA 5d — 3 domestic brokers net-buy Rp 1.2T + foreign inflow → score > +60.

## A2 — Broker Intel

- Reads: brokers/ registry cache, broker-activity per code, brokers/top daily.
- Steps: (1) classify each active broker by origin/cohort; (2) behavior class per
  broker (accumulating/distributing/neutral from top/ ranks); (3) sector exposure
  shift week-over-week; (4) emit rotation signal on sign flip with evidence rows.
- Output: behavior map, rotation signal (from→to + net Rp delta).
- Fixture: Financials net −Rp 800M → Consumer +Rp 1.1B flip detected (sector_flow_idx.json).

## A3 — News Sentiment (Adaptive RAG)

- Reads: news (incremental), filings, suspensions.
- Steps: (1) fetch candidate articles; (2) LLM confidence check — confident →
  answer from context, uncertain (rare ticker) → force retrieval + ground;
  (3) per-article sentiment + confidence; (4) aggregate trend
  improving/deteriorating/stable; (5) insider summary from filings.
- Output: score −1..+1, trend, key events (≤5, cited), insider line.
- Offline path: when no news/filings/suspensions snapshots exist, the agent falls
  back to the derived `news_items` table (`NewsSince`) instead of failing.
- Fixture: ticker with 2 bullish + 1 neutral + 1 director buy → positive trend cited.

## A4 — Fundamental

- Reads: company/report (sections=overview,valuation,financials,dividend),
  financials/quarterly (n≤8), get-segments.
- Steps: (1) P/E, P/B vs subsector median (subsector/report statistics);
  (2) revenue/earnings 8Q trend; (3) ROE trajectory, debt/equity, payout ratio;
  (4) grade A–F from weighted rubric (profitability 35, growth 25, leverage 20, payout 20).
- Output: score 0–100, grade, vs-peers table, red flags.
- Fixture: BBCA — premium P/E vs banks median, declining ROE flagged.

## A5 — Technical

- Reads: daily (≤90d), most-traded, top-changes, free-float.
- Steps: (1) volume vs 20d avg multiples; (2) momentum positioning from movers;
  (3) relative volume vs market; (4) liquidity grade from free-float %.
- Output: momentum signal (strong/up/flat/down), anomaly flag with dates,
  liquidity grade.
- Fixture: 3.2× volume spike flagged with date + mover rank cited.

## A6 — Event Catalyst

- Reads: corporate-actions, quarterly-dates, listing-performance.
- Steps: (1) upcoming dividends/splits/AGM with dates; (2) next earnings estimate;
  (3) IPO-window context for recent listings; (4) score opportunity 0–100
  (yield × certainty − earnings-risk).
- Output: catalyst calendar rows (event, date, H−N, score).
- Fixture: ex-div in 23d with yield + payout flag rendered.

## A7 — Master Synthesizer

- Reads: A1–A6 outputs + risk profile + accuracy-ledger weights.
- Steps: (1) weight signals (conservative→fundamental-heavy, aggressive→
  technical+broker-heavy); (2) agreement bonus when ≥3 agents align, conflict
  flag when fundamentals oppose flows; (3) conviction 1–5 from weighted score
  spread; (4) position size via capped Kelly (max 10% single name);
  (5) thesis ≤5 sentences, each claim cited.
- Output: BUY/HOLD/AVOID, conviction, size %, thesis, conflict flags.
- Fixture: good fundamental + broker selling → HOLD-or-lower with conflict cited.
