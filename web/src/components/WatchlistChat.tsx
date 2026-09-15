import { createResource, createSignal, For, Show } from "solid-js";
import { api } from "../lib/api";

// WatchlistDrawer: global watchlist add/remove backed by /api/watchlist.
export function WatchlistDrawer() {
  const [wl, { refetch }] = createResource(() => api.watchlist());
  const [ticker, setTicker] = createSignal("");
  const [err, setErr] = createSignal("");
  async function add() {
    setErr("");
    try {
      await api.addWatch(ticker().toUpperCase().trim());
      setTicker("");
      refetch();
    } catch (e) { setErr(String(e)); }
  }
  async function del(t: string) {
    await api.removeWatch(t);
    refetch();
  }
  return (
    <div class="fs-card">
      <h3>Watchlist</h3>
      <input class="fs-input" placeholder="BBCA" value={ticker()} onInput={(e) => setTicker(e.currentTarget.value)} style={{ width: "90px" }} />
      <button class="fs-btn" onClick={add}>Add</button>
      <Show when={err()}><p class="fs-muted">{err()}</p></Show>
      <ul><For each={wl()?.watchlist || []}>{(t) => <li><a href={`/report/${t}`}>{t}</a> <button class="fs-btn ghost" onClick={() => del(t)}>×</button></li>}</For></ul>
    </div>
  );
}

// ChatSidebar: context-aware Q&A backed by POST /api/chat.
export function ChatSidebar() {
  const [msg, setMsg] = createSignal("");
  const [log, setLog] = createSignal<{ q: string; a: string }[]>([]);
  async function send() {
    const q = msg().trim();
    if (!q) return;
    setMsg("");
    try {
      const r = await api.chat(q);
      setLog((p) => [...p, { q, a: r.answer }].slice(-10));
    } catch (e) {
      setLog((p) => [...p, { q, a: String(e) }].slice(-10));
    }
  }
  return (
    <div class="fs-card">
      <h3>AI Chat</h3>
      <For each={log()}>
        {(m) => <div><p><strong>You:</strong> {m.q}</p><p class="fs-muted">{m.a.slice(0, 500)}</p></div>}
      </For>
      <input class="fs-input" placeholder="Ask about your watchlist…" value={msg()} onInput={(e) => setMsg(e.currentTarget.value)} style={{ width: "220px" }} />
      <button class="fs-btn" onClick={send}>Send</button>
    </div>
  );
}
