import { createResource, createSignal, For, Show } from "solid-js";
import { api } from "../lib/api";
import { Citations } from "../components/Citations";
import { EmptyState, PageHead } from "../components/ui";
import { Badge } from "../components/ui/badge";
import { Button } from "../components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "../components/ui/card";
import { TextField, TextFieldInput } from "../components/ui/text-field";

const TYPES = ["morning-briefing", "accumulation-radar", "foreign-reversal", "insider-tape", "earnings-countdown", "dividend-calendar", "weekend-review"];

export default function Routines() {
  const [data, { refetch }] = createResource(() => api.routines());
  const [runs, { refetch: refetchRuns }] = createResource(() => api.runs().then((r) => r.runs.slice(0, 20)));
  const [type_, setType] = createSignal("morning-briefing");
  const [schedule, setSchedule] = createSignal("");
  const [channels, setChannels] = createSignal("");
  const [busy, setBusy] = createSignal(false);
  const [briefing] = createResource(() => api.briefing().catch(() => null));
  async function subscribe() {
    setBusy(true);
    try {
      await api.createRoutine({
        type: type_(),
        schedule_cron: schedule().trim() || undefined,
        channels: channels().split(",").map((c) => c.trim()).filter(Boolean),
      });
      refetch();
    } finally { setBusy(false); }
  }
  async function toggle(id: number, enabled: boolean) {
    await api.updateRoutine(id, { enabled: !enabled });
    refetch();
  }
  async function del(id: number) {
    await fetch(`/api/routines/${id}`, { method: "DELETE", headers: { "X-User-Key": localStorage.getItem("fs-key") || "demo" } });
    refetch();
  }
  return (
    <div class="space-y-4">
      <PageHead title="Routine Manager" sub="Scheduled deliveries with citations." />
      <Card>
        <CardHeader><CardTitle>Subscribe</CardTitle></CardHeader>
        <CardContent class="flex flex-wrap gap-2">
          <TextField class="w-52">
            <select class="h-10 w-full rounded-md border border-input bg-background px-3 text-sm" value={type_()} onChange={(e) => setType(e.currentTarget.value)}>
              <For each={TYPES}>{(t) => <option value={t}>{t}</option>}</For>
            </select>
          </TextField>
          <TextField class="w-60"><TextFieldInput placeholder="schedule cron (blank = default)" value={schedule()} onInput={(e) => setSchedule(e.currentTarget.value)} /></TextField>
          <TextField class="w-52"><TextFieldInput placeholder="channels, comma-separated" value={channels()} onInput={(e) => setChannels(e.currentTarget.value)} /></TextField>
          <Button onClick={subscribe} disabled={busy()}>Subscribe</Button>
        </CardContent>
      </Card>
      <Show when={(data()?.routines || []).length} fallback={
        <EmptyState icon="🗓️" title="No routines yet" hint="Subscribe above — the morning briefing runs 07:30 WIB daily." />
      }>
        <div class="grid gap-4 md:grid-cols-2">
          <For each={data()?.routines || []}>
            {(r) => (
              <Card>
                <CardHeader class="flex flex-row items-center justify-between space-y-0 pb-2">
                  <CardTitle class="text-base">{r.type}</CardTitle>
                  <Badge variant={r.enabled ? "success" : "secondary"}>{r.enabled ? "on" : "off"}</Badge>
                </CardHeader>
                <CardContent class="space-y-2">
                  <p class="font-mono text-xs text-muted-foreground">{r.schedule_cron}</p>
                  <div class="flex gap-2">
                    <Button variant="outline" size="sm" onClick={() => toggle(r.id, r.enabled)}>{r.enabled ? "Disable" : "Enable"}</Button>
                    <Button variant="destructive" size="sm" onClick={() => del(r.id)}>Delete</Button>
                  </div>
                </CardContent>
              </Card>
            )}
          </For>
        </div>
      </Show>
      <div class="grid gap-4 md:grid-cols-2">
        <Card>
          <CardHeader><CardTitle>Latest briefing</CardTitle></CardHeader>
          <CardContent>
            <Show when={briefing()} fallback={<p class="text-sm text-muted-foreground">No briefing yet — it generates after the first morning-briefing run.</p>}>
              <pre class="whitespace-pre-wrap text-sm text-muted-foreground">{(briefing() as { payload: string })?.payload?.slice(0, 800)}</pre>
              <div class="mt-2"><Citations items={(briefing() as { citations: string })?.citations} /></div>
            </Show>
          </CardContent>
        </Card>
        <Card>
          <CardHeader class="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle>Run history</CardTitle>
            <Button variant="ghost" size="sm" onClick={() => refetchRuns()}>Refresh</Button>
          </CardHeader>
          <CardContent>
            <ul class="space-y-1 text-sm text-muted-foreground"><For each={runs() || []}>{(r) => <li>{String(r.started_at)} — {String(r.status)}</li>}</For></ul>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
