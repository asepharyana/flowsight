# Routines

Each routine: schedule, inputs (Sectors endpoints), detection logic, delivery format.
All routines read snapshots (never live-fetch inside delivery), attach citations,
and record a `routine_runs` row.

## R1 — Morning Briefing (07:30 WIB daily)

- Inputs: broker-summary/top + foreign-flow (yesterday), corporate-actions (week
  ahead), quarterly-dates universe (`since=` 7d), top-changes (1d).
- Logic: top-5 accumulation by net-buy sum; foreign net per watchlist ticker;
  earnings + ex-div agenda next 7d; biggest 1d mover with one-line cause (news match).
- Delivery: one Telegram/Discord message, ≤25 lines: header date, 5 accumulation
  rows (ticker, net Rp, #brokers), foreign table, agenda list, mover of the day.

## R2 — Accumulation Radar (every 30 min, 09:00–16:00 WIB)

- Inputs: broker-summary/top + broker-activity/top per active broker + daily volume.
- Logic: rule 1 (≥3 brokers net-buy 5d + volume > 1.5× 20d avg). First-fire only
  per (ticker, 5d window); re-fire requires net-buy sum growth > 25%.
- Delivery: alert card — ticker, score, top-3 brokers with net values, volume
  multiple, link to `/report/:ticker`.

## R3 — Foreign Reversal Watch (every 30 min)

- Inputs: foreign-flow per watchlist ticker (rolling 6d).
- Logic: rule 2 (5d cumulative outflow then 1d inflow, or reverse). Threshold:
  1d flow magnitude > 2× trailing 5d daily average.
- Delivery: alert card — direction flip, amounts, 6d mini-series, context line
  (e.g. "first inflow after 5 selling days").

## R4 — Insider Tape (every 30 min)

- Inputs: filings/ incremental (transaction_type=buy, holder director/major).
- Logic: rule 3 (buy volume > 2× 30d avg for that ticker, or ≥3 distinct insiders
  in 7d). Watchlist tickers only for push; others land in dashboard feed.
- Delivery: alert card — who (holder type), volume, price if present, vs-average
  multiple, filing date.

## R5 — Earnings Countdown (daily 08:00; fires at H−7, H−3, H−1)

- Inputs: company quarterly-dates per watchlist ticker + financials/quarterly (n≤8).
- Logic: next expected report ≈ last report + ~90d (refined when universe
  quarterly-dates shows a new date). Attach 8-quarter revenue/earnings mini-trend.
- Delivery: countdown card with trend summary + link to full quarterly table.

## R6 — Dividend Calendar (daily 08:00; fires at H−14, H−3)

- Inputs: corporate-actions per watchlist ticker (upcoming + historical dividends).
- Logic: ex-date within window; projected yield from last close; payout-ratio check
  from report dividend section (flag > 80% as aggressive).
- Delivery: calendar card — ex-date, DPS, est. yield, payout flag, history sparkline.

## R7 — Weekend Review (Saturday 09:00)

- Inputs: week snapshots (daily closes, flows, news, filings, routine run history).
- Logic: week movers per watchlist position, what drove them (top cited event each),
  open risks (conflict flags, concentration), next-week agenda (earnings/ex-div).
- Delivery: longer digest (report-lite) + archived to `briefings`.
