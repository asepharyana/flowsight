// Typed backend client (backend is the only source of truth; web never calls Sectors directly).
const BASE = "";
function headers(): HeadersInit {
  return { "Content-Type": "application/json", "X-User-Key": localStorage.getItem("fs-key") || "demo" };
}
async function req<T>(path: string, init?: RequestInit): Promise<T> {
  const r = await fetch(BASE + path, { ...init, headers: { ...headers(), ...(init?.headers || {}) } });
  if (!r.ok) {
    let msg = r.statusText;
    try { const e = await r.json(); msg = e?.error?.message || msg; } catch { /* keep status */ }
    throw new Error(msg);
  }
  const ct = r.headers.get("content-type") || "";
  if (ct.includes("application/pdf")) return (await r.blob()) as unknown as T;
  if (ct.includes("text/")) return (await r.text()) as unknown as T;
  return (await r.json()) as T;
}
export interface Citation { endpoint: string; snapshot_at: string; ticker?: string; stale?: boolean }
export interface Health { last_cycle_at: string; credits_today: number; scheduler_ok: boolean; stale_flags: string[] }
export interface FlowSummary { date: string; foreign_net_total: number; top_accumulation: { ticker: string; net_sum: number; brokers: number }[]; closes: { ticker: string; close: number; date: string }[]; citations: Citation[] }
export interface ScreenRow { symbol: string; name: string; composite: number; breakdown: Record<string, unknown>; citations: Citation[] }
export interface Routine { id: number; user_key: string; type: string; schedule_cron: string; channels_json?: string; channels?: string[]; enabled: boolean; last_run?: unknown }
export interface AlertItem { id: number; user_key: string; name: string; rule_json: string; channels_json: string; last_fired: string }
export interface ReportPayload { ticker: string; generated_at: string; sections: { name: string; body: string; citations: Citation[] }[]; synthesis: { recommendation: string; conviction: number; thesis: string; position_pct: number; conflict: boolean; conflict_note?: string }; agent_scores: unknown[]; citations: Citation[]; narasi_awam?: string }
export interface AuthUser { id: number; email: string; name: string; avatar_url: string; user_key: string; google_configured: boolean }
export interface ForeignSeries { dates: string[]; nets: number[]; reversal: boolean; start?: string; end?: string; citations: Citation[] }
export const api = {
  health: (force = false) => req<Health>(`/api/health${force ? "?force=1" : ""}`),
  flowSummary: () => req<FlowSummary>("/api/flow/summary"),
  flowBroker: (ticker: string) => req<{ buyers: unknown[]; sellers: unknown[]; citations: Citation[] }>(`/api/flow/broker?ticker=${ticker}`),
  flowForeign: (ticker: string) => req<ForeignSeries>(`/api/flow/foreign?ticker=${ticker}`),
  screen: (body: Record<string, unknown>) => req<{ rows: ScreenRow[]; count: number }>("/api/screen", { method: "POST", body: JSON.stringify(body) }),
  routines: () => req<{ routines: Routine[] }>("/api/routines"),
  createRoutine: (body: Record<string, unknown>) => req<{ id: number }>("/api/routines", { method: "POST", body: JSON.stringify(body) }),
  updateRoutine: (id: number, body: Record<string, unknown>) => req<unknown>(`/api/routines/${id}`, { method: "PATCH", body: JSON.stringify(body) }),
  runs: (routine_id?: number) => req<{ runs: Record<string, unknown>[] }>(`/api/routine-runs${routine_id ? `?routine_id=${routine_id}` : ""}`),
  briefing: () => req<{ date: string; payload: string; citations: string; narasi?: string }>("/api/briefing/today"),
  destinations: () => req<{ destinations: { id: number; kind: string; label: string; enabled: boolean; configured: boolean }[] }>("/api/destinations"),
  createDestination: (body: Record<string, unknown>) => req<{ id: number }>("/api/destinations", { method: "POST", body: JSON.stringify(body) }),
  updateDestination: (id: number, body: Record<string, unknown>) => req<unknown>(`/api/destinations/${id}`, { method: "PATCH", body: JSON.stringify(body) }),
  deleteDestination: (id: number) => req<unknown>(`/api/destinations/${id}`, { method: "DELETE" }),
  alerts: () => req<{ alerts: AlertItem[] }>("/api/alerts"),
  createAlert: (body: Record<string, unknown>) => req<{ id: number }>("/api/alerts", { method: "POST", body: JSON.stringify(body) }),
  deleteAlert: (id: number) => req<unknown>(`/api/alerts/${id}`, { method: "DELETE" }),
  alertEvents: (since = "2000-01-01", ticker = "") => req<{ events: Record<string, unknown>[] }>(`/api/alert-events?since=${since}${ticker ? `&ticker=${ticker}` : ""}`),
  report: (ticker: string, profile = "moderate") => req<ReportPayload>(`/api/report/${ticker}?profile=${profile}`, { method: "POST" }),
  reportMd: (ticker: string) => req<string>(`/api/report/${ticker}?format=md`, { method: "POST" }),
  interrogate: (ticker: string, question: string, report_id?: number) => req<{ answer: string; report_id: number; citations: unknown }>(`/api/report/${ticker}/ask`, { method: "POST", body: JSON.stringify({ question, report_id }) }),
  watchlist: () => req<{ watchlist: string[] }>("/api/watchlist"),
  addWatch: (ticker: string) => req<unknown>("/api/watchlist", { method: "POST", body: JSON.stringify({ ticker }) }),
  removeWatch: (ticker: string) => req<unknown>(`/api/watchlist/${ticker}`, { method: "DELETE" }),
  risk: () => req<{ concentration: { ticker: string; sector: string; weight: number }[]; correlation: Record<string, Record<string, number>>; beta: number; warnings: string[] }>("/api/portfolio/risk"),
  accuracy: () => req<{ agents: { agent: string; calls: number; resolved: number; hits: number; hit_rate: number }[] }>("/api/accuracy"),
  chat: (message: string) => req<{ answer: string }>(`/api/chat`, { method: "POST", body: JSON.stringify({ message }) }),
  me: () => req<{ user: AuthUser }>(`/api/auth/me`),
  logout: () => req<{ ok: boolean }>(`/api/auth/logout`, { method: "POST" }),
};
// SSE hook helper: subscribe to channels agents|alerts|activity.
export function subscribeSSE(onEvent: (channel: string, data: string) => void): () => void {
  const es = new EventSource("/api/stream");
  es.addEventListener("agents", (e) => onEvent("agents", (e as MessageEvent).data));
  es.addEventListener("alerts", (e) => onEvent("alerts", (e as MessageEvent).data));
  es.addEventListener("activity", (e) => onEvent("activity", (e as MessageEvent).data));
  es.onmessage = (e) => onEvent("message", e.data);
  return () => es.close();
}
export function fmtIDR(v: number): string {
  const neg = v < 0; const a = Math.abs(v);
  let s = `${Math.round(a)}`;
  if (a >= 1e12) s = `Rp${(a / 1e12).toFixed(2)}T`;
  else if (a >= 1e9) s = `Rp${Math.round(a / 1e9)}B`;
  else if (a >= 1e6) s = `Rp${Math.round(a / 1e6)}M`;
  return neg ? "-" + s : s;
}
