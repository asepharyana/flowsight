# Tech stack

Pinned versions. Change here before code. CI enforces the gates at the bottom.

## Backend — `backend/` (Go 1.23)

| Piece | Choice | Why |
|---|---|---|
| Runtime | Go 1.23 | Single binary, fast cold start on demo machines, `net/http` routing mature since 1.22 |
| Router | chi v5.2.3 | Thin router over stdlib mux (middleware, route groups); no framework lock-in |
| HTTP client | stdlib `net/http` + tuned `Transport` | One shared client for all Sectors calls (pooling, per-endpoint timeouts) |
| Validation | go-playground/validator v10.25.0 | Request struct tags = kontrak docs/API.md; gagal validasi → 422 |
| DB | `database/sql` + modernc.org/sqlite v1.39.0 (pure Go) | Nol CGO — `mattn/go-sqlite3` butuh gcc dan gagal di mesin juri tanpa toolchain; schema Postgres-compatible, migrasi SQL polos bernomor, tanpa ORM |
| Cache | go-redis v9.12.1; in-memory TTL fallback bila `REDIS_URL` kosong | Cache registry/taxonomy (TTL 24 jam); demo tetap jalan tanpa Redis |
| Scheduler | robfig/cron v3.0.1 | Cron per routine + interval ingestion dalam satu proses |
| LLM | Plain HTTPS ke endpoint OpenAI-compatible (`LLM_BASE_URL`) | Function calling untuk synthesis/report/briefing; `LLM_MODEL_TRIAGE` murah + `LLM_MODEL_SYNTH` kuat, override lewat env |
| PDF export | gofpdf (jung-kurt fork) | Pure Go, tanpa system deps |
| Config | env via `os.Getenv` + `godotenv` untuk dev | Semua secret dari env; contoh di `.env.example` |
| Test | `go test` + `httptest` (mock upstream Sectors) | Agent/rule/budget tests atas fixture JSON bentuk API asli |
| Lint/type | `gofmt -l` + `go vet ./...` (+ golangci-lint bila tersedia) | Compiler sudah strict; vet menangkap yang penting |

## Frontend — `web/` (SolidJS + typed CSS)

| Piece | Choice | Why |
|---|---|---|
| Framework | SolidJS 1.9.15 + Vite 6.4.3 + TypeScript 5.6.3 | Fine-grained reactivity — ideal untuk live feed/SSE tanpa re-render tree; bundle kecil untuk demo cepat |
| Styling | Typed CSS modules + CSS custom-property tokens (`web/src/styles/`) | Atomic CSS deterministic, typed via TS, tanpa runtime; ganti Tailwind sepenuhnya |
| Charts | Chart.js 4.4.7 via solid-chartjs 1.3.11 | Wrapper Solid resmi untuk flow/series; Recharts React-only jadi tidak dipakai |
| Data fetch | `fetch` + typed client (`lib/api.ts`) + `EventSource` untuk SSE | Backend satu-satunya sumber kebenaran; web tidak pernah manggil Sectors langsung |
| Test/gate | `tsc --noEmit` + `vite build` | Cukup untuk hackathon; tanpa e2e framework |

## Package / runtime management

| Piece | Choice | Why |
|---|---|---|
| Go deps | Go modules (`go.mod`, vendoring opsional via `go mod vendor`) | Build reproducible; `go build ./...` satu perintah |
| JS deps | pnpm 9 + `pnpm-lock.yaml` | Install deterministik; fallback `npm` jika pnpm tidak ada |
| Env | `.env` (tidak di-commit) — `SECTORS_API_KEY`, `LLM_API_KEY`, `LLM_BASE_URL`, `LLM_MODEL_SYNTH`, `LLM_MODEL_TRIAGE`, `TELEGRAM_BOT_TOKEN`, `DISCORD_WEBHOOK_URL`, `REDIS_URL`, `BWS_PROJECT_ID` | Semua secret dari env; contoh di `.env.example`. Di prod simpan secret di Bitwarden Secrets Manager dan jalankan via `sh scripts/bws-run.sh …` (env tetap fallback). `TELEGRAM_*`/`DISCORD_*` hanya fallback server; tiap user kelola tujuannya di `/api/destinations` |
| Procfile dev | dua proses: `go run ./cmd/server` (atau `air` untuk reload) + `vite dev` (+ redis opsional) | Demo tetap jalan tanpa Redis |

## Alternatives declined

- **Python/FastAPI** — startup + packaging demo lebih rapuh (venv, pip) dibanding satu binary Go; konkurensi agent paralel setara via goroutine.
- **Next.js/React** — overhead framework + re-render model untuk dashboard live; Solid memberi update granular dengan bundle lebih kecil.
- **Tailwind / StyleX** — diganti typed CSS modules + token CSS: nol runtime, tanpa scanning step, tanpa unplugin versi (registry belum stabil); kontrak visual tetap lewat `styles/tokens.css`.
- **Recharts** — React-only; Chart.js via solid-chartjs menutup kebutuhan chart di Solid.
- **mattn/go-sqlite3** — butuh CGO/gcc; modernc pure-Go selalu bisa build.
- **GORM / sqlc / Alembic-style migrator** — overhead untuk 13 tabel; SQL polos + skrip bernomor cukup dan mudah diaudit juri.
- **Celery / job queue eksternal** — butuh broker; cron in-process cukup untuk siklus 30 menit.
- **WebSocket** — push satu arah saja (agents/alerts/activity); SSE lebih simpel + auto-reconnect.
- **tRPC / GraphQL** — REST + JSON typed tanpa layer tambahan.
- **MongoDB** — data relasional time-series (ticker × date); SQLite + indeks tepat lebih cepat dibangun.

## CI gates (per commit)

1. `gofmt -l backend/` kosong + `go vet ./...` bersih
2. `go test ./...` (termasuk budget test: kredit per siklus ≤ cap pada fixture)
3. `tsc --noEmit` + `vite build` di `web/`
4. Larangan: tidak ada HTTP call ke `api.sectors.app` di luar `backend/sectors/`;
   tidak ada angka user-visible tanpa citation (review checklist, bukan linter).
