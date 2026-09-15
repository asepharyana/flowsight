import { createSignal, For, Show } from "solid-js";
import { useParams } from "@solidjs/router";
import { api, type ReportPayload } from "../lib/api";
import { Citations } from "../components/Citations";

export default function Report() {
  const params = useParams();
  const [rep, setRep] = createSignal<ReportPayload | null>(null);
  const [md, setMd] = createSignal("");
  const [question, setQuestion] = createSignal("kenapa conviction segitu?");
  const [answer, setAnswer] = createSignal("");
  const [err, setErr] = createSignal("");
  async function load() {
    setErr("");
    try { setRep(await api.report(params.ticker)); }
    catch (e) { setErr(String(e)); }
  }
  async function exportMd() { setMd(await api.reportMd(params.ticker)); }
  async function ask() {
    const r = await api.interrogate(params.ticker, question());
    setAnswer(r.answer);
  }
  return (
    <div>
      <h1>Report: {params.ticker}</h1>
      <button class="fs-btn" onClick={load}>Generate report</button>
      <button class="fs-btn ghost" onClick={exportMd}>Export MD</button>
      <button class="fs-btn ghost" onClick={async () => {
        const r = await fetch(`/api/report/${params.ticker}?format=html`, { method: "POST", headers: { "X-User-Key": localStorage.getItem("fs-key") || "demo" } });
        const html = await r.text();
        const url = URL.createObjectURL(new Blob([html], { type: "text/html" }));
        window.open(url, "_blank");
      }}>HTML</button>
      <button class="fs-btn ghost" onClick={() => {
        const blob = new Blob([JSON.stringify(rep(), null, 1)], { type: "application/json" });
        const a = document.createElement("a");
        a.href = URL.createObjectURL(blob);
        a.download = `${params.ticker}-report.json`;
        a.click();
      }}>JSON</button>
      <button class="fs-btn ghost" onClick={async () => {
        const r = await fetch(`/api/report/${params.ticker}?format=pdf`, { method: "POST", headers: { "X-User-Key": localStorage.getItem("fs-key") || "demo" } });
        const blob = await r.blob();
        const url = URL.createObjectURL(blob);
        window.open(url, "_blank");
      }}>PDF</button>
      <Show when={err()}><p class="fs-muted">{err()}</p></Show>
      <Show when={rep()}>
        <div class="fs-card">
          <h2 class={`rec-${rep()!.synthesis.recommendation}`}>{rep()!.synthesis.recommendation} · conviction {rep()!.synthesis.conviction}/5</h2>
          <p>{rep()!.synthesis.thesis}</p>
          <Citations items={rep()!.citations} />
        </div>
        <For each={rep()!.sections}>
          {(s) => <div class="fs-card"><h3>{s.name}</h3><p>{s.body}</p><Citations items={s.citations} /></div>}
        </For>
        <div class="fs-card">
          <h3>Interrogate this report</h3>
          <input class="fs-input" value={question()} onInput={(e) => setQuestion(e.currentTarget.value)} style={{ width: "320px" }} />
          <button class="fs-btn" onClick={ask}>Ask</button>
          <Show when={answer()}><p>{answer()}</p></Show>
        </div>
      </Show>
      <Show when={md()}><div class="fs-card"><pre class="fs-muted">{md().slice(0, 2000)}</pre></div></Show>
    </div>
  );
}
