import { createResource, createSignal, For, Show } from "solid-js";
import { api } from "../lib/api";
import { Citations } from "../components/Citations";

export default function Routines() {
  const [data, { refetch }] = createResource(() => api.routines());
  const [runs, { refetch: refetchRuns }] = createResource(() => api.runs().then((r) => r.runs.slice(0, 20)));
  const [type_, setType] = createSignal("morning-briefing");
  const [schedule, setSchedule] = createSignal("");
  const [channels, setChannels] = createSignal("");
  const [briefing] = createResource(() => api.briefing().catch(() => null));
  async function subscribe() {
    await api.createRoutine({
      type: type_(),
      schedule_cron: schedule().trim() || undefined,
      channels: channels().split(",").map((c) => c.trim()).filter(Boolean),
    });
    refetch();
  }
  async function toggle(id: number, enabled: boolean) {
    await api.updateRoutine(id, { enabled: !enabled });
    refetch();
  }
  return (
    <div>
      <h1>Routine Manager</h1>
      <div class="fs-card">
        <h3>Subscribe</h3>
        <select class="fs-input" value={type_()} onChange={(e) => setType(e.currentTarget.value)}>
          <For each={["morning-briefing","accumulation-radar","foreign-reversal","insider-tape","earnings-countdown","dividend-calendar","weekend-review"]}>
            {(t) => <option value={t}>{t}</option>}
          </For>
        </select>
        <input class="fs-input" placeholder="schedule cron (blank = default)" value={schedule()} onInput={(e) => setSchedule(e.currentTarget.value)} style={{ width: "260px" }} />
        <input class="fs-input" placeholder="channels, comma-separated" value={channels()} onInput={(e) => setChannels(e.currentTarget.value)} style={{ width: "220px" }} />
        <button class="fs-btn" onClick={subscribe}>Subscribe</button>
      </div>
      <div class="fs-grid">
        <For each={data()?.routines || []}>
          {(r) => (
            <div class="fs-card">
              <strong>{r.type}</strong> <span class="fs-muted">{r.schedule_cron} · {r.enabled ? "on" : "off"}</span>
              <div><button class="fs-btn ghost" onClick={() => toggle(r.id, r.enabled)}>{r.enabled ? "Disable" : "Enable"}</button></div>
            </div>
          )}
        </For>
      </div>
      <Show when={!data()?.routines?.length}><p class="fs-muted">No routines yet.</p></Show>
      <div class="fs-card">
        <h3>Latest briefing</h3>
        <Show when={briefing()} fallback={<p class="fs-muted">No briefing yet.</p>}>
          <pre class="fs-muted">{(briefing() as { payload: string })?.payload?.slice(0, 800)}</pre>
          <Citations items={(briefing() as { citations: string })?.citations} />
        </Show>
      </div>
      <div class="fs-card">
        <h3>Run history</h3>
        <button class="fs-btn ghost" onClick={() => refetchRuns()}>Refresh</button>
        <ul><For each={runs() || []}>{(r) => <li class="fs-muted">{String(r.started_at)} — {String(r.status)}</li>}</For></ul>
      </div>
    </div>
  );
}
