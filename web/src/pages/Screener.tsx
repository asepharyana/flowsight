import { createSignal, For, Show } from "solid-js";
import { api, type ScreenRow } from "../lib/api";
import { Citations } from "../components/Citations";

const SAVED_KEY = "fs-saved-screeners";

export default function Screener() {
  const [mode, setMode] = createSignal<"where" | "q">("q");
  const [q, setQ] = createSignal("");
  const [brokerMin, setBrokerMin] = createSignal(0);
  const [rows, setRows] = createSignal<ScreenRow[]>([]);
  const [err, setErr] = createSignal("");
  const loadSaved = (): Record<string, string> => {
    try { return JSON.parse(localStorage.getItem(SAVED_KEY) || "{}"); } catch { return {}; }
  };
  const [saved, setSaved] = createSignal<Record<string, string>>(loadSaved());
  const [sname, setSname] = createSignal("");
  async function run() {
    setErr("");
    try {
      const body = mode() === "q"
        ? { q: q() || undefined, institutional: { broker_score_min: brokerMin() }, limit: 20 }
        : { where: q() || undefined, institutional: { broker_score_min: brokerMin() }, limit: 20 };
      const r = await api.screen(body);
      setRows(r.rows);
    } catch (e) { setErr(String(e)); }
  }
  function save() {
    const all = { ...saved(), [sname().trim() || q().slice(0, 24) || "untitled"]: q() };
    localStorage.setItem(SAVED_KEY, JSON.stringify(all));
    setSaved(all);
  }
  return (
    <div>
      <h1>Institutional Screener</h1>
      <div class="fs-card">
        <select class="fs-input" value={mode()} onChange={(e) => setMode(e.currentTarget.value as "where" | "q")}>
          <option value="q">Natural language (q)</option>
          <option value="where">SQL-like (where)</option>
        </select>
        <input class="fs-input" placeholder={mode() === "q" ? "e.g. large banks with foreign inflow" : "e.g. market_cap > 10T"} value={q()} onInput={(e) => setQ(e.currentTarget.value)} style={{ width: "320px" }} />
        <label class="fs-muted"> broker ≥ <input class="fs-input" type="number" value={brokerMin()} onInput={(e) => setBrokerMin(Number(e.currentTarget.value))} style={{ width: "80px" }} /></label>
        <button class="fs-btn" onClick={run}>Screen</button>
        <Show when={err()}><p class="fs-muted">{err()}</p></Show>
      </div>
      <div class="fs-scroll"><table class="fs-table">
        <thead><tr><th>Symbol</th><th>Composite</th><th>Breakdown</th><th>Citations</th></tr></thead>
        <tbody>
          <For each={rows()}>
            {(r) => (
              <tr>
                <td><a href={`/report/${r.symbol}`}>{r.symbol}</a></td>
                <td>{r.composite.toFixed(1)}</td>
                <td class="fs-muted">{JSON.stringify(r.breakdown)}</td>
                <td><Citations items={r.citations} /></td>
              </tr>
            )}
          </For>
        </tbody>
      </table></div>
      <div class="fs-card">
        <h3>Saved screeners</h3>
        <input class="fs-input" placeholder="name" value={sname()} onInput={(e) => setSname(e.currentTarget.value)} style={{ width: "140px" }} />
        <button class="fs-btn ghost" onClick={save}>Save current</button>
        <ul><For each={Object.entries(saved())}>{([n, query]) => <li><button class="fs-btn ghost" onClick={() => { setQ(query); run(); }}>{n}</button> <span class="fs-muted">{query.slice(0, 60)}</span></li>}</For></ul>
      </div>
      <Show when={!rows().length}><p class="fs-muted">Run a screen to see ranked rows with per-row signal breakdown.</p></Show>
    </div>
  );
}
