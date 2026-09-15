import { createSignal, For, Show, onMount } from "solid-js";
import { useParams } from "@solidjs/router";
import { api, type ReportPayload } from "../lib/api";
import { Citations } from "../components/Citations";
import { RecBadge, Skeleton } from "../components/ui";

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
    <div>
      <div class="fs-page-head">
        <h1>Report: {params.ticker}</h1>
        <button class="fs-btn ghost" onClick={load}>↻ Regenerate</button>
      </div>
      <Show when={err()}><p style={{ color: "var(--fs-avoid)" }}>{err()}</p></Show>
      <Show when={busy()}><Skeleton rows={5} /></Show>
      <Show when={rep()}>
        <div class="fs-card" style={{ "border-left": "4px solid var(--fs-accent)" }}>
          <div class="fs-row">
            <RecBadge rec={rep()!.synthesis.recommendation} />
            <strong>conviction {rep()!.synthesis.conviction}/5</strong>
            <Show when={rep()!.synthesis.conflict}><span class="fs-badge info">conflict flagged</span></Show>
            <span class="fs-muted">position {rep()!.synthesis.position_pct}%</span>
          </div>
          <p>{rep()!.synthesis.thesis}</p>
          <Citations items={rep()!.citations} />
        </div>
        <div class="fs-card" style={{ "margin-top": "14px" }}>
          <h3>Export</h3>
          <div class="fs-row">
            <button class="fs-btn ghost" onClick={exportMd}>Markdown</button>
            <button class="fs-btn ghost" onClick={async () => {
              const r = await fetch(`/api/report/${params.ticker}?format=html`, { method: "POST", headers: authHeaders() });
              dl(URL.createObjectURL(new Blob([await r.text()], { type: "text/html" })), `${params.ticker}-report.html`);
            }}>HTML</button>
            <button class="fs-btn ghost" onClick={() => {
              dl(URL.createObjectURL(new Blob([JSON.stringify(rep(), null, 1)], { type: "application/json" })), `${params.ticker}-report.json`);
            }}>JSON</button>
            <button class="fs-btn ghost" onClick={async () => {
              const r = await fetch(`/api/report/${params.ticker}?format=pdf`, { method: "POST", headers: authHeaders() });
              dl(URL.createObjectURL(await r.blob()), `${params.ticker}-report.pdf`);
            }}>PDF</button>
          </div>
        </div>
        <For each={rep()!.sections}>
          {(s) => <div class="fs-card" style={{ "margin-top": "14px" }}><h3>{s.name}</h3><p style={{ "white-space": "pre-wrap" }}>{s.body}</p><Citations items={s.citations} /></div>}
        </For>
        <div class="fs-card" style={{ "margin-top": "14px" }}>
          <h3>Interrogate this report</h3>
          <div class="fs-row">
            <input class="fs-input" placeholder="Ask about this report…" value={question()} onInput={(e) => setQuestion(e.currentTarget.value)} style={{ flex: 1 }} />
            <button class="fs-btn" onClick={ask} disabled={asking()}>{asking() ? "…" : "Ask"}</button>
          </div>
          <Show when={answer()}><p style={{ "margin-top": "10px" }}>{answer()}</p></Show>
        </div>
      </Show>
      <Show when={md()}><div class="fs-card" style={{ "margin-top": "14px" }}><pre class="fs-muted">{md().slice(0, 2000)}</pre></div></Show>
    </div>
  );
}
