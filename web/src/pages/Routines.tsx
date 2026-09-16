import { createResource, createSignal, For, Show } from "solid-js";
import { api } from "../lib/api";
import { Term, IstilahStrip } from "../components/Awam";
import { EmptyState, PageHead } from "../components/ui";
import { Badge } from "../components/ui/badge";
import { Button } from "../components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "../components/ui/card";

// Tipe -> penjelasan manfaat bahasa awam.
const MANFAAT: Record<string, { judul: string; desc: string }> = {
  "morning-briefing": { judul: "☀️ Ringkasan pagi", desc: "Dapat kabar pasar tiap pagi jam 07:30 — siapa diborong, ke mana uang asing." },
  "accumulation-radar": { judul: "🐋 Radar borongan bandar", desc: "Dikabari kalau broker besar mulai borong saham." },
  "foreign-reversal": { judul: "🌍 Pantau balik arah asing", desc: "Dikabari kalau uang asing berbalik arah (masuk ↔ keluar)." },
  "insider-tape": { judul: "🕵️ Gerak orang dalam", desc: "Dikabari kalau direksi/pemilik saham ikut beli." },
  "earnings-countdown": { judul: "📊 Pengingat laporan keuangan", desc: "Dikabari sebelum emiten rilis laporan keuangan." },
  "dividend-calendar": { judul: "💰 Pengingat dividen", desc: "Dikabari sebelum tanggal bagi dividen (biar tidak kelewat)." },
  "weekend-review": { judul: "📝 Review mingguan", desc: "Ringkasan seminggu: apa yang terjadi dan pelajaran." },
};

function RoutinesInner() {
  const [data, { refetch }] = createResource(() => api.routines());
  const [runs, { refetch: refetchRuns }] = createResource(() => api.runs().then((r) => r.runs.slice(0, 20)));
  const [type_, setType] = createSignal("morning-briefing");
  const [busy, setBusy] = createSignal(false);
  const [err, setErr] = createSignal("");
  const [briefing] = createResource(() => api.briefing().catch(() => null));
  async function subscribe() {
    setBusy(true);
    try { await api.createRoutine({ type: type_() }); refetch(); }
    catch (e) { setErr(String(e)); }
    finally { setBusy(false); }
  }
  const fmtTime = (s: unknown) => {
    const t = String(s || "");
    if (!t) return "—";
    // ISO timestamp → "16 Sep 07:30" (local time).
    const d = new Date(t);
    return Number.isNaN(d.getTime()) ? t : d.toLocaleString("id-ID", { day: "numeric", month: "short", hour: "2-digit", minute: "2-digit" });
  };
  async function toggle(id: number, enabled: boolean) {
    try { await api.updateRoutine(id, { enabled: !enabled }); refetch(); } catch (e) { setErr(String(e)); }
  }
  async function del(id: number) {
    try { await api.deleteRoutine(id); refetch(); } catch (e) { setErr(String(e)); }
  }
  return (
    <div class="space-y-4">
      <PageHead title="Jadwal otomatis 🗓️" sub="Pilih sekali — sistem yang kerja tiap hari. Ini namanya routine." />
      <Show when={err()}><p class="text-sm text-destructive">{err()}</p></Show>
      <Card>
        <CardHeader><CardTitle>Mau dilayani apa?</CardTitle><CardDescription>Jadwal standar sudah diatur (mis. ringkasan jam 07:30) — tidak perlu isi cron.</CardDescription></CardHeader>
        <CardContent class="space-y-3">
          <div class="grid gap-2 md:grid-cols-2">
            <For each={Object.entries(MANFAAT)}>{([tipe, m]) => (
              <Button variant={type_() === tipe ? "default" : "outline"} class="h-auto flex-col items-start p-3" onClick={() => setType(tipe)}>
                <span class="font-semibold">{m.judul}</span><span class="text-xs font-normal opacity-80">{m.desc}</span>
              </Button>
            )}</For>
          </div>
          <Button onClick={subscribe} disabled={busy()}>Aktifkan ✓</Button>
        </CardContent>
      </Card>
      <Show when={(data()?.routines || []).length} fallback={
        <EmptyState icon="🗓️" title="Belum ada jadwal" hint="Pilih layanan di atas — sekali klik langsung jalan." />
      }>
        <div class="grid gap-4 md:grid-cols-2">
          <For each={data()?.routines || []}>
            {(r) => (
              <Card>
                <CardHeader class="flex flex-row items-center justify-between space-y-0 pb-2">
                  <CardTitle class="text-base">{MANFAAT[r.type]?.judul || r.type}</CardTitle>
                  <Badge variant={r.enabled ? "success" : "secondary"}>{r.enabled ? "jalan" : "mati"}</Badge>
                </CardHeader>
                <CardContent class="space-y-2">
                  <p class="text-sm text-muted-foreground">{MANFAAT[r.type]?.desc || ""}</p>
                  <div class="flex gap-2">
                    <Button variant="outline" size="sm" onClick={() => toggle(r.id, r.enabled)}>{r.enabled ? "Matikan" : "Nyalakan"}</Button>
                    <Button variant="destructive" size="sm" onClick={() => del(r.id)}>Hapus</Button>
                  </div>
                </CardContent>
              </Card>
            )}
          </For>
        </div>
      </Show>
      <div class="grid gap-4 md:grid-cols-2">
        <Card>
          <CardHeader><CardTitle>Ringkasan terakhir</CardTitle><CardDescription><Term kata="briefing" /> pagi yang sudah terbit.</CardDescription></CardHeader>
          <CardContent>
            <Show when={briefing()} fallback={<p class="text-sm text-muted-foreground">Belum terbit — aktifkan "Ringkasan pagi" di atas.</p>}>
              <Show when={(briefing() as { narasi?: string })?.narasi} fallback={<pre class="whitespace-pre-wrap text-sm text-muted-foreground">{(briefing() as { payload: string })?.payload?.slice(0, 600)}</pre>}>
                <p class="whitespace-pre-wrap text-sm leading-relaxed">{(briefing() as { narasi?: string })?.narasi}</p>
              </Show>
            </Show>
          </CardContent>
        </Card>
        <Card>
          <CardHeader class="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle>Riwayat jalan</CardTitle>
            <Button variant="ghost" size="sm" onClick={() => refetchRuns()}>Refresh</Button>
          </CardHeader>
          <CardContent>
            <ul class="space-y-1 text-sm text-muted-foreground">
              <For each={runs() || []}>{(r) => (
                <li class="flex items-center justify-between gap-2">
                  <span class="flex items-center gap-2">
                    <Badge variant={r.status === "ok" ? "success" : r.status === "error" ? "error" : "secondary"} class="capitalize">{String(r.status)}</Badge>
                    <span>{fmtTime(r.started_at)}</span>
                  </span>
                  <span class="font-mono text-xs">{r.credits_used ? `${String(r.credits_used)} kredit` : `#${String(r.routine_id)}`}</span>
                </li>
              )}</For>
            </ul>
          </CardContent>
        </Card>
      </div>
      <div><IstilahStrip /></div>
    </div>
  );
}

import { Gate } from "../index";

export default function Routines() {
  return <Gate fitur="Jadwal otomatis">{RoutinesInner()}</Gate>;
}
