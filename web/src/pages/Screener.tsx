import { createSignal, For, Show } from "solid-js";
import { api, type ScreenRow } from "../lib/api";
import { Citations } from "../components/Citations";
import { BreakdownBars, EmptyState, PageHead, RecBadge } from "../components/ui";
import { Badge } from "../components/ui/badge";
import { Button } from "../components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "../components/ui/card";
import { Skeleton } from "../components/ui/skeleton";
import { TextField, TextFieldInput } from "../components/ui/text-field";
import { Switch, SwitchControl, SwitchLabel, SwitchThumb } from "../components/ui/switch";

const SAVED_KEY = "fs-saved-screeners";

function scoreOf(r: ScreenRow): number {
  const b = r.breakdown as Record<string, unknown>;
  for (const k of ["composite", "score", "broker_score", "broker"]) {
    if (typeof b[k] === "number") return b[k] as number;
  }
  return r.composite || 0;
}

export default function Screener() {
  const [mode, setMode] = createSignal<"where" | "q">("q");
  const [q, setQ] = createSignal("");
  const [brokerMin, setBrokerMin] = createSignal(0);
  const [foreignOnly, setForeignOnly] = createSignal(false);
  const [insiderOnly, setInsiderOnly] = createSignal(false);
  const [rows, setRows] = createSignal<ScreenRow[]>([]);
  const [ran, setRan] = createSignal(false);
  const [busy, setBusy] = createSignal(false);
  const [err, setErr] = createSignal("");
  const loadSaved = (): Record<string, string> => {
    try { return JSON.parse(localStorage.getItem(SAVED_KEY) || "{}"); } catch { return {}; }
  };
  const [saved, setSaved] = createSignal<Record<string, string>>(loadSaved());
  const [sname, setSname] = createSignal("");
  async function run() {
    setErr(""); setBusy(true);
    try {
      const institutional: Record<string, unknown> = { broker_score_min: brokerMin() };
      if (foreignOnly()) institutional.foreign_inflow = true;
      if (insiderOnly()) institutional.insider_buying = true;
      const body = mode() === "q"
        ? { q: q() || undefined, institutional, limit: 20 }
        : { where: q() || undefined, institutional, limit: 20 };
      const r = await api.screen(body);
      setRows(r.rows); setRan(true);
    } catch (e) { setErr(String(e)); }
    finally { setBusy(false); }
  }
  function save() {
    const all = { ...saved(), [sname().trim() || q().slice(0, 24) || "untitled"]: q() };
    localStorage.setItem(SAVED_KEY, JSON.stringify(all));
    setSaved(all);
  }
  return (
    <div class="space-y-4">
      <PageHead title="Institutional Screener" sub="Rank tickers by smart-money signals." />
      <Card>
        <CardContent class="space-y-3 pt-6">
          <div class="flex flex-wrap items-center gap-2">
            <div class="flex gap-1 rounded-md bg-muted p-1">
              <Button size="sm" variant={mode() === "q" ? "default" : "ghost"} onClick={() => setMode("q")}>Natural language</Button>
              <Button size="sm" variant={mode() === "where" ? "default" : "ghost"} onClick={() => setMode("where")}>SQL-like</Button>
            </div>
            <TextField class="min-w-52 flex-1">
              <TextFieldInput placeholder={mode() === "q" ? "e.g. large banks with foreign inflow" : "e.g. market_cap > 10T"} value={q()} onInput={(e) => setQ(e.currentTarget.value)} />
            </TextField>
            <Button onClick={run} disabled={busy()}>{busy() ? "Screening…" : "Screen"}</Button>
          </div>
          <div class="flex flex-wrap items-center gap-4 text-sm">
            <label class="flex items-center gap-2 text-muted-foreground">broker ≥
              <TextField class="w-20"><TextFieldInput type="number" value={brokerMin()} onInput={(e) => setBrokerMin(Number(e.currentTarget.value))} /></TextField>
            </label>
            <Switch checked={foreignOnly()} onChange={setForeignOnly}>
              <SwitchControl><SwitchThumb /></SwitchControl>
              <SwitchLabel>foreign inflow</SwitchLabel>
            </Switch>
            <Switch checked={insiderOnly()} onChange={setInsiderOnly}>
              <SwitchControl><SwitchThumb /></SwitchControl>
              <SwitchLabel>insider buying</SwitchLabel>
            </Switch>
          </div>
          <Show when={err()}><p class="text-sm text-destructive">{err()}</p></Show>
        </CardContent>
      </Card>
      <Show when={busy()}><div class="space-y-2"><Skeleton class="h-24 w-full" /><Skeleton class="h-24 w-full" /></div></Show>
      <Show when={ran() && !busy()}>
        <Show when={rows().length} fallback={<EmptyState icon="🔍" title="No matches" hint="Loosen the filters and try again." />}>
          <div class="grid gap-4 md:grid-cols-2">
            <For each={rows()}>
              {(r) => (
                <Card>
                  <CardHeader class="flex flex-row items-center justify-between space-y-0 pb-2">
                    <a class="font-bold text-primary hover:underline" href={`/report/${r.symbol}`}>{r.symbol}</a>
                    <Badge variant="secondary" class="font-mono">{r.composite.toFixed(1)}</Badge>
                  </CardHeader>
                  <CardContent class="space-y-2">
                    <BreakdownBars breakdown={r.breakdown as Record<string, unknown>} />
                    <Citations items={r.citations} />
                    <Show when={scoreOf(r) !== 0}><RecBadge rec={scoreOf(r) >= 40 ? "BUY" : scoreOf(r) <= -40 ? "AVOID" : "HOLD"} /></Show>
                  </CardContent>
                </Card>
              )}
            </For>
          </div>
        </Show>
      </Show>
      <Show when={!ran() && !busy()}>
        <EmptyState icon="🔍" title="Run a screen" hint="Ranked rows with per-signal bars appear here." />
      </Show>
      <Card>
        <CardHeader><CardTitle>Saved screeners</CardTitle></CardHeader>
        <CardContent class="space-y-3">
          <div class="flex gap-2">
            <TextField class="w-36"><TextFieldInput placeholder="name" value={sname()} onInput={(e) => setSname(e.currentTarget.value)} /></TextField>
            <Button variant="outline" onClick={save}>Save current</Button>
          </div>
          <ul class="space-y-1 text-sm">
            <For each={Object.entries(saved())}>{([n, query]) => <li><Button variant="ghost" size="sm" onClick={() => { setQ(query); run(); }}>{n}</Button> <span class="text-muted-foreground">{query.slice(0, 60)}</span></li>}</For>
          </ul>
        </CardContent>
      </Card>
    </div>
  );
}
