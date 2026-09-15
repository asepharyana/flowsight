import { createResource, createSignal, onCleanup, For, Show } from "solid-js";
import { api, subscribeSSE, fmtIDR } from "../lib/api";
import { AccumulationTable, ActivityFeed, RotationMap } from "../components/Cards";
import { Citations } from "../components/Citations";
import { Line } from "solid-chartjs";
import { Chart, registerables } from "chart.js";
Chart.register(...registerables);

export default function Dashboard() {
  const [health] = createResource(() => api.health());
  const [flow, { refetch }] = createResource(() => api.flowSummary());
  const [events, { refetch: refetchEv }] = createResource(() => api.alertEvents("2000-01-01").then((r) => r.events.slice(0, 10)));
  const [foreign] = createResource(() => api.flowForeign("BBCA").catch(() => null));
  const [live, setLive] = createSignal<string[]>([]);
  const stop = subscribeSSE((ch, data) => {
    setLive((p) => [`${ch}: ${data.slice(0, 120)}`, ...p].slice(0, 5));
    if (ch === "alerts" || ch === "activity") { refetch(); refetchEv(); }
  });
  onCleanup(stop);
  const flowChart = () => ({
    labels: (foreign()?.dates || []) as string[],
    datasets: [{ label: "BBCA foreign net", data: (foreign()?.nets || []) as number[] }],
  });
  return (
    <div>
      <h1>Smart Money Dashboard</h1>
      <Show when={health()?.stale_flags?.length}>
        <p class="fs-muted">Stale: <For each={health()!.stale_flags}>{(f) => <span>{f}; </span>}</For></p>
      </Show>
      <p class="fs-muted">
        Credits today: {health()?.credits_today ?? "…"} · Scheduler: {health()?.scheduler_ok ? "ok" : "down"} ·
        Last cycle: {health()?.last_cycle_at || "never"}
      </p>
      <div class="fs-grid">
        <div class="fs-card">
          <h3>Foreign net total</h3>
          <p style={{ "font-size": "24px" }}>{flow() ? fmtIDR(flow()!.foreign_net_total) : "…"}</p>
          <Citations items={flow()?.citations} />
        </div>
        <AccumulationTable rows={flow()?.top_accumulation || []} />
        <RotationMap signal={undefined} citations={flow()?.citations} />
        <div class="fs-card">
          <h3>Foreign flow (BBCA)</h3>
          <Show when={(foreign()?.dates || []).length} fallback={<p class="fs-muted">No foreign-flow series yet.</p>}>
            <Line data={flowChart()} options={{ responsive: true }} />
            <Show when={foreign()?.reversal}><p>Reversal flag set.</p></Show>
          </Show>
        </div>
        <ActivityFeed events={(events() || []).map((e) => ({ ticker: String(e.ticker || ""), message: String(e.message || ""), date: String(e.date || "") }))} />
        <div class="fs-card">
          <h3>Live agent panel</h3>
          <Show when={live().length} fallback={<p class="fs-muted">Waiting for SSE events…</p>}>
            <ul><For each={live()}>{(l) => <li class="fs-muted">{l}</li>}</For></ul>
          </Show>
        </div>
      </div>
    </div>
  );
}
