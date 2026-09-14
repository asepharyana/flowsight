# FlowSight — Sectors API v2 Reference (learned from schema.json + docs)

Source: `https://docs.sectors.app/schema.json` (OpenAPI, 70 paths) + llms.txt.
Base: `https://api.sectors.app/v2/`. Auth header: `Authorization: <raw-key>` (REST).
v1 discontinued 2026-05-11 — all `/v1/*` return 410. Use v2 only.

## Global constraints
- IDX symbol: 4 letters, optional `.jk`, case-insensitive (`BBCA`, `bbca.jk`).
- Broker codes: 2-letter exchange-member IDs (`MG`, `AK`, `CC`) — valid list from `GET /v2/brokers/`.
- Date windows: broker endpoints max 14 days; daily/foreign-flow/idx-total/most-traded max 90 days (clamped). Future `end` → 400.
- Pagination: `limit`/`offset` where listed. `GET /v2/close/` paginated per trading day.
- Credit traps (defaults are expensive — always narrow params):
  - `top-changes` default (2 class × 5 periods) = 10 credits → always set `classifications` + `periods`.
  - `company/report` default (all 8 sections) = 8 credits → always set `sections`.
  - `subsector/report` default (all 6 sections) = 6 credits → always set `sections`.
  - `financials/quarterly` = 1 credit per quarter → bound `n_quarters`.
  - Universe quarterly-dates full sweep ≈ 32 pages → poll incrementally with `since`.
  - `free-float` = 1 credit per 100 companies; filters mutually exclusive (one per request).
  - `news`: `extension=idx` vs `extension=mining` params mutually exclusive (400 if mixed).

## A. Screener & taxonomy (7)
| Method + path | Params | Cost | FlowSight use |
|---|---|---|---|
| GET `/v2/companies/` | `where`, `q`, `order_by`, `desc`, `limit`≤200, `offset`, `include_query_values` (`q` overrides all) | 1 (structured) | NL + SQL screener core |
| GET `/v2/free-float/` | one of `sector`/`sub_sector`/`industry`/`sub_industry` | 1/100 cos | Liquidity grade, sector sweep |
| GET `/v2/subsectors/` | — | 1 | Slug source (cache daily) |
| GET `/v2/industries/` | — | 1 | Slug source (cache daily) |
| GET `/v2/subindustries/` | — | 1 | Slug source (cache daily) |
| GET `/v2/tags/` | — | 1 | News/filing tag filter values (cache daily) |
| GET `/v2/companies/list_companies_with_segments/` | — | 1 | Check segment availability (cache weekly) |

## B. Company core (7)
| Method + path | Params | Cost | FlowSight use |
|---|---|---|---|
| GET `/v2/company/report/{symbol}/` (or `?symbol=`) | `sections` ∈ overview, valuation, future, peers, financials, dividend, management, ownership | 1/section | Fundamental agent; report sections |
| GET `/v2/company/get-segments/{symbol}/` | `financial_year` | 1 | Revenue breakdown (Sankey-ready) |
| GET `/v2/company/get_quarterly_financial_dates/{symbol}/` | — | 1 | Valid `report_date` values per ticker |
| GET `/v2/financials/quarterly/{symbol}/` | `report_date`, `approx`, `n_quarters` | 1/quarter | Earnings trend (banks add net_interest_income, gross_loan, total_deposit) |
| GET `/v2/company/corporate-actions/{symbol}/` | — | 1 | Splits/rights/warrants/AGM/dividends → Dividend Calendar, Event agent |
| GET `/v2/company/shareholders-composition/{symbol}/` | `year` (≥2021) | 1 | Local vs foreign holder mix (9 categories × _l/_f) |
| GET `/v2/listing-performance/{symbol}/` | — (post-May-2005 only) | 1 | IPO context (7/30/90/365d windows) |

## C. Universe polling (2) — cheap sweeps, no per-ticker loop
| Method + path | Params | Cost | FlowSight use |
|---|---|---|---|
| GET `/v2/close/` | `date` (default latest), `limit`, `offset` | 1/page | Full-universe close, one sweep per cycle |
| GET `/v2/companies/quarterly-financial-dates/` | `year`, `since`, `limit`≤30, `offset` | 1/page | Freshness polling: `since=` returns only newly-reported companies |

## D. Market & rankings (5)
| Method + path | Params | Cost | FlowSight use |
|---|---|---|---|
| GET `/v2/daily/{symbol}/` | `start`, `end` (≤90d) | 1 | Price+volume+MCap series per watchlist ticker |
| GET `/v2/idx-total/` | `start`, `end` (≤90d, ≥2021-01-01) | 1 | IHSG total MCap trend (macro context) |
| GET `/v2/index-daily/{index_code}/` | lq45, idx30, kompas100, ihsg, jii70… (≥2019-01-02) | 1 | Index benchmark for beta/correlation |
| GET `/v2/companies/top-changes/` | `classifications` top_gainers/top_losers, `periods` 1d/7d/14d/30d/365d, `sub_sector`, `n_stock`, `min_mcap_billion` | 1 per class×period | Momentum input (request minimal combos) |
| GET `/v2/most-traded/` | `start`, `end`, `sub_sector`, `n_stock`, `adjusted` | 2 | Relative volume leaders |

## E. Brokers — the moat (7)
| Method + path | Params | Cost | FlowSight use |
|---|---|---|---|
| GET `/v2/brokers/` | `cohort` (retail/mixed/institutional/unknown), `origin` (foreign/domestic) | 1 | Registry cache → classify every code seen |
| GET `/v2/brokers/top/` | `date`, `metric`, `n_brokers`, `origin`, `cohort` | 2 | Daily broker ranking → who is active today |
| GET `/v2/broker-activity/{broker_code}/` | `symbol`, `start`, `end` (≤14d) | 1 | All (stock,day) rows per broker |
| GET `/v2/broker-activity/{broker_code}/top/` | `start`, `end`, `n_brokers` | 2 | Top accumulations/distributions per broker |
| GET `/v2/broker-summary/{symbol}/` | `broker_code`, `start`, `end` (≤14d) | 1 | Per-broker daily rows per ticker (lots, freq, avg price) |
| GET `/v2/broker-summary/{symbol}/top/` | `start`, `end`, `cohort`, `origin`, `n_brokers` | 2 | Top buyers/sellers per ticker → accumulation rule |
| GET `/v2/foreign-flow/{symbol}/` | `start`, `end` (≤90d) | 1 | Net foreign inflow series → reversal + sentiment |

## F. News & events (3)
| Method + path | Params | Cost | FlowSight use |
|---|---|---|---|
| GET `/v2/news/` | `extension`=idx, `sector`, `sub_sector`, `tags`, `symbols`, `keyword`, `start`, `end` | 1 | Sentiment agent input (incremental via `since`-style start) |
| GET `/v2/filings/` | `symbol`, `sector`, `sub_sector`, `tags`, `transaction_type`, `holder_type`, `start`, `end` | 1 | Insider Tape routine |
| GET `/v2/suspensions/` | `symbol`, `start`, `end` | 1 | Suspension Watch (reason + IDX PDF link) |

## G. Subsector (2)
| Method + path | Params | Cost | FlowSight use |
|---|---|---|---|
| GET `/v2/subsector/report/{sub_sector}/` (or `?sub_sector=`) | kebab-case slug; `sections` ∈ statistics, market_cap, stability, valuation, growth, companies | 1/section | Sector rotation map, peer medians |

## H. SGX (9) — phase 2, regional extension
`sgx/companies/` (where/q), `sgx/companies/top/`, `sgx/company/report[/{symbol}]`,
`sgx/daily/{symbol}/`, `sgx/filings/`, `sgx/news/`, `sgx/buybacks/`, `sgx/short-sell/`,
`sgx/sectors/`, `sgx/subsectors/`, `sgx/tags/`. Symbols 3–4 chars, output carries `.SI`.
Killer angle: apply the same routine engine to SGX (short-sell + buybacks have no IDX equivalent).

## I. KLSE (4) — phase 2
`klse/sectors/`, `klse/companies/?sector=`, `klse/companies/top/`, `klse/company/report[/{symbol}]`.
Symbols are 4-digit codes (`1155`). Basic coverage only.

## J. Mining (19) — optional commodity vertical
Companies: list/detail/financials (USD millions)/ownership/performance by `slug`.
Trade: commodities list, price history (≤3y range), exports (Gold/Copper/Coal),
global-commodity, sales-destination by slug.
Sites: index + detail (lat/long), resources-reserves index + per-province detail,
total-production. Licenses: IUP/IUPK list, auctions + WIUP detail, contracts.
Angle: commodity-price → mining-stock linkage routine (coal/nickel price moves → watchlist miners).

## Credit budget (per 30-min cycle, W = 20 watchlist tickers)
| Step | Calls | Credits |
|---|---|---|
| close/ sweep | ~10 pages | ~10 |
| top-changes (1 class × 2 periods) | 1 | 2 |
| most-traded | 1 | 2 |
| idx-total | 1 | 1 |
| brokers/top | 1 | 2 |
| broker-summary/top + foreign-flow + daily per ticker | 3 × 20 | 80 |
| news + filings + suspensions (incremental) | 3 | 3 |
| quarterly-dates universe (`since=`) | ~2 pages | ~2 |
| **Total per cycle** | | **≈100** |
Morning briefing extra: report (2–3 sections × 5 tickers ≈ 10–15) + corporate-actions ×5 + quarterly (n=4 ×5 = 20) ≈ 35–40.
Rules: never full universe quarterly sweep without `since`; cache registry/taxonomy/tags daily; `sections` always explicit.
