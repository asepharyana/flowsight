# FlowSight — Track 02: Automation & Workflows

**One-liner:** FlowSight is for Indonesian retail investors who can't monitor the market all day — it automates institutional money-flow tracking and pushes actionable alerts so they never miss what big players are doing.

## 1. Problem

6M+ retail SID di Indonesia mengambil keputusan dari harga dan rumor. Data yang dipakai
institusi — arus broker, foreign flow, insider filings — tersedia lewat Sectors API tapi
mentah dan tercecer di 70 endpoint. Tidak ada retail tool yang mengubahnya menjadi
rutinitas otomatis: setiap hari investor harus buka app, tarik data manual, dan
interpret sendiri. Produk existing (StockPilot, Invezgo, Stockbit) semuanya on-demand:
user bertanya, AI menjawab, selesai. Tidak ada yang bekerja saat user tidur.

## 2. Concept: Autopilot Routines

FlowSight bukan tool yang ditanya — ia rutinitas yang berjalan sendiri. User berlangganan
routine sekali, agent mengeksekusinya sesuai jadwal, hasilnya tiba di Telegram/Discord
tanpa user membuka app.

### Routine bawaan (v1)
1. **Morning Briefing (07:30 WIB)** — top 5 akumulasi semalam, foreign flow kemarin,
   agenda earnings & ex-div minggu ini. Satu digest, langsung kirim.
2. **Accumulation Radar (tiap 30 mnt, 09:00–16:00)** — deteksi ≥3 broker net-buy +
   volume anomali; temuan langsung jadi alert dengan konteks (siapa, berapa, sejak kapan).
3. **Foreign Reversal Watch** — outflow 5 hari berbalik inflow: sinyal pembalikan yang
   hampir tidak pernah terpantau manual.
4. **Insider Tape** — setiap ada director/major-holder buy di watchlist, user tahu
   hari yang sama beserta volumenya vs rata-rata 30 hari.
5. **Earnings Countdown** — H-7, H-3, H-1 sebelum laporan kuartalan ticker watchlist,
   lengkap dengan ekspektasi dari tren 8 kuartal terakhir.
6. **Dividend Calendar** — ex-date mendekat + yield proyeksi + histori payout, otomatis
   dari corporate-actions.
7. **Weekend Review (Sabtu 09:00)** — ringkasan mingguan portofolio: apa yang bergerak,
   kenapa, dan apa yang perlu perhatian minggu depan.

### Kenapa ini unik
- Kompetitor menunggu ditanya. FlowSight bekerja tanpa ditanya.
- Setiap routine = pipeline nyata (ingest → detect → synthesize → deliver), bukan
  satu LLM call. Inilah inti Track 02: data Sectors hidup di dalam rutinitas berulang.
- User membangun kebiasaan lewat produk, bukan lewat usaha: buka Telegram pagi,
  briefing sudah ada.

## 3. Verifiable AI (differentiator kedua)

Setiap angka di setiap output menempel ke sumbernya: endpoint Sectors + timestamp snapshot.
Contoh: "Foreign inflow Rp 340M/hari (foreign-flow/BBCA, snapshot 12 Sep 16:00 WIB)".
User bisa klik dan memverifikasi. Tidak ada klaim tanpa jejak. Ini menjawab masalah
terbesar AI finansial: halusinasi angka yang terdengar meyakinkan.

- Report menyimpan `citations[]`: setiap section menunjuk ke snapshot ID.
- Alert menyertakan data mentah ringkas + link ke dashboard detail.
- Jika data basi (>1 sesi), output menandainya eksplisit sebagai stale.

## 4. Accuracy Ledger (differentiator ketiga)

Setiap rekomendasi BUY/HOLD/AVOID dicatat dengan tanggal, lalu dievaluasi 30 hari
kemudian terhadap actual return. Hasilnya tampil publik per agent di dashboard:
"Smart Money Tracker: 68% tepat (47/69 calls)". Bobot agent di synthesizer mengikuti
rekam jejak, bukan asumsi. Tidak ada kompetitor IDX yang membuka track record
modelnya sendiri.

## 5. Agent System

7 specialist agents, dieksekusi paralel via goroutine, diorkestrasi scheduler + on-demand.

| Agent | Input (Sectors API) | Output |
|---|---|---|
| Smart Money Tracker | broker-summary-top, broker-activity-top, foreign-flow | Skor -100..+100, fase akumulasi, pemain kunci |
| Broker Intel | broker-registry, broker-activity-by-code, brokers/top | Klasifikasi perilaku broker, sinyal rotasi sektor |
| News Sentiment (Adaptive RAG) | news, filings, suspensions | Skor sentimen + tren, ringkasan insider, event kunci |
| Fundamental | company/report, quarterly-financials, segments | Skor fundamental, valuasi vs peers, grade A–F |
| Technical | daily-transaction, most-traded, top-changes, free-float | Sinyal momentum, anomali volume, grade likuiditas |
| Event Catalyst | corporate-actions, quarterly-dates, IPO performance | Kalender katalis, skor peluang event |
| Master Synthesizer | 6 output + risk profile + bobot accuracy | BUY/HOLD/AVOID + conviction 1–5 + tesis + sizing |

### Agent features
1. **Watchtower mode** — agents jalan tiap 30 menit saat market hours; temuan penting
   langsung jadi alert.
2. **Cross-signal correlation** — confidence naik saat sinyal selaras (fundamental
   bullish + akumulasi + sentimen naik); conflict flag saat bertentangan (fundamental
   bagus tapi broker jualan).
3. **Report interrogation** — tiap report bisa ditanya follow-up ("kenapa conviction
   cuma 3?"), jawaban grounding ke data report itu.
4. **Natural-language screener** — parameter `q=` Sectors + filter institusional
   (broker score, foreign trend, insider buying) yang di-compute sendiri.
5. **Live agent panel** — SSE stream status 7 agents + skor real-time di dashboard.

## 6. Platform Features
1. **Smart Money Dashboard** — foreign net flow, tabel akumulasi broker, peta rotasi
   sektor, activity feed real-time.
2. **Routine Manager** — subscribe/unsubscribe routine, atur jadwal + kanal notifikasi
   per routine, riwayat eksekusi.
3. **Institutional Screener** — query builder SQL-like + NL toggle, saved screeners,
   hasil berperingkat + breakdown sinyal.
4. **Alert Engine** — user rules + auto alert → webhook Telegram/Discord.
5. **One-Click Report** — research report 7 section, export PDF/HTML/MD/JSON,
   lengkap dengan citations.
6. **Portfolio Risk** — konsentrasi sektor, matriks korelasi, beta vs IHSG.
7. **AI Chat sidebar** — context-aware dari watchlist.

## 7. Architecture (authoritative: docs/ARCHITECTURE.md; stack: docs/TECH-STACK.md)
- Frontend: SolidJS 1.9 + Vite 6 + TypeScript + typed CSS + Chart.js (SSE streaming, responsive)
- Backend: Go 1.23 + chi v5 + database/sql (modernc.org/sqlite, pure Go) + goroutine
  (eksekusi agent paralel)
- LLM: Plain HTTPS ke endpoint OpenAI-compatible (`LLM_BASE_URL`); `LLM_MODEL_TRIAGE` murah + `LLM_MODEL_SYNTH` kuat
- Data: Sectors API v2 `https://api.sectors.app/v2/`, auth `Authorization: <key>`
  dari env `SECTORS_API_KEY`
- Store: SQLite (stdlib, skema Postgres-compatible) + Redis 7 cache (degradasi in-memory jika kosong)
- Scheduler: robfig/cron v3 in-process, ingestion tiap 30 min saat market hours + routine harian/mingguan
- Notify: outbound webhook → Telegram / Discord
- PDF: gofpdf (pure Go, tanpa system deps); test: `go test` + `httptest` (mock upstream Sectors); gate: `gofmt` + `go vet` + `go test` + `tsc` + `vite build`

## 8. Data model
- `snapshots(ticker, date, source, payload)` — raw ingestion
- `broker_activity(broker_code, ticker, date, buy, sell, net, lots, freq)`
- `foreign_flow(ticker, date, net_inflow)`
- `news_items(ticker, date, source, sentiment, confidence, url)`
- `filings(ticker, date, holder_type, txn_type, volume)`
- `routines(id, user_key, type, schedule, channels, enabled)`
- `routine_runs(routine_id, started_at, status, payload_json)`
- `alerts(id, user_key, name, rule_json, channels, last_fired)`
- `alert_events(alert_id, ticker, date, message, context_json, citations_json)`
- `watchlists(user_key, ticker, added_at)`
- `reports(id, ticker, generated_at, payload_json, citations_json)`
- `agent_accuracy(agent, ticker, prediction, date, resolved, hit)`
- `briefings(date, payload_json, citations_json)`

## 9. Backend routes
- `GET /api/flow/summary`, `GET /api/flow/broker`, `GET /api/flow/foreign`
- `POST /api/screen`
- `GET /api/routines`, `POST /api/routines`, `PATCH /api/routines/:id`, `GET /api/routine-runs`
- `GET /api/briefing/today`
- `GET /api/alerts`, `POST /api/alerts`, `DELETE /api/alerts/:id`, `GET /api/alert-events`
- `POST /api/report/:ticker`
- `POST /api/watchlist`, `GET /api/watchlist`
- `GET /api/portfolio/risk`, `GET /api/accuracy`
- `POST /api/chat`, `GET /api/health`

## 10. Frontend pages
- `/` Smart Money Dashboard + live agent panel + activity feed
- `/routines` Routine Manager (subscribe, jadwal, kanal, riwayat)
- `/screener` institutional screener
- `/alerts` rule builder + event history
- `/portfolio` risk heatmap + correlation + accuracy ledger
- `/report/:ticker` report + citations + interrogation scoped ke report
- Global: watchlist drawer + AI chat sidebar

## 11. Detection rules v1
1. Accumulation: ≥3 broker net-buy 5d + volume > 1.5× avg 20d
2. Foreign reversal: net outflow 5d lalu inflow 1d
3. Insider spike: director buy > 2× avg 30d
4. Unusual volume: >3× avg 20d, bukan earnings date
5. Sector rotation: net broker flow subsector balik arah week-over-week
6. Suspension watch: suspensi baru di watchlist

## 12. Sectors endpoints (70 paths — detail: docs/API-REFERENCE.md)
Company core: company/report (8 sections), get-segments, quarterly-financial-dates,
financials/quarterly, corporate-actions, shareholders-composition, listing-performance.
Universe sweeps: close/ (full-universe, paginated), companies/quarterly-financial-dates
(`since=` incremental). Market: daily, idx-total, index-daily, top-changes, most-traded.
Brokers: brokers/ (registry), brokers/top, broker-activity, broker-activity/top,
broker-summary, broker-summary/top, foreign-flow. Events: news, filings, suspensions.
Subsector: subsector/report (6 sections). Phase 2: SGX (9), KLSE (4), mining (19).

## 13. 48h timeline
| 0–3 | Setup: repo, API client, DB schema, health |
| 3–8 | Ingestion scheduler + snapshots |
| 8–14 | 7 agents + cross-signal correlation |
| 14–20 | Routine engine (briefing + radar) + webhooks |
| 20–26 | Screener + citations pipeline |
| 26–32 | Report generator + interrogation |
| 32–38 | Portfolio risk + accuracy ledger |
| 38–44 | Frontend wiring + SSE live panel |
| 44–48 | Polish, demo script, deck |

## 14. Verification
- `GET /api/health` balik last cycle + credits today + scheduler state (verifikasi dalam smoke test)
- Fixture briefing generate dari seed tanpa empty section + citations lengkap (unit test)
- Fixture akumulasi → alert event tercatat (rules_test) + webhook ke kanal uji bila token di-env
- Screener balikin ranked list + breakdown per row (api_test)
- Report BBCA 7 section terisi dari seed/live + citations, export PDF/HTML/MD/JSON (smoke)
- Key hanya dari env (`SECTORS_API_KEY`), v2 paths only (client_test: v1 ditolak sebelum HTTP)
- Market tutup → demo offline pakai historical replay seed (scripts/demo.sh)

## 15. Risks
- Butuh Insider API key sebelum jam 0
- Rate limit → cache Redis + siklus 30 min, tanpa loop per-ticker agresif
- v1 mati (410) — pakai v2 saja
