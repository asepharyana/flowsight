import { createResource, createSignal, For } from "solid-js";
import { api } from "../lib/api";

export default function Alerts() {
  const [alerts, { refetch }] = createResource(() => api.alerts());
  const [events, { refetch: refetchEv }] = createResource(() => api.alertEvents("2000-01-01").then((r) => r.events.slice(0, 30)));
  const [dests, { refetch: refetchDests }] = createResource(() => api.destinations());
  const [name, setName] = createSignal("accumulation watcher");
  const [rule, setRule] = createSignal('{"all": true}');
  const [channels, setChannels] = createSignal("");
  const [dkind, setDkind] = createSignal("telegram");
  const [dlabel, setDlabel] = createSignal("");
  const [dbot, setDbot] = createSignal("");
  const [dchat, setDchat] = createSignal("");
  const [dhook, setDhook] = createSignal("");
  const [derr, setDerr] = createSignal("");
  async function create() {
    await api.createAlert({
      name: name(),
      rule: JSON.parse(rule()),
      channels: channels().split(",").map((c) => c.trim()).filter(Boolean),
    });
    refetch();
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
    <div>
      <h1>Alerts</h1>
      <div class="fs-card">
        <h3>Push destinations (per login)</h3>
        <p class="fs-muted">Secrets stay server-side — the list never shows tokens. Delivery uses your enabled destinations; server env is fallback.</p>
        <select class="fs-input" value={dkind()} onChange={(e) => setDkind(e.currentTarget.value)}>
          <option value="telegram">Telegram</option>
          <option value="discord">Discord</option>
        </select>
        <input class="fs-input" placeholder="label (e.g. my phone)" value={dlabel()} onInput={(e) => setDlabel(e.currentTarget.value)} style={{ width: "160px" }} />
        {dkind() === "telegram" ? (
          <>
            <input class="fs-input" placeholder="bot token" value={dbot()} onInput={(e) => setDbot(e.currentTarget.value)} style={{ width: "220px" }} />
            <input class="fs-input" placeholder="chat id" value={dchat()} onInput={(e) => setDchat(e.currentTarget.value)} style={{ width: "140px" }} />
          </>
        ) : (
          <input class="fs-input" placeholder="https://discord webhook url" value={dhook()} onInput={(e) => setDhook(e.currentTarget.value)} style={{ width: "300px" }} />
        )}
        <button class="fs-btn" onClick={addDest}>Add destination</button>
        {derr() ? <p class="fs-muted">{derr()}</p> : null}
        <ul>
          {(dests()?.destinations || []).map((d) => (
            <li>{d.kind} — {d.label || `#${d.id}`} <span class="fs-muted">{d.enabled ? "on" : "off"} · {d.configured ? "configured" : "missing secret"}</span>
              <button class="fs-btn ghost" onClick={() => toggleDest(d.id, d.enabled)}>{d.enabled ? "Disable" : "Enable"}</button>
              <button class="fs-btn ghost" onClick={() => delDest(d.id)}>del</button>
            </li>
          ))}
        </ul>
      </div>
      <div class="fs-card">
        <h3>New rule</h3>
        <input class="fs-input" value={name()} onInput={(e) => setName(e.currentTarget.value)} />
        <input class="fs-input" value={rule()} onInput={(e) => setRule(e.currentTarget.value)} style={{ width: "280px" }} />
        <input class="fs-input" placeholder="Telegram/Discord channels, comma-separated" value={channels()} onInput={(e) => setChannels(e.currentTarget.value)} style={{ width: "300px" }} />
        <button class="fs-btn" onClick={create}>Create</button>
        <p class="fs-muted">Delivery: your enabled destinations above (server env fallback). Empty = record only.</p>
      </div>
      <div class="fs-grid">
        <div class="fs-card"><h3>Rules</h3>
          <ul><For each={alerts()?.alerts || []}>{(a) => <li>{a.name} <span class="fs-muted">last: {a.last_fired || "never"}</span> <button class="fs-btn ghost" onClick={() => del(a.id)}>del</button></li>}</For></ul>
        </div>
        <div class="fs-card"><h3>Event history</h3>
          <button class="fs-btn ghost" onClick={() => refetchEv()}>Refresh</button>
          <ul><For each={events() || []}>{(e) => <li><strong>{String(e.ticker)}</strong> — {String(e.message)}</li>}</For></ul>
        </div>
      </div>
    </div>
  );
}
