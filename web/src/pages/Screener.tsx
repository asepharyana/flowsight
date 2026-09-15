import { createSignal, For, Show } from "solid-js";
import { api, type ScreenRow } from "../lib/api";
import { verdictFor } from "../lib/awam";
import { Term, VerdictBadge, AlasanBar, IstilahStrip } from "../components/Awam";
import { Badge } from "../components/ui/badge";
import { Button } from "../components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "../components/ui/card";
import { Skeleton } from "../components/ui/skeleton";
import { useNavigate } from "@solidjs/router";

// Preset awam -> body /api/screen.
const PRESETS: { label: string; desc: string; body: Record<string, unknown> }[] = [
  { label: "🔥 Yang lagi diborong bandar", desc: "Broker besar net-beli besar", body: { institutional: { broker_score_min: 5 }, limit: 20 } },
  { label: "🌍 Yang asing lagi beli", desc: "Uang luar negeri masuk", body: { institutional: { foreign_inflow: true }, limit: 20 } },
  { label: "🕵️ Yang orang dalamnya ikut beli", desc: "Insider buying terdeteksi", body: { institutional: { insider_buying: true }, limit: 20 } },
  { label: "📦 Semua — urut paling menarik", desc: "Tanpa filter, ranking gabungan", body: { limit: 20 } },
];

export default function Screener() {
  const navigate = useNavigate();
  const [rows, setRows] = createSignal<ScreenRow[]>([]);
  const [ran, setRan] = createSignal(false);
  const [busy, setBusy] = createSignal(false);
  const [err, setErr] = createSignal("");
  const [aktif, setAktif] = createSignal("");
  async function runPreset(p: (typeof PRESETS)[number]) {
    setErr(""); setBusy(true); setAktif(p.label);
    try {
      const r = await api.screen(p.body);
      setRows(r.rows); setRan(true);
    } catch (e) { setErr(String(e)); }
    finally { setBusy(false); }
  }
  const hasil = () => rows().map((r) => ({ row: r, ...verdictFor(r) }));
  return (
    <div class="space-y-4">
      <div>
        <h1 class="text-2xl font-bold tracking-tight">Cari saham menarik 🔍</h1>
        <p class="text-sm text-muted-foreground">Pilih satu kategori — sistem yang menyaring <Term kata="screener" /> buat kamu.</p>
      </div>
      <div class="grid gap-3 md:grid-cols-2">
        <For each={PRESETS}>{(p) => (
          <Card class={aktif() === p.label ? "border-primary" : ""}>
            <CardHeader class="pb-2"><CardTitle class="text-base">{p.label}</CardTitle><CardDescription>{p.desc}</CardDescription></CardHeader>
            <CardContent><Button size="sm" onClick={() => runPreset(p)} disabled={busy()}>{busy() && aktif() === p.label ? "Menyaring…" : "Tampilkan"}</Button></CardContent>
          </Card>
        )}</For>
      </div>
      <Show when={err()}><p class="text-sm text-destructive">{err()}</p></Show>
      <Show when={busy()}><div class="space-y-2"><Skeleton class="h-24 w-full" /><Skeleton class="h-24 w-full" /></div></Show>
      <Show when={ran() && !busy()}>
        <Show when={hasil().length} fallback={<p class="text-sm text-muted-foreground">Tidak ada yang cocok — coba kategori lain.</p>}>
          <h2 class="text-lg font-semibold">Hasil ({hasil().length})</h2>
          <div class="grid gap-4 md:grid-cols-2">
            <For each={hasil()}>{(s) => {
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
                      <Button size="sm" onClick={() => navigate(`/report/${s.row.symbol}`)}>Kenapa? Jelaskan</Button>
                    </div>
                  </CardContent>
                </Card>
              );
            }}</For>
          </div>
        </Show>
      </Show>
      <div><IstilahStrip /></div>
    </div>
  );
}
