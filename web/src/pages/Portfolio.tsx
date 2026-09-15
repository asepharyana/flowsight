import { createResource, For, Show } from "solid-js";
import { api } from "../lib/api";
import { Term, IstilahStrip } from "../components/Awam";
import { Bar } from "solid-chartjs";
import { Chart, registerables } from "chart.js";
import { PageHead } from "../components/ui";
import { Badge } from "../components/ui/badge";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "../components/ui/card";
import { Skeleton } from "../components/ui/skeleton";
Chart.register(...registerables);

function PortfolioInner() {
  const [risk] = createResource(() => api.risk());
  const conc = () => risk()?.concentration || [];
  const betaN = () => Number(risk()?.beta ?? 1);
  const betaArti = () => {
    const b = betaN();
    if (b > 1.2) return "bergerak lebih liar dari pasar — naik lebih kencang, turun lebih dalam.";
    if (b < 0.8) return "lebih kalem dari pasar — cocok buat yang tidak suka deg-degan.";
    return "geraknya seirama pasar — tidak terlalu liar, tidak terlalu kalem.";
  };
  const saran = () => {
    const c = conc();
    if (!c.length) return "";
    // Sektor dulu (risiko sistemik) baru bobot single-ticker.
    const perSek: Record<string, number> = {};
    for (const x of c) perSek[x.sector] = (perSek[x.sector] || 0) + x.weight;
    const [sekTop, wSek] = Object.entries(perSek).sort((a, b) => b[1] - a[1])[0];
    if (wSek > 0.4) return `Sektor ${sekTop} porsinya ${(wSek * 100).toFixed(0)}% — kalau sektor itu jatuh, semua ikut. Coba lirik sektor lain.`;
    const top = c[0];
    if (top.weight > 0.4) return `${top.ticker} porsinya ${(top.weight * 100).toFixed(0)}% — kebanyakan telur di satu keranjang. Pertimbangkan tambah saham beda sektor.`;
    return "Sebarannya sudah lumayan — tidak numpuk di satu tempat. Pertahankan. 👍";
  };
  const chartData = () => ({
    labels: conc().map((c) => c.ticker),
    datasets: [{ label: "Porsi %", data: conc().map((c) => +(c.weight * 100).toFixed(1)) }],
  });
  return (
    <div class="space-y-4">
      <PageHead title="Risiko portofolio kamu 💼" sub="Dihitung dari watchlist-mu. Bahasa manusia, bukan laporan bank." />
      <Show when={!risk.loading} fallback={<div class="space-y-2"><Skeleton class="h-24 w-full" /><Skeleton class="h-24 w-full" /></div>}>
        <Card class="border-primary/30 bg-gradient-to-br from-primary/10 to-transparent">
          <CardHeader><CardTitle class="text-xl">Saran 1 kalimat 💡</CardTitle></CardHeader>
          <CardContent><p class="text-base leading-relaxed">{saran() || "Tambah saham ke watchlist dulu biar bisa dinilai."}</p></CardContent>
        </Card>
        <div class="grid gap-4 md:grid-cols-2">
          <Card>
            <CardHeader><CardTitle>Porsi tiap saham</CardTitle><CardDescription>Siapa yang paling dominan di <Term kata="portofolio" />-mu.</CardDescription></CardHeader>
            <CardContent>
              <Show when={conc().length} fallback={<p class="text-sm text-muted-foreground">Tambah saham ke watchlist dulu.</p>}>
                <Bar data={chartData()} options={{ responsive: true }} />
              </Show>
              <For each={risk()?.warnings || []}>{(w) => <p class="mt-1 text-sm">⚠️ {w}</p>}</For>
            </CardContent>
          </Card>
          <Card>
            <CardHeader><CardTitle>Kelakuan vs pasar</CardTitle><CardDescription>Skor beta = {betaN().toFixed(2)}</CardDescription></CardHeader>
            <CardContent class="space-y-3">
              <p class="text-base">Portofolio kamu <strong>{betaArti()}</strong></p>
              <div><Badge variant="outline"><Term kata="diversifikasi" />: {[...new Set(conc().map((x) => x.sector))].length} sektor</Badge></div>
            </CardContent>
          </Card>
        </div>
      </Show>
      <div><IstilahStrip /></div>
    </div>
  );
}

import { Gate } from "../index";

export default function Portfolio() {
  return <Gate fitur="Portofolio">{PortfolioInner()}</Gate>;
}
