import { createResource, createSignal, onCleanup, For, Show } from "solid-js";
import { api, subscribeSSE, fmtIDR } from "../lib/api";
import { AccumulationTable, ActivityFeed, RotationMap } from "../components/Cards";
import { Citations } from "../components/Citations";
import { Badge } from "../components/ui/badge";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "../components/ui/card";
import { Skeleton } from "../components/ui/skeleton";
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
    <div class="space-y-6">
      <div>
        <h1 class="text-2xl font-bold tracking-tight">Smart Money Dashboard</h1>
        <p class="text-sm text-muted-foreground">
          Credits {health()?.credits_today ?? "…"} · Scheduler {health()?.scheduler_ok ? "ok" : "down"} · Last cycle {health()?.last_cycle_at || "never"}
        </p>
      </div>
      <Show when={health()?.stale_flags?.length}>
        <p class="text-sm text-muted-foreground">Stale: <For each={health()!.stale_flags}>{(f) => <Badge variant="outline">{f}</Badge>}</For></p>
      </Show>
      <Show when={!flow.loading} fallback={<div class="space-y-2"><Skeleton class="h-24 w-full" /><Skeleton class="h-24 w-full" /><Skeleton class="h-24 w-full" /></div>}>
        <div class="grid gap-4 md:grid-cols-2">
          <Card>
            <CardHeader><CardTitle>Foreign net total</CardTitle><CardDescription>Aggregate across snapshots.</CardDescription></CardHeader>
            <CardContent>
              <p class="font-mono text-2xl font-bold">{flow() ? fmtIDR(flow()!.foreign_net_total) : "…"}</p>
              <Citations items={flow()?.citations} />
            </CardContent>
          </Card>
          <AccumulationTable rows={flow()?.top_accumulation || []} />
          <RotationMap signal={undefined} citations={flow()?.citations} />
          <Card>
            <CardHeader><CardTitle>Foreign flow (BBCA)</CardTitle></CardHeader>
            <CardContent>
              <Show when={(foreign()?.dates || []).length} fallback={<p class="text-sm text-muted-foreground">No foreign-flow series yet.</p>}>
                <Line data={flowChart()} options={{ responsive: true }} />
                <Show when={foreign()?.reversal}><p class="text-sm">Reversal flag set.</p></Show>
              </Show>
            </CardContent>
          </Card>
          <ActivityFeed events={(events() || []).map((e) => ({ ticker: String(e.ticker || ""), message: String(e.message || ""), date: String(e.date || "") }))} />
          <Card>
            <CardHeader><CardTitle>Live agent panel</CardTitle></CardHeader>
            <CardContent>
              <Show when={live().length} fallback={<p class="text-sm text-muted-foreground">Waiting for SSE events…</p>}>
                <ul class="space-y-1 text-sm text-muted-foreground"><For each={live()}>{(l) => <li>{l}</li>}</For></ul>
              </Show>
            </CardContent>
          </Card>
        </div>
      </Show>
      <Card>
        <CardHeader><CardTitle>Morning briefing</CardTitle></CardHeader>
        <CardContent>
          <Show when={briefing()} fallback={<p class="text-sm text-muted-foreground">No briefing yet — subscribe to the morning-briefing routine.</p>}>
            <pre class="whitespace-pre-wrap text-sm text-muted-foreground">{(briefing() as { payload: string })?.payload?.slice(0, 800)}</pre>
          </Show>
        </CardContent>
      </Card>
    </div>
  );
}
