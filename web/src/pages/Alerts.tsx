import { createResource, createSignal, For, Show } from "solid-js";
import { api } from "../lib/api";
import { EmptyState, PageHead } from "../components/ui";
import { Badge } from "../components/ui/badge";
import { Button } from "../components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "../components/ui/card";
import { TextField, TextFieldInput } from "../components/ui/text-field";

const FIELDS = ["score", "broker_score", "foreign_net", "volume_mult", "composite"];
const OPS = [">=", "<=", ">", "<", "=="];

type Cond = { field: string; op: string; value: number };

export default function Alerts() {
  const [alerts, { refetch }] = createResource(() => api.alerts());
  const [events, { refetch: refetchEv }] = createResource(() => api.alertEvents("2000-01-01").then((r) => r.events.slice(0, 30)));
  const [dests, { refetch: refetchDests }] = createResource(() => api.destinations());
  const [name, setName] = createSignal("accumulation watcher");
  const [field, setField] = createSignal("score");
  const [op, setOp] = createSignal(">=");
  const [value, setValue] = createSignal(60);
  const [conds, setConds] = createSignal<Cond[]>([]);
  const [channels, setChannels] = createSignal("");
  const [busy, setBusy] = createSignal(false);
  const [dkind, setDkind] = createSignal("telegram");
  const [dlabel, setDlabel] = createSignal("");
  const [dbot, setDbot] = createSignal("");
  const [dchat, setDchat] = createSignal("");
  const [dhook, setDhook] = createSignal("");
  const [derr, setDerr] = createSignal("");
  const rulePreview = () => JSON.stringify(conds().length ? { all: conds() } : { all: true });
  function addCond() {
    setConds((p) => [...p, { field: field(), op: op(), value: value() }]);
  }
  async function create() {
    setBusy(true);
    try {
      await api.createAlert({
        name: name(),
        rule: JSON.parse(rulePreview()),
        channels: channels().split(",").map((c) => c.trim()).filter(Boolean),
      });
      setConds([]);
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
  async function toggleDest(id: number, enabled: boolean) {
    await api.updateDestination(id, { enabled: !enabled });
    refetchDests();
  }
  async function delDest(id: number) { await api.deleteDestination(id); refetchDests(); }
  return (
    <div class="space-y-4">
      <PageHead title="Alerts" sub="Rules evaluated over snapshots each cycle." />
      <Card>
        <CardHeader>
          <CardTitle>Push destinations (per login)</CardTitle>
          <CardDescription>Secrets stay server-side — the list never shows tokens.</CardDescription>
        </CardHeader>
        <CardContent class="space-y-3">
          <div class="flex flex-wrap gap-2">
            <div class="flex gap-1 rounded-md bg-muted p-1">
              <Button size="sm" variant={dkind() === "telegram" ? "default" : "ghost"} onClick={() => setDkind("telegram")}>Telegram</Button>
              <Button size="sm" variant={dkind() === "discord" ? "default" : "ghost"} onClick={() => setDkind("discord")}>Discord</Button>
            </div>
            <TextField class="w-40"><TextFieldInput placeholder="label (e.g. my phone)" value={dlabel()} onInput={(e) => setDlabel(e.currentTarget.value)} /></TextField>
            {dkind() === "telegram" ? (
              <>
                <TextField class="w-52"><TextFieldInput placeholder="bot token" value={dbot()} onInput={(e) => setDbot(e.currentTarget.value)} /></TextField>
                <TextField class="w-32"><TextFieldInput placeholder="chat id" value={dchat()} onInput={(e) => setDchat(e.currentTarget.value)} /></TextField>
              </>
            ) : (
              <TextField class="min-w-64 flex-1"><TextFieldInput placeholder="https://discord webhook url" value={dhook()} onInput={(e) => setDhook(e.currentTarget.value)} /></TextField>
            )}
            <Button onClick={addDest}>Add destination</Button>
          </div>
          <Show when={derr()}><p class="text-sm text-destructive">{derr()}</p></Show>
          <ul class="space-y-1.5 text-sm">
            <For each={(dests()?.destinations || [])}>
              {(d) => (
                <li class="flex flex-wrap items-center gap-2">
                  <Badge variant={d.enabled ? "success" : "secondary"}>{d.kind}</Badge>
                  <span>{d.label || `#${d.id}`}</span>
                  <span class="text-muted-foreground">{d.enabled ? "on" : "off"} · {d.configured ? "configured" : "missing secret"}</span>
                  <span class="flex-1" />
                  <Button variant="outline" size="sm" onClick={() => toggleDest(d.id, d.enabled)}>{d.enabled ? "Disable" : "Enable"}</Button>
                  <Button variant="destructive" size="sm" onClick={() => delDest(d.id)}>Delete</Button>
                </li>
              )}
            </For>
          </ul>
        </CardContent>
      </Card>
      <Card>
        <CardHeader><CardTitle>New rule</CardTitle></CardHeader>
        <CardContent class="space-y-3">
          <TextField><TextFieldInput placeholder="rule name" value={name()} onInput={(e) => setName(e.currentTarget.value)} /></TextField>
          <div class="flex flex-wrap gap-2">
            <select class="h-10 rounded-md border border-input bg-background px-3 text-sm" value={field()} onChange={(e) => setField(e.currentTarget.value)}>
              <For each={FIELDS}>{(f) => <option value={f}>{f}</option>}</For>
            </select>
            <select class="h-10 rounded-md border border-input bg-background px-3 text-sm" value={op()} onChange={(e) => setOp(e.currentTarget.value)}>
              <For each={OPS}>{(o) => <option value={o}>{o}</option>}</For>
            </select>
            <TextField class="w-24"><TextFieldInput type="number" value={value()} onInput={(e) => setValue(Number(e.currentTarget.value))} /></TextField>
            <Button variant="outline" onClick={addCond}>+ condition</Button>
          </div>
          <Show when={conds().length}>
            <ul class="space-y-1 text-sm text-muted-foreground"><For each={conds()}>{(c, i) => <li>{c.field} {c.op} {c.value} <Button variant="ghost" size="sm" onClick={() => setConds((p) => p.filter((_, j) => j !== i()))}>×</Button></li>}</For></ul>
          </Show>
          <p class="text-sm text-muted-foreground">Rule JSON: <code class="font-mono">{rulePreview()}</code></p>
          <div class="flex gap-2">
            <TextField class="flex-1"><TextFieldInput placeholder="channels, comma-separated (blank = record only)" value={channels()} onInput={(e) => setChannels(e.currentTarget.value)} /></TextField>
            <Button onClick={create} disabled={busy()}>Create rule</Button>
          </div>
        </CardContent>
      </Card>
      <div class="grid gap-4 md:grid-cols-2">
        <Card>
          <CardHeader><CardTitle>Rules</CardTitle></CardHeader>
          <CardContent>
            <Show when={(alerts()?.alerts || []).length} fallback={<EmptyState icon="🔔" title="No rules" hint="Create one above." />}>
              <ul class="space-y-1.5 text-sm"><For each={alerts()?.alerts || []}>{(a) => <li class="flex items-center gap-2">{a.name} <span class="text-muted-foreground">last: {a.last_fired || "never"}</span><span class="flex-1" /><Button variant="destructive" size="sm" onClick={() => del(a.id)}>Delete</Button></li>}</For></ul>
            </Show>
          </CardContent>
        </Card>
        <Card>
          <CardHeader class="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle>Event history</CardTitle>
            <Button variant="ghost" size="sm" onClick={() => refetchEv()}>Refresh</Button>
          </CardHeader>
          <CardContent>
            <Show when={(events() || []).length} fallback={<p class="text-sm text-muted-foreground">No events yet.</p>}>
              <ul class="space-y-1.5 text-sm"><For each={events() || []}>{(e) => <li><Badge variant="secondary">{String(e.ticker)}</Badge> {String(e.message)}</li>}</For></ul>
            </Show>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
