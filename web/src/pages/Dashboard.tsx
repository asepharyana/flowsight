import { createResource, createSignal, onCleanup, For, Show } from "solid-js";
import { api, subscribeSSE, fmtIDR } from "../lib/api";
import { AccumulationTable, ActivityFeed, RotationMap } from "../components/Cards";
import { Citations } from "../components/Citations";
import { StatCard, EmptyState, Skeleton } from "../components/ui";
import { Line } from "solid-chartjs";
import { Chart, registerables } from "chart.js";
Chart.register(...registerables);

export default function Dashboard() {
  const [health] = createResource(() => api.health());
  const [flow, { refetch }] = createResource(() => api.flowSummary());
  const [events, { refetch: refetchEv }] = createResource(() => api.alertEvents("2000-01-01").then((r) => r.events.slice(0, 10)));
  const [foreign] = createResource(() => api.flowForeign("BBCA").catch(() => null));
  const [briefing] = createResource(() => api.briefing().catch(() => null));
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
      <div class="fs-page-head">
        <h1>Smart Money Dashboard</h1>
        <p class="fs-muted">Credits {health()?.credits_today ?? "…"} · Scheduler {health()?.scheduler_ok ? "ok" : "down"} · Last cycle {health()?.last_cycle_at || "never"}</p>
      </div>
      <Show when={health()?.stale_flags?.length}>
        <p class="fs-muted">Stale: <For each={health()!.stale_flags}>{(f) => <span>{f}; </span>}</For></p>
      </Show>
      <Show when={!flow.loading} fallback={<Skeleton rows={4} />}>
        <div class="fs-grid">
          <StatCard title="Foreign net total" value={flow() ? fmtIDR(flow()!.foreign_net_total) : "…"}>
            <Citations items={flow()?.citations} />
          </StatCard>
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
      </Show>
      <div style={{ "margin-top": "14px" }}>
        <div class="fs-card">
          <h3>Morning briefing</h3>
          <Show when={briefing()} fallback={<p class="fs-muted">No briefing yet — subscribe to the morning-briefing routine.</p>}>
            <pre class="fs-muted">{(briefing() as { payload: string })?.payload?.slice(0, 800)}</pre>
          </Show>
        </div>
      </div>
    </div>
  );
}
