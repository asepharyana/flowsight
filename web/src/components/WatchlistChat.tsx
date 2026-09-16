import { createResource, createSignal, For, Show } from "solid-js";
import { api } from "../lib/api";
import { Badge } from "./ui/badge";
import { Button } from "./ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "./ui/card";
import { TextField, TextFieldInput } from "./ui/text-field";

export function WatchlistDrawer() {
  const [wl, { refetch }] = createResource(() => api.watchlist());
  const [ticker, setTicker] = createSignal("");
  const [err, setErr] = createSignal("");
  async function add() {
    setErr("");
    const t = ticker().toUpperCase().trim();
    if (!t) return;
    try {
      await api.addWatch(t);
      setTicker("");
      refetch();
    } catch (e) { setErr(String(e)); }
  }
  async function del(t: string) {
    try {
      await api.removeWatch(t);
      refetch();
    } catch (e) { setErr(String(e)); }
  }
  return (
    <Card>
      <CardHeader class="pb-2"><CardTitle class="text-base">Watchlist</CardTitle></CardHeader>
      <CardContent class="space-y-2">
        <div class="flex gap-2">
          <label class="sr-only" for="wl-ticker">Tambah ticker</label>
          <TextField class="w-24"><TextFieldInput id="wl-ticker" placeholder="BBCA" value={ticker()} onInput={(e) => setTicker(e.currentTarget.value)} /></TextField>
          <Button size="sm" onClick={add}>Add</Button>
        </div>
        <Show when={err()}><p class="text-xs text-destructive">{err()}</p></Show>
        <ul class="flex flex-wrap gap-1.5">
          <Show when={(wl()?.watchlist || []).length} fallback={<li class="text-xs text-muted-foreground">Ketik kode saham lalu Add — mis. BBCA, TLKM, BBRI.</li>}>
            <For each={wl()?.watchlist || []}>{(t) => <li class="flex items-center gap-1"><a href={`/report/${encodeURIComponent(t)}`}><Badge variant="secondary">{t}</Badge></a> <button class="text-xs text-muted-foreground hover:text-foreground" aria-label={`Hapus ${t} dari watchlist`} onClick={() => del(t)}>×</button></li>}</For>
          </Show>
        </ul>
      </CardContent>
    </Card>
  );
}

export function ChatSidebar() {
  const [msg, setMsg] = createSignal("");
  const [log, setLog] = createSignal<{ q: string; a: string; err?: boolean }[]>([]);
  async function send() {
    const q = msg().trim();
    if (!q) return;
    setMsg("");
    try {
      const r = await api.chat(q);
      setLog((p) => [...p, { q, a: r.answer }].slice(-10));
    } catch (e) {
      setLog((p) => [...p, { q, a: String(e), err: true }].slice(-10));
    }
  }
  return (
    <Card>
      <CardHeader class="pb-2"><CardTitle class="text-base">AI Chat</CardTitle></CardHeader>
      <CardContent class="space-y-2">
        <Show when={log().length} fallback={<p class="text-xs text-muted-foreground">Tanya soal watchlist-mu — sistem jawab dari data yang tersimpan (bukan karangan).</p>}>
          <div class="max-h-64 space-y-2 overflow-y-auto">
            <For each={log()}>
              {(m) => (
                <div class="text-sm">
                  <p><strong>You:</strong> {m.q}</p>
                  <p classList={{ "text-muted-foreground": !m.err, "text-destructive": !!m.err }}>{m.a.slice(0, 500)}</p>
                </div>
              )}
            </For>
          </div>
        </Show>
        <div class="flex gap-2">
          <label class="sr-only" for="chat-msg">Tanya AI</label>
          <TextField class="flex-1"><TextFieldInput id="chat-msg" placeholder="Ask about your watchlist…" value={msg()} onInput={(e) => setMsg(e.currentTarget.value)} onKeyDown={(e: KeyboardEvent) => { if (e.key === "Enter") send(); }} /></TextField>
          <Button size="sm" onClick={send}>Send</Button>
        </div>
      </CardContent>
    </Card>
  );
}
