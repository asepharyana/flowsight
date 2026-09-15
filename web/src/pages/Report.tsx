import { createSignal, For, Show, onMount } from "solid-js";
import { useParams } from "@solidjs/router";
import { api, type ReportPayload } from "../lib/api";
import { Citations } from "../components/Citations";
import { PageHead, RecBadge } from "../components/ui";
import { Badge } from "../components/ui/badge";
import { Button } from "../components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "../components/ui/card";
import { Skeleton } from "../components/ui/skeleton";
import { TextField, TextFieldInput } from "../components/ui/text-field";

export default function Report() {
  const params = useParams();
  const [rep, setRep] = createSignal<ReportPayload | null>(null);
  const [md, setMd] = createSignal("");
  const [question, setQuestion] = createSignal("");
  const [answer, setAnswer] = createSignal("");
  const [busy, setBusy] = createSignal(true);
  const [asking, setAsking] = createSignal(false);
  const [err, setErr] = createSignal("");
  const authHeaders = () => ({ "X-User-Key": localStorage.getItem("fs-key") || "demo" });
  async function load() {
    setErr(""); setBusy(true);
    try { setRep(await api.report(params.ticker)); }
    catch (e) { setErr(String(e)); }
    finally { setBusy(false); }
  }
  onMount(load);
  async function exportMd() { setMd(await api.reportMd(params.ticker)); }
  async function ask() {
    if (!question().trim()) return;
    setAsking(true);
    try {
      const r = await api.interrogate(params.ticker, question());
      setAnswer(r.answer);
    } finally { setAsking(false); }
  }
  function dl(url: string, name: string) {
    const a = document.createElement("a");
    a.href = url; a.download = name; a.click();
  }
  return (
    <div class="space-y-4">
      <div class="flex items-center justify-between">
        <PageHead title={`Report: ${params.ticker}`} />
        <Button variant="outline" onClick={load}>↻ Regenerate</Button>
      </div>
      <Show when={err()}><p class="text-sm text-destructive">{err()}</p></Show>
      <Show when={busy()}><div class="space-y-2"><Skeleton class="h-32 w-full" /><Skeleton class="h-24 w-full" /></div></Show>
      <Show when={rep()}>
        <Card class="border-l-4 border-l-primary">
          <CardContent class="flex flex-wrap items-center gap-2 pt-6">
            <RecBadge rec={rep()!.synthesis.recommendation} />
            <strong>conviction {rep()!.synthesis.conviction}/5</strong>
            <Show when={rep()!.synthesis.conflict}><Badge variant="secondary">conflict flagged</Badge></Show>
            <span class="text-sm text-muted-foreground">position {rep()!.synthesis.position_pct}%</span>
          </CardContent>
          <CardContent>
            <p class="text-sm">{rep()!.synthesis.thesis}</p>
            <div class="mt-2"><Citations items={rep()!.citations} /></div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader><CardTitle>Export</CardTitle></CardHeader>
          <CardContent class="flex flex-wrap gap-2">
            <Button variant="outline" onClick={exportMd}>Markdown</Button>
            <Button variant="outline" onClick={async () => {
              const r = await fetch(`/api/report/${params.ticker}?format=html`, { method: "POST", headers: authHeaders() });
              dl(URL.createObjectURL(new Blob([await r.text()], { type: "text/html" })), `${params.ticker}-report.html`);
            }}>HTML</Button>
            <Button variant="outline" onClick={() => {
              dl(URL.createObjectURL(new Blob([JSON.stringify(rep(), null, 1)], { type: "application/json" })), `${params.ticker}-report.json`);
            }}>JSON</Button>
            <Button variant="outline" onClick={async () => {
              const r = await fetch(`/api/report/${params.ticker}?format=pdf`, { method: "POST", headers: authHeaders() });
              dl(URL.createObjectURL(await r.blob()), `${params.ticker}-report.pdf`);
            }}>PDF</Button>
          </CardContent>
        </Card>
        <For each={rep()!.sections}>
          {(s) => <Card><CardHeader><CardTitle>{s.name}</CardTitle></CardHeader><CardContent><p class="whitespace-pre-wrap text-sm">{s.body}</p><div class="mt-2"><Citations items={s.citations} /></div></CardContent></Card>}
        </For>
        <Card>
          <CardHeader><CardTitle>Interrogate this report</CardTitle></CardHeader>
          <CardContent class="space-y-3">
            <div class="flex gap-2">
              <TextField class="flex-1"><TextFieldInput placeholder="Ask about this report…" value={question()} onInput={(e) => setQuestion(e.currentTarget.value)} /></TextField>
              <Button onClick={ask} disabled={asking()}>{asking() ? "…" : "Ask"}</Button>
            </div>
            <Show when={answer()}><p class="text-sm">{answer()}</p></Show>
          </CardContent>
        </Card>
      </Show>
      <Show when={md()}><Card><CardContent class="pt-6"><pre class="whitespace-pre-wrap text-sm text-muted-foreground">{md().slice(0, 2000)}</pre></CardContent></Card></Show>
    </div>
  );
}
