import { createResource, createSignal, For, Show } from "solid-js";
import { api } from "../lib/api";
import { Term, IstilahStrip } from "../components/Awam";
import { EmptyState, PageHead } from "../components/ui";
import { Badge } from "../components/ui/badge";
import { Button } from "../components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "../components/ui/card";
import { TextField, TextFieldInput } from "../components/ui/text-field";

// Template siap pakai -> rule JSON backend.
const TEMPLATES: { label: string; desc: string; rule: Record<string, unknown> }[] = [
  { label: "🐋 Bandar mulai borong", desc: "Beri tahu kalau broker besar net-beli di saham watchlist", rule: { all: [{ field: "broker_score", op: ">=", value: 30 }] } },
  { label: "🌍 Asing masuk deras", desc: "Beri tahu kalau uang asing inflow besar", rule: { all: [{ field: "foreign_net", op: ">=", value: 50000000000 }] } },
  { label: "🚨 Bandar mulai jualan", desc: "Beri tahu kalau broker besar net-jual", rule: { all: [{ field: "broker_score", op: "<=", value: -30 }] } },
  { label: "📢 Semua gerakan mencolok", desc: "Skor gabungan di atas ambang", rule: { all: [{ field: "score", op: ">=", value: 60 }] } },
];

export default function Alerts() {
  const [alerts, { refetch }] = createResource(() => api.alerts());
  const [events, { refetch: refetchEv }] = createResource(() => api.alertEvents("2000-01-01").then((r) => r.events.slice(0, 30)));
  const [dests, { refetch: refetchDests }] = createResource(() => api.destinations());
  const [tpl, setTpl] = createSignal(TEMPLATES[0]);
  const [channels, setChannels] = createSignal("");
  const [busy, setBusy] = createSignal(false);
  const [dkind, setDkind] = createSignal("telegram");
  const [dlabel, setDlabel] = createSignal("");
  const [dbot, setDbot] = createSignal("");
  const [dchat, setDchat] = createSignal("");
  const [dhook, setDhook] = createSignal("");
  const [derr, setDerr] = createSignal("");
  async function create() {
    setBusy(true);
    try {
      await api.createAlert({ name: tpl().label, rule: tpl().rule, channels: channels().split(",").map((c) => c.trim()).filter(Boolean) });
      refetch();
    } finally { setBusy(false); }
  }
  async function del(id: number) { await api.deleteAlert(id); refetch(); }
  async function addDest() {
    setDerr("");
    try {
      const body: Record<string, unknown> = dkind() === "telegram"
        ? { kind: "telegram", label: dlabel().trim(), bot_token: dbot().trim(), chat_id: dchat().trim() }
        : { kind: "discord", label: dlabel().trim(), webhook_url: dhook().trim() };
      await api.createDestination(body);
      setDbot(""); setDchat(""); setDhook(""); setDlabel("");
      refetchDests();
    } catch (e) { setDerr(String(e)); }
  }
  async function toggleDest(id: number, enabled: boolean) { await api.updateDestination(id, { enabled: !enabled }); refetchDests(); }
  async function delDest(id: number) { await api.deleteDestination(id); refetchDests(); }
  return (
    <div class="space-y-4">
      <PageHead title="Notifikasi otomatis 🔔" sub="Pilih kejadian yang mau kamu dikabari — tanpa perlu paham angka." />
      <Card>
        <CardHeader><CardTitle>1. Mau dikabari soal apa?</CardTitle><CardDescription>Pilih satu template <Term kata="alert" />.</CardDescription></CardHeader>
        <CardContent class="space-y-3">
          <div class="grid gap-2 md:grid-cols-2">
            <For each={TEMPLATES}>{(t) => (
              <Button variant={tpl().label === t.label ? "default" : "outline"} class="h-auto flex-col items-start p-3" onClick={() => setTpl(t)}>
                <span class="font-semibold">{t.label}</span><span class="text-xs font-normal opacity-80">{t.desc}</span>
              </Button>
            )}</For>
          </div>
          <div class="flex gap-2">
            <TextField class="flex-1"><TextFieldInput placeholder="Kirim ke mana? (kosongkan = catat saja)" value={channels()} onInput={(e) => setChannels(e.currentTarget.value)} /></TextField>
            <Button onClick={create} disabled={busy()}>Pasang notifikasi</Button>
          </div>
        </CardContent>
      </Card>
      <Card>
        <CardHeader><CardTitle>2. Dikirim ke mana?</CardTitle><CardDescription>Telegram / Discord milikmu. Token tersimpan aman di server.</CardDescription></CardHeader>
        <CardContent class="space-y-3">
          <div class="flex flex-wrap gap-2">
            <div class="flex gap-1 rounded-md bg-muted p-1">
              <Button size="sm" variant={dkind() === "telegram" ? "default" : "ghost"} onClick={() => setDkind("telegram")}>Telegram</Button>
              <Button size="sm" variant={dkind() === "discord" ? "default" : "ghost"} onClick={() => setDkind("discord")}>Discord</Button>
            </div>
            <TextField class="w-40"><TextFieldInput placeholder="nama (mis. HP saya)" value={dlabel()} onInput={(e) => setDlabel(e.currentTarget.value)} /></TextField>
            {dkind() === "telegram" ? (
              <><TextField class="w-52"><TextFieldInput placeholder="token bot" value={dbot()} onInput={(e) => setDbot(e.currentTarget.value)} /></TextField>
              <TextField class="w-32"><TextFieldInput placeholder="chat id" value={dchat()} onInput={(e) => setDchat(e.currentTarget.value)} /></TextField></>
            ) : (
              <TextField class="min-w-64 flex-1"><TextFieldInput placeholder="https://discord webhook url" value={dhook()} onInput={(e) => setDhook(e.currentTarget.value)} /></TextField>
            )}
            <Button onClick={addDest}>Tambah tujuan</Button>
          </div>
          <Show when={derr()}><p class="text-sm text-destructive">{derr()}</p></Show>
          <ul class="space-y-1.5 text-sm">
            <For each={(dests()?.destinations || [])}>
              {(d) => (
                <li class="flex flex-wrap items-center gap-2">
                  <Badge variant={d.enabled ? "success" : "secondary"}>{d.kind}</Badge>
                  <span>{d.label || `#${d.id}`}</span>
                  <span class="text-muted-foreground">{d.enabled ? "aktif" : "mati"} · {d.configured ? "siap" : "belum ada token"}</span>
                  <span class="flex-1" />
                  <Button variant="outline" size="sm" onClick={() => toggleDest(d.id, d.enabled)}>{d.enabled ? "Matikan" : "Nyalakan"}</Button>
                  <Button variant="destructive" size="sm" onClick={() => delDest(d.id)}>Hapus</Button>
                </li>
              )}
            </For>
          </ul>
        </CardContent>
      </Card>
      <div class="grid gap-4 md:grid-cols-2">
        <Card>
          <CardHeader><CardTitle>Notifikasi terpasang</CardTitle></CardHeader>
          <CardContent>
            <Show when={(alerts()?.alerts || []).length} fallback={<EmptyState icon="🔔" title="Belum ada" hint="Pilih template di atas, sekali klik." />}>
              <ul class="space-y-1.5 text-sm"><For each={alerts()?.alerts || []}>{(a) => <li class="flex items-center gap-2">{a.name} <span class="text-muted-foreground">terakhir: {a.last_fired || "belum pernah"}</span><span class="flex-1" /><Button variant="destructive" size="sm" onClick={() => del(a.id)}>Hapus</Button></li>}</For></ul>
            </Show>
          </CardContent>
        </Card>
        <Card>
          <CardHeader class="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle>Kejadian terakhir</CardTitle>
            <Button variant="ghost" size="sm" onClick={() => refetchEv()}>Refresh</Button>
          </CardHeader>
          <CardContent>
            <Show when={(events() || []).length} fallback={<p class="text-sm text-muted-foreground">Belum ada kejadian.</p>}>
              <ul class="space-y-1.5 text-sm"><For each={events() || []}>{(e) => <li><Badge variant="secondary">{String(e.ticker)}</Badge> {String(e.message)}</li>}</For></ul>
            </Show>
          </CardContent>
        </Card>
      </div>
      <div><IstilahStrip /></div>
    </div>
  );
}
