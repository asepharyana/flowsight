import { createResource, createSignal, createMemo, For, Show } from "solid-js";
import { A, useNavigate } from "@solidjs/router";
import { api } from "../lib/api";
import { useAuth } from "../components/auth";
import { verdictFor, heroSummary, konteksPasar, fmtRp, fmtHarga } from "../lib/awam";
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
  const { me } = useAuth();
  // screen+alertEvents are login-gated; source = me() so login/logout refetches.
  const [flow] = createResource(() => api.flowSummary());
  const [screen] = createResource(me, (user) => (user ? api.screen({ limit: 5 }).catch(() => null) : null));
  const [briefing] = createResource(() => api.briefing().catch(() => null));
  const [events] = createResource(me, (user) => (user ? api.alertEvents("2000-01-01").then((r) => r.events.slice(0, 5)).catch(() => []) : []));
  const [chartTiker, setChartTiker] = createSignal("BBCA");
  const [foreign] = createResource(chartTiker, (t) => api.flowForeign(t).catch(() => null));

  const ringkasan = () => {
    const n = briefing()?.narasi?.trim();
    if (n) return n.split("\n").map((s) => s.replace(/^[-•\d.]+\s*/, "")).filter(Boolean).slice(0, 3);
    const p = briefing()?.payload;
    return p ? heroSummary(p) : [];
  };
  const sorotan = () => (screen()?.rows || []).map((r) => ({ row: r, ...verdictFor(r) }));
  const closeMap = () => Object.fromEntries((flow()?.closes || []).map((c) => [c.ticker, c]));
  const chartData = () => ({
    labels: (foreign()?.dates || []) as string[],
    datasets: [{
      label: `${chartTiker()} — uang asing harian (Rp juta)`,
      data: (foreign()?.nets || []) as number[],
      borderColor: "#3b82f6",
      backgroundColor: "rgba(59,130,246,.15)",
      fill: true,
      tension: 0.25,
    }],
  });
  const chartOpts = () => ({
    responsive: true,
    plugins: {
      tooltip: { callbacks: { label: (c: { parsed: { y: number } }) => ` ${fmtRp(c.parsed.y)}` } },
      legend: { display: false },
    },
    scales: {
      y: { ticks: { callback: (v: number | string) => fmtRp(Number(v)), maxTicksLimit: 5 } },
    },
  });
  const periode = () => {
    const f = foreign();
    return f?.start && f?.end ? `${f.start} → ${f.end}` : "";
  };

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
          <Show when={sorotan().length}>
            <p class="mt-3 border-t pt-3 text-sm text-muted-foreground">🧭 {konteksPasar(sorotan(), flow()?.foreign_net_total)}</p>
          </Show>
          <div class="mt-3"><IstilahStrip /></div>
        </CardContent>
      </Card>

      {/* SOROTAN SAHAM */}
      <div>
        <h2 class="mb-3 text-lg font-semibold">Saham sorotan hari ini</h2>
        <Show when={!screen.loading} fallback={<div class="grid gap-4 md:grid-cols-2"><Skeleton class="h-44" /><Skeleton class="h-44" /></div>}>
          <Show when={sorotan().length} fallback={<p class="text-sm text-muted-foreground">Belum ada data sorotan.</p>}>
            <div class="grid gap-4 md:grid-cols-2">
              <For each={sorotan()}>{(s) => {
                const b = (s.row.breakdown || {}) as Record<string, unknown>;
                const cx = closeMap()[s.row.symbol];
                return (
                  <Card class={s.verdict === "Dilirik" ? "border-emerald-500/50" : s.verdict === "Dilepas" ? "border-red-500/50" : ""}>
                    <CardHeader class="flex flex-row items-center justify-between space-y-0 pb-2">
                      <CardTitle class="text-xl font-bold">
                        {s.row.symbol}
                        <Show when={cx?.close}><span class="ml-2 font-mono text-sm font-normal text-muted-foreground">{fmtHarga(cx.close)}</span></Show>
                      </CardTitle>
                      <VerdictBadge verdict={s.verdict} />
                    </CardHeader>
                    <CardContent class="space-y-3">
                      <p class="text-sm">
                        Harga terakhir <strong class="font-mono">{cx?.close ? fmtHarga(cx.close) : "—"}</strong>
                        {" "}— {s.alasan}.
                      </p>
                      <AlasanBar asing={Number(b.foreign ?? 0)} broker={Number(b.broker_score ?? b.broker ?? 0)} />
                      <div class="flex gap-2">
                        <Button size="sm" onClick={() => navigate(`/report/${s.row.symbol}`)}>Kenapa? Jelaskan</Button>
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
            <Line data={chartData()} options={chartOpts() as never} />
            <p class="mt-1 text-xs text-muted-foreground">Periode {periode()} · sumber: Sectors API (lihat sitasi report).</p>
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
          <CardHeader><CardTitle>Yang paling diborong</CardTitle><CardDescription>Total <Term kata="net" /> beli broker 5 hari: <strong>{flow() ? fmtRp(flow()!.foreign_net_total) : "…"}</strong></CardDescription></CardHeader>
          <CardContent>
            <ul class="space-y-1 text-sm">
              <For each={flow()?.top_accumulation || []}>{(t) => <li class="flex justify-between"><A class="font-medium underline-offset-2 hover:underline" href={`/report/${t.ticker}`}>{t.ticker}</A><span class="font-mono">{fmtRp(t.net_sum)} · {t.brokers} broker</span></li>}</For>
            </ul>
            <div class="mt-2"><Badge variant="outline">Angka detail untuk yang penasaran — keputusan ada di kartu sorotan ☝️</Badge></div>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
