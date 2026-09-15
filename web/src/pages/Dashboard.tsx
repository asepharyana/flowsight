import { createResource, createSignal, For, Show } from "solid-js";
import { A, useNavigate } from "@solidjs/router";
import { api, fmtIDR } from "../lib/api";
import { verdictFor, heroSummary } from "../lib/awam";
import { Term, VerdictBadge, AlasanBar, IstilahStrip } from "../components/Awam";
import { Badge } from "../components/ui/badge";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "../components/ui/card";
import { Button } from "../components/ui/button";
import { Skeleton } from "../components/ui/skeleton";
import { Line } from "solid-chartjs";
import { Chart, registerables } from "chart.js";
Chart.register(...registerables);

export default function Dashboard() {
  const navigate = useNavigate();
  const [flow] = createResource(() => api.flowSummary());
  const [screen] = createResource(() => api.screen({ limit: 5 }).catch(() => null));
  const [briefing] = createResource(() => api.briefing().catch(() => null));
  const [events] = createResource(() => api.alertEvents("2000-01-01").then((r) => r.events.slice(0, 5)).catch(() => []));
  const [chartTiker, setChartTiker] = createSignal("BBCA");
  const [foreign] = createResource(chartTiker, (t) => api.flowForeign(t).catch(() => null));

  const ringkasan = () => {
    const p = (briefing() as { payload: string } | null)?.payload;
    return p ? heroSummary(p) : [];
  };
  const sorotan = () => (screen()?.rows || []).map((r) => ({ row: r, ...verdictFor(r) }));
  const chartData = () => ({
    labels: (foreign()?.dates || []) as string[],
    datasets: [{ label: `${chartTiker()} — uang asing harian`, data: (foreign()?.nets || []) as number[] }],
  });

  return (
    <div class="space-y-6">
      {/* HERO: hari ini ada apa */}
      <Card class="border-primary/30 bg-gradient-to-br from-primary/10 to-transparent">
        <CardHeader>
          <CardTitle class="text-2xl">Hari ini ada apa? 📈</CardTitle>
          <CardDescription>Ringkasan pasar pagi ini — tanpa perlu paham istilah ekonomi.</CardDescription>
        </CardHeader>
        <CardContent>
          <Show when={!briefing.loading} fallback={<div class="space-y-2"><Skeleton class="h-4 w-full" /><Skeleton class="h-4 w-3/4" /></div>}>
            <Show when={ringkasan().length} fallback={<p class="text-sm text-muted-foreground">Belum ada ringkasan hari ini — coba lagi setelah scheduler jalan.</p>}>
              <ul class="space-y-2 text-base leading-relaxed">
                <For each={ringkasan()}>{(k) => <li>• {k}</li>}</For>
              </ul>
            </Show>
          </Show>
          <div class="mt-3"><IstilahStrip /></div>
        </CardContent>
      </Card>

      {/* SOROTAN SAHAM */}
      <div>
        <h2 class="mb-3 text-lg font-semibold">Saham sorotan hari ini</h2>
        <Show when={!screen.loading} fallback={<div class="grid gap-4 md:grid-cols-2"><Skeleton class="h-40" /><Skeleton class="h-40" /></div>}>
          <Show when={sorotan().length} fallback={<p class="text-sm text-muted-foreground">Belum ada data sorotan.</p>}>
            <div class="grid gap-4 md:grid-cols-2">
              <For each={sorotan()}>{(s) => {
                const b = (s.row.breakdown || {}) as Record<string, unknown>;
                return (
                  <Card>
                    <CardHeader class="flex flex-row items-center justify-between space-y-0 pb-2">
                      <CardTitle class="text-xl font-bold">{s.row.symbol}</CardTitle>
                      <VerdictBadge verdict={s.verdict} />
                    </CardHeader>
                    <CardContent class="space-y-3">
                      <p class="text-sm">Karena: <strong>{s.alasan}</strong>.</p>
                      <AlasanBar asing={Number(b.foreign ?? 0)} broker={Number(b.broker_score ?? b.broker ?? 0)} />
                      <div class="flex gap-2">
                        <Button size="sm" onClick={() => navigate(`/report?t=${s.row.symbol}`)}>Kenapa? Jelaskan</Button>
                        <Button size="sm" variant="outline"><A href={`/screener`}>Bandingkan</A></Button>
                      </div>
                    </CardContent>
                  </Card>
                );
              }}</For>
            </div>
          </Show>
        </Show>
      </div>

      {/* UANG ASING PER SAHAM */}
      <Card>
        <CardHeader>
          <CardTitle>Uang asing lagi ke mana?</CardTitle>
          <CardDescription><Term kata="foreign flow" /> harian — naik = asing beli, turun = asing jual. Pilih saham:</CardDescription>
        </CardHeader>
        <CardContent>
          <div class="mb-3 flex flex-wrap gap-2">
            <For each={(flow()?.top_accumulation || []).map((t) => t.ticker)}>{(t) =>
              <Button size="sm" variant={chartTiker() === t ? "default" : "outline"} onClick={() => setChartTiker(t)}>{t}</Button>
            }</For>
          </div>
          <Show when={(foreign()?.dates || []).length} fallback={<p class="text-sm text-muted-foreground">Belum ada data grafik.</p>}>
            <Line data={chartData()} options={{ responsive: true }} />
            <Show when={foreign()?.reversal}><p class="mt-2 text-sm font-medium">⚠️ Arahnya baru berbalik — perhatikan beberapa hari ke depan.</p></Show>
          </Show>
        </CardContent>
      </Card>

      {/* AKTIVITAS + ANGKA MENTAH LIPAT */}
      <div class="grid gap-4 md:grid-cols-2">
        <Card>
          <CardHeader><CardTitle>Kabar terbaru</CardTitle><CardDescription><Term kata="alert" /> dan aktivitas sistem.</CardDescription></CardHeader>
          <CardContent>
            <Show when={(events() || []).length} fallback={<p class="text-sm text-muted-foreground">Belum ada kabar — pasang alert biar dapat notifikasi.</p>}>
              <ul class="space-y-2 text-sm">
                <For each={events() || []}>{(e) => <li>• <strong>{String(e.ticker || "")}</strong> — {String(e.message || "")} <span class="text-muted-foreground">({String(e.date || "")})</span></li>}</For>
              </ul>
            </Show>
            <div class="mt-3"><Button size="sm" variant="outline"><A href="/alerts">Atur notifikasi saya</A></Button></div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader><CardTitle>Yang paling diborong</CardTitle><CardDescription>Total <Term kata="net" /> beli broker hari ini: <strong>{flow() ? fmtIDR(flow()!.foreign_net_total) : "…"}</strong></CardDescription></CardHeader>
          <CardContent>
            <ul class="space-y-1 text-sm">
              <For each={flow()?.top_accumulation || []}>{(t) => <li class="flex justify-between"><A class="font-medium underline-offset-2 hover:underline" href={`/report?t=${t.ticker}`}>{t.ticker}</A><span class="font-mono">{fmtIDR(t.net_sum)} · {t.brokers} broker</span></li>}</For>
            </ul>
            <div class="mt-2"><Badge variant="outline">Angka detail untuk yang penasaran — keputusan ada di kartu sorotan ☝️</Badge></div>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
