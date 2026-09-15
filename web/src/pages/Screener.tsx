import { createSignal, For, Show } from "solid-js";
import { api, type ScreenRow } from "../lib/api";
import { Citations } from "../components/Citations";
import { BreakdownBars, EmptyState, RecBadge, Skeleton } from "../components/ui";

const SAVED_KEY = "fs-saved-screeners";

// scoreOf extracts a -100..+100 composite from the row breakdown for the badge.
function scoreOf(r: ScreenRow): number {
  const b = r.breakdown as Record<string, unknown>;
  for (const k of ["composite", "score", "broker_score", "broker"]) {
    if (typeof b[k] === "number") return b[k] as number;
  }
  return r.composite || 0;
}

export default function Screener() {
  const [mode, setMode] = createSignal<"where" | "q">("q");
  const [q, setQ] = createSignal("");
  const [brokerMin, setBrokerMin] = createSignal(0);
  const [foreignOnly, setForeignOnly] = createSignal(false);
  const [insiderOnly, setInsiderOnly] = createSignal(false);
  const [rows, setRows] = createSignal<ScreenRow[]>([]);
  const [ran, setRan] = createSignal(false);
  const [busy, setBusy] = createSignal(false);
  const [err, setErr] = createSignal("");
  const loadSaved = (): Record<string, string> => {
    try { return JSON.parse(localStorage.getItem(SAVED_KEY) || "{}"); } catch { return {}; }
  };
  const [saved, setSaved] = createSignal<Record<string, string>>(loadSaved());
  const [sname, setSname] = createSignal("");
  async function run() {
    setErr(""); setBusy(true);
    try {
      const institutional: Record<string, unknown> = { broker_score_min: brokerMin() };
      if (foreignOnly()) institutional.foreign_inflow = true;
      if (insiderOnly()) institutional.insider_buying = true;
      const body = mode() === "q"
        ? { q: q() || undefined, institutional, limit: 20 }
        : { where: q() || undefined, institutional, limit: 20 };
      const r = await api.screen(body);
      setRows(r.rows); setRan(true);
    } catch (e) { setErr(String(e)); }
    finally { setBusy(false); }
  }
  function save() {
    const all = { ...saved(), [sname().trim() || q().slice(0, 24) || "untitled"]: q() };
    localStorage.setItem(SAVED_KEY, JSON.stringify(all));
    setSaved(all);
  }
  return (
    <div>
      <div class="fs-page-head"><h1>Institutional Screener</h1><p class="fs-muted">Rank tickers by smart-money signals.</p></div>
      <div class="fs-card">
        <div class="fs-row">
          <select class="fs-input" value={mode()} onChange={(e) => setMode(e.currentTarget.value as "where" | "q")}>
            <option value="q">Natural language (q)</option>
            <option value="where">SQL-like (where)</option>
          </select>
          <input class="fs-input" placeholder={mode() === "q" ? "e.g. large banks with foreign inflow" : "e.g. market_cap > 10T"} value={q()} onInput={(e) => setQ(e.currentTarget.value)} style={{ flex: 1, "min-width": "200px" }} />
          <button class="fs-btn" onClick={run} disabled={busy()}>{busy() ? " screening…" : "Screen"}</button>
        </div>
        <div class="fs-row" style={{ "margin-top": "10px" }}>
          <label class="fs-muted">broker ≥ <input class="fs-input" type="number" value={brokerMin()} onInput={(e) => setBrokerMin(Number(e.currentTarget.value))} style={{ width: "80px" }} /></label>
          <label class="fs-muted"><input type="checkbox" checked={foreignOnly()} onChange={(e) => setForeignOnly(e.currentTarget.checked)} /> foreign inflow</label>
          <label class="fs-muted"><input type="checkbox" checked={insiderOnly()} onChange={(e) => setInsiderOnly(e.currentTarget.checked)} /> insider buying</label>
        </div>
        <Show when={err()}><p style={{ color: "var(--fs-avoid)" }}>{err()}</p></Show>
      </div>
      <Show when={busy()}><div style={{ "margin-top": "14px" }}><Skeleton rows={3} /></div></Show>
      <Show when={ran() && !busy()}>
        <Show when={rows().length} fallback={<div style={{ "margin-top": "14px" }}><EmptyState icon="🔍" title="No matches" hint="Loosen the filters and try again." /></div>}>
          <div class="fs-grid" style={{ "margin-top": "14px" }}>
            <For each={rows()}>
              {(r) => (
                <div class="fs-card">
                  <div class="fs-row" style={{ "justify-content": "space-between" }}>
                    <strong><a href={`/report/${r.symbol}`}>{r.symbol}</a></strong>
                    <span class="fs-muted">{r.composite.toFixed(1)}</span>
                  </div>
                  <div style={{ "margin": "8px 0" }}><BreakdownBars breakdown={r.breakdown as Record<string, unknown>} /></div>
                  <Citations items={r.citations} />
                  <Show when={scoreOf(r) !== 0}><div style={{ "margin-top": "6px" }}><RecBadge rec={scoreOf(r) >= 40 ? "BUY" : scoreOf(r) <= -40 ? "AVOID" : "HOLD"} /></div></Show>
                </div>
              )}
            </For>
          </div>
        </Show>
      </Show>
      <Show when={!ran() && !busy()}>
        <div style={{ "margin-top": "14px" }}><EmptyState icon="🔍" title="Run a screen" hint="Ranked rows with per-signal bars appear here." /></div>
      </Show>
      <div class="fs-card" style={{ "margin-top": "14px" }}>
        <h3>Saved screeners</h3>
        <div class="fs-row">
          <input class="fs-input" placeholder="name" value={sname()} onInput={(e) => setSname(e.currentTarget.value)} style={{ width: "140px" }} />
          <button class="fs-btn ghost" onClick={save}>Save current</button>
        </div>
        <ul><For each={Object.entries(saved())}>{([n, query]) => <li><button class="fs-btn ghost" onClick={() => { setQ(query); run(); }}>{n}</button> <span class="fs-muted">{query.slice(0, 60)}</span></li>}</For></ul>
      </div>
    </div>
  );
}
