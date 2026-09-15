import { createResource, For, Show } from "solid-js";
import { api } from "../lib/api";
import { Bar } from "solid-chartjs";
import { Chart, registerables } from "chart.js";
import { StatCard, Skeleton } from "../components/ui";
Chart.register(...registerables);

export default function Portfolio() {
  const [risk] = createResource(() => api.risk());
  const [acc] = createResource(() => api.accuracy());
  const chartData = () => ({
    labels: (risk()?.concentration || []).map((c) => c.ticker),
    datasets: [{ label: "Weight %", data: (risk()?.concentration || []).map((c) => +(c.weight * 100).toFixed(1)) }],
  });
  return (
    <div>
      <div class="fs-page-head"><h1>Portfolio Risk + Accuracy Ledger</h1><p class="fs-muted">Computed from your watchlist.</p></div>
      <Show when={!risk.loading && !acc.loading} fallback={<Skeleton rows={4} />}>
        <div class="fs-grid">
          <div class="fs-card">
            <h3>Concentration</h3>
            <Show when={(risk()?.concentration || []).length} fallback={<p class="fs-muted">Add tickers to your watchlist to see concentration.</p>}>
              <Bar data={chartData()} options={{ responsive: true }} />
            </Show>
            <For each={risk()?.warnings || []}>{(w) => <p>⚠️ {w}</p>}</For>
          </div>
          <StatCard title="Beta vs index" value={risk() ? String(risk()!.beta) : "…"}>
            <h3 style={{ "margin-top": "12px" }}>Correlation heatmap</h3>
            <CorrHeatmap matrix={risk()?.correlation || {}} />
          </StatCard>
          <div class="fs-card">
            <h3>Agent accuracy</h3>
            <table class="fs-table">
              <thead><tr><th>Agent</th><th>Calls</th><th>Resolved</th><th>Hit %</th></tr></thead>
              <tbody>
                <For each={acc()?.agents || []}>
                  {(a) => <tr><td>{a.agent}</td><td>{a.calls}</td><td>{a.resolved}</td><td>{(a.hit_rate * 100).toFixed(0)}%</td></tr>}
                </For>
              </tbody>
            </table>
          </div>
        </div>
      </Show>
    </div>
  );
}

function CorrHeatmap(props: { matrix: Record<string, Record<string, number>> }) {
  const keys = () => Object.keys(props.matrix);
  const cell = (v: number) => {
    const a = Math.min(1, Math.abs(v));
    const bg = v >= 0 ? `rgba(63,185,80,${(0.15 + 0.65 * a).toFixed(2)})` : `rgba(248,81,73,${(0.15 + 0.65 * a).toFixed(2)})`;
    return bg;
  };
  return (
    <Show when={keys().length} fallback={<p class="fs-muted">No correlation data yet.</p>}>
      <div class="fs-scroll"><table class="fs-table">
        <thead><tr><th></th><For each={keys()}>{(k) => <th>{k}</th>}</For></tr></thead>
        <tbody>
          <For each={keys()}>
            {(r) => (
              <tr>
                <td><strong>{r}</strong></td>
                <For each={keys()}>
                  {(c) => {
                    const v = props.matrix[r]?.[c] ?? 0;
                    return <td style={{ background: cell(v) }} title={`${r}/${c} = ${v.toFixed(2)}`}>{v.toFixed(2)}</td>;
                  }}
                </For>
              </tr>
            )}
          </For>
        </tbody>
      </table></div>
    </Show>
  );
}
