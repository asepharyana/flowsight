import { createSignal, For, Show, onMount } from "solid-js";
import { api, type ScreenRow } from "../lib/api";
import { verdictFor } from "../lib/awam";
import { Term, VerdictBadge, AlasanBar, IstilahStrip } from "../components/Awam";
import { Badge } from "../components/ui/badge";
import { Button } from "../components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "../components/ui/card";
import { Skeleton } from "../components/ui/skeleton";
import { TextField, TextFieldInput } from "../components/ui/text-field";
import { useNavigate } from "@solidjs/router";

// Preset awam -> body /api/screen. PRESET_SEMUA = default auto-jalan.
export const PRESET_SEMUA = { label: "📦 Semua — urut paling menarik", desc: "Tanpa filter, ranking gabungan", body: { limit: 20 } };
const PRESETS: { label: string; desc: string; body: Record<string, unknown> }[] = [
  { label: "🔥 Yang lagi diborong bandar", desc: "Broker besar net-beli besar", body: { institutional: { broker_score_min: 5 }, limit: 20 } },
  { label: "🌍 Yang asing lagi beli", desc: "Uang luar negeri masuk", body: { institutional: { foreign_inflow: true }, limit: 20 } },
  { label: "🕵️ Yang orang dalamnya ikut beli", desc: "Insider buying terdeteksi", body: { institutional: { insider_buying: true }, limit: 20 } },
  PRESET_SEMUA,
];

function ScreenerInner() {
  const navigate = useNavigate();
  const [rows, setRows] = createSignal<ScreenRow[]>([]);
  const [ran, setRan] = createSignal(false);
  const [busy, setBusy] = createSignal(false);
  const [err, setErr] = createSignal("");
  const [aktif, setAktif] = createSignal("");
  const [q, setQ] = createSignal("");
  const [showCustom, setShowCustom] = createSignal(false);
  async function runPreset(p: (typeof PRESETS)[number]) {
    setErr(""); setBusy(true); setAktif(p.label); setShowCustom(false);
    try {
      const r = await api.screen(p.body);
      setRows(r.rows); setRan(true);
    } catch (e) { setErr(String(e)); }
    finally { setBusy(false); }
  }
  async function runCustom() {
    const query = q().trim();
    if (!query) return;
    setErr(""); setBusy(true); setAktif("custom");
    try {
      const r = await api.screen({ q: query, limit: 20 });
      setRows(r.rows); setRan(true);
    } catch (e) { setErr(String(e)); }
    finally { setBusy(false); }
  }
  // Default = semua: auto-jalan preset "Semua" sekali saat halaman dibuka.
  onMount(() => { if (!ran()) void runPreset(PRESET_SEMUA); });
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
        <Card class={showCustom() ? "border-primary" : ""}>
          <CardHeader class="pb-2"><CardTitle class="text-base">🔎 Cari nama / sektor sendiri</CardTitle><CardDescription>Ketik nama perusahaan atau sektor (mis. "bank", "energi").</CardDescription></CardHeader>
          <CardContent class="flex gap-2">
            <TextField class="flex-1"><TextFieldInput placeholder="mis. bank, bca, energi…" value={q()} onInput={(e) => { setQ(e.currentTarget.value); setShowCustom(true); }} onKeyDown={(e: KeyboardEvent) => { if (e.key === "Enter") runCustom(); }} /></TextField>
            <Button size="sm" onClick={runCustom} disabled={busy() || !q().trim()}>{busy() && aktif() === "custom" ? "Menyaring…" : "Cari"}</Button>
          </CardContent>
        </Card>
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

import { Gate } from "../index";

export default function Screener() {
  return <Gate fitur="Cari saham"><ScreenerInner /></Gate>;
}
