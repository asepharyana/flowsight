import { createResource, For, Show } from "solid-js";
import { api } from "../lib/api";
import { Bar } from "solid-chartjs";
import { Chart, registerables } from "chart.js";
import { PageHead } from "../components/ui";
import { Card, CardContent, CardHeader, CardTitle } from "../components/ui/card";
import { Skeleton } from "../components/ui/skeleton";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "../components/ui/table";
Chart.register(...registerables);

export default function Portfolio() {
  const [risk] = createResource(() => api.risk());
  const [acc] = createResource(() => api.accuracy());
  const chartData = () => ({
    labels: (risk()?.concentration || []).map((c) => c.ticker),
    datasets: [{ label: "Weight %", data: (risk()?.concentration || []).map((c) => +(c.weight * 100).toFixed(1)) }],
  });
  return (
    <div class="space-y-4">
      <PageHead title="Portfolio Risk + Accuracy Ledger" sub="Computed from your watchlist." />
      <Show when={!risk.loading && !acc.loading} fallback={<div class="space-y-2"><Skeleton class="h-24 w-full" /><Skeleton class="h-24 w-full" /></div>}>
        <div class="grid gap-4 md:grid-cols-2">
          <Card>
            <CardHeader><CardTitle>Concentration</CardTitle></CardHeader>
            <CardContent>
              <Show when={(risk()?.concentration || []).length} fallback={<p class="text-sm text-muted-foreground">Add tickers to your watchlist to see concentration.</p>}>
                <Bar data={chartData()} options={{ responsive: true }} />
              </Show>
              <For each={risk()?.warnings || []}>{(w) => <p class="text-sm">⚠️ {w}</p>}</For>
            </CardContent>
          </Card>
          <Card>
            <CardHeader><CardTitle>Beta vs index</CardTitle></CardHeader>
            <CardContent class="space-y-3">
              <p class="font-mono text-2xl font-bold">{risk() ? String(risk()!.beta) : "…"}</p>
              <h3 class="text-sm font-semibold">Correlation heatmap</h3>
              <CorrHeatmap matrix={risk()?.correlation || {}} />
            </CardContent>
          </Card>
          <Card>
            <CardHeader><CardTitle>Agent accuracy</CardTitle></CardHeader>
            <CardContent>
              <Table>
                <TableHeader><TableRow><TableHead>Agent</TableHead><TableHead class="text-right">Calls</TableHead><TableHead class="text-right">Resolved</TableHead><TableHead class="text-right">Hit %</TableHead></TableRow></TableHeader>
                <TableBody>
                  <For each={acc()?.agents || []}>
                    {(a) => <TableRow><TableCell>{a.agent}</TableCell><TableCell class="text-right">{a.calls}</TableCell><TableCell class="text-right">{a.resolved}</TableCell><TableCell class="text-right">{(a.hit_rate * 100).toFixed(0)}%</TableCell></TableRow>}
                  </For>
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        </div>
      </Show>
    </div>
  );
}

function CorrHeatmap(props: { matrix: Record<string, Record<string, number>> }) {
  const keys = () => Object.keys(props.matrix);
  const cell = (v: number) => {
    const a = Math.min(1, Math.abs(v));
    return v >= 0 ? `rgba(34,197,94,${(0.12 + 0.6 * a).toFixed(2)})` : `rgba(239,68,68,${(0.12 + 0.6 * a).toFixed(2)})`;
  };
  return (
    <Show when={keys().length} fallback={<p class="text-sm text-muted-foreground">No correlation data yet.</p>}>
      <div class="overflow-x-auto"><Table>
        <TableHeader><TableRow><TableHead></TableHead><For each={keys()}>{(k) => <TableHead>{k}</TableHead>}</For></TableRow></TableHeader>
        <TableBody>
          <For each={keys()}>
            {(r) => (
              <TableRow>
                <TableCell class="font-bold">{r}</TableCell>
                <For each={keys()}>
                  {(c) => {
                    const v = props.matrix[r]?.[c] ?? 0;
                    return <TableCell style={{ background: cell(v) }} title={`${r}/${c} = ${v.toFixed(2)}`}>{v.toFixed(2)}</TableCell>;
                  }}
                </For>
              </TableRow>
            )}
          </For>
        </TableBody>
      </Table></div>
    </Show>
  );
}
