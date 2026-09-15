import { createResource, createSignal, For, Show } from "solid-js";
import { api } from "../lib/api";
import { Citations } from "../components/Citations";
import { EmptyState } from "../components/ui";

const TYPES = ["morning-briefing", "accumulation-radar", "foreign-reversal", "insider-tape", "earnings-countdown", "dividend-calendar", "weekend-review"];

export default function Routines() {
  const [data, { refetch }] = createResource(() => api.routines());
  const [runs, { refetch: refetchRuns }] = createResource(() => api.runs().then((r) => r.runs.slice(0, 20)));
  const [type_, setType] = createSignal("morning-briefing");
  const [schedule, setSchedule] = createSignal("");
  const [channels, setChannels] = createSignal("");
  const [busy, setBusy] = createSignal(false);
  const [runNowId, setRunNowId] = createSignal<number | null>(null);
  const [briefing] = createResource(() => api.briefing().catch(() => null));
  async function subscribe() {
    setBusy(true);
    try {
      await api.createRoutine({
        type: type_(),
        schedule_cron: schedule().trim() || undefined,
        channels: channels().split(",").map((c) => c.trim()).filter(Boolean),
      });
      refetch();
    } finally { setBusy(false); }
  }
  async function toggle(id: number, enabled: boolean) {
    await api.updateRoutine(id, { enabled: !enabled });
    refetch();
  }
  async function del(id: number) {
    await fetch(`/api/routines/${id}`, { method: "DELETE", headers: { "X-User-Key": localStorage.getItem("fs-key") || "demo" } });
    refetch();
  }
  return (
    <div>
      <div class="fs-page-head"><h1>Routine Manager</h1><p class="fs-muted">Scheduled deliveries with citations.</p></div>
      <div class="fs-card">
        <h3>Subscribe</h3>
        <div class="fs-row">
          <select class="fs-input" value={type_()} onChange={(e) => setType(e.currentTarget.value)}>
            <For each={TYPES}>{(t) => <option value={t}>{t}</option>}</For>
          </select>
          <input class="fs-input" placeholder="schedule cron (blank = default)" value={schedule()} onInput={(e) => setSchedule(e.currentTarget.value)} style={{ width: "240px" }} />
          <input class="fs-input" placeholder="channels, comma-separated" value={channels()} onInput={(e) => setChannels(e.currentTarget.value)} style={{ width: "200px" }} />
          <button class="fs-btn" onClick={subscribe} disabled={busy()}>Subscribe</button>
        </div>
      </div>
      <Show when={(data()?.routines || []).length} fallback={
        <div style={{ "margin-top": "14px" }}><EmptyState icon="🗓️" title="No routines yet" hint="Subscribe above — the morning briefing runs 07:30 WIB daily." /></div>
      }>
        <div class="fs-grid" style={{ "margin-top": "14px" }}>
          <For each={data()?.routines || []}>
            {(r) => (
              <div class="fs-card">
                <div class="fs-row" style={{ "justify-content": "space-between" }}>
                  <strong>{r.type}</strong>
                  <span class={`fs-badge ${r.enabled ? "buy" : "info"}`}>{r.enabled ? "on" : "off"}</span>
                </div>
                <p class="fs-muted">{r.schedule_cron}</p>
                <div class="fs-row">
                  <button class="fs-btn ghost" onClick={() => toggle(r.id, r.enabled)}>{r.enabled ? "Disable" : "Enable"}</button>
                  <button class="fs-btn ghost" disabled={runNowId() === r.id}>Run</button>
                  <button class="fs-btn danger" onClick={() => del(r.id)}>Delete</button>
                </div>
              </div>
            )}
          </For>
        </div>
      </Show>
      <div class="fs-grid" style={{ "margin-top": "14px" }}>
        <div class="fs-card">
          <h3>Latest briefing</h3>
          <Show when={briefing()} fallback={<p class="fs-muted">No briefing yet — it generates after the first morning-briefing run.</p>}>
            <pre class="fs-muted" style={{ "white-space": "pre-wrap" }}>{(briefing() as { payload: string })?.payload?.slice(0, 800)}</pre>
            <Citations items={(briefing() as { citations: string })?.citations} />
          </Show>
        </div>
        <div class="fs-card">
          <h3>Run history</h3>
          <button class="fs-btn ghost" onClick={() => refetchRuns()}>Refresh</button>
          <ul><For each={runs() || []}>{(r) => <li class="fs-muted">{String(r.started_at)} — {String(r.status)}</li>}</For></ul>
        </div>
      </div>
    </div>
  );
}
