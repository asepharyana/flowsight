import { createResource, createSignal, For, Show } from "solid-js";
import { api } from "../lib/api";
import { EmptyState, Skeleton } from "../components/ui";

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
    <div>
      <div class="fs-page-head"><h1>Alerts</h1><p class="fs-muted">Rules evaluated over snapshots each cycle.</p></div>
      <div class="fs-card">
        <h3>Push destinations (per login)</h3>
        <p class="fs-muted">Secrets stay server-side — the list never shows tokens.</p>
        <div class="fs-row">
          <select class="fs-input" value={dkind()} onChange={(e) => setDkind(e.currentTarget.value)}>
            <option value="telegram">Telegram</option>
            <option value="discord">Discord</option>
          </select>
          <input class="fs-input" placeholder="label (e.g. my phone)" value={dlabel()} onInput={(e) => setDlabel(e.currentTarget.value)} style={{ width: "160px" }} />
          {dkind() === "telegram" ? (
            <>
              <input class="fs-input" placeholder="bot token" value={dbot()} onInput={(e) => setDbot(e.currentTarget.value)} style={{ width: "200px" }} />
              <input class="fs-input" placeholder="chat id" value={dchat()} onInput={(e) => setDchat(e.currentTarget.value)} style={{ width: "120px" }} />
            </>
          ) : (
            <input class="fs-input" placeholder="https://discord webhook url" value={dhook()} onInput={(e) => setDhook(e.currentTarget.value)} style={{ flex: 1 }} />
          )}
          <button class="fs-btn" onClick={addDest}>Add destination</button>
        </div>
        {derr() ? <p style={{ color: "var(--fs-avoid)" }}>{derr()}</p> : null}
        <ul>
          {(dests()?.destinations || []).map((d) => (
            <li><span class={`fs-badge ${d.enabled ? "buy" : "info"}`}>{d.kind}</span> {d.label || `#${d.id}`} <span class="fs-muted">{d.enabled ? "on" : "off"} · {d.configured ? "configured" : "missing secret"}</span>
              <button class="fs-btn ghost" onClick={() => toggleDest(d.id, d.enabled)}>{d.enabled ? "Disable" : "Enable"}</button>
              <button class="fs-btn danger" onClick={() => delDest(d.id)}>Delete</button>
            </li>
          ))}
        </ul>
      </div>
      <div class="fs-card" style={{ "margin-top": "14px" }}>
        <h3>New rule</h3>
        <div class="fs-row">
          <input class="fs-input" placeholder="rule name" value={name()} onInput={(e) => setName(e.currentTarget.value)} style={{ flex: 1 }} />
        </div>
        <div class="fs-row" style={{ "margin-top": "10px" }}>
          <select class="fs-input" value={field()} onChange={(e) => setField(e.currentTarget.value)}>
            <For each={FIELDS}>{(f) => <option value={f}>{f}</option>}</For>
          </select>
          <select class="fs-input" value={op()} onChange={(e) => setOp(e.currentTarget.value)}>
            <For each={OPS}>{(o) => <option value={o}>{o}</option>}</For>
          </select>
          <input class="fs-input" type="number" value={value()} onInput={(e) => setValue(Number(e.currentTarget.value))} style={{ width: "100px" }} />
          <button class="fs-btn ghost" onClick={addCond}>+ condition</button>
        </div>
        <Show when={conds().length}>
          <ul><For each={conds()}>{(c, i) => <li class="fs-muted">{c.field} {c.op} {c.value} <button class="fs-btn ghost" onClick={() => setConds((p) => p.filter((_, j) => j !== i()))}>×</button></li>}</For></ul>
        </Show>
        <p class="fs-muted">Rule JSON: <code>{rulePreview()}</code></p>
        <div class="fs-row">
          <input class="fs-input" placeholder="channels, comma-separated (blank = record only)" value={channels()} onInput={(e) => setChannels(e.currentTarget.value)} style={{ flex: 1 }} />
          <button class="fs-btn" onClick={create} disabled={busy()}>Create rule</button>
        </div>
      </div>
      <div class="fs-grid" style={{ "margin-top": "14px" }}>
        <div class="fs-card"><h3>Rules</h3>
          <Show when={(alerts()?.alerts || []).length} fallback={<EmptyState icon="🔔" title="No rules" hint="Create one above." />}>
            <ul><For each={alerts()?.alerts || []}>{(a) => <li>{a.name} <span class="fs-muted">last: {a.last_fired || "never"}</span> <button class="fs-btn danger" onClick={() => del(a.id)}>Delete</button></li>}</For></ul>
          </Show>
        </div>
        <div class="fs-card"><h3>Event history</h3>
          <button class="fs-btn ghost" onClick={() => refetchEv()}>Refresh</button>
          <Show when={(events() || []).length} fallback={<p class="fs-muted">No events yet.</p>}>
            <ul><For each={events() || []}>{(e) => <li><strong>{String(e.ticker)}</strong> — {String(e.message)}</li>}</For></ul>
          </Show>
        </div>
      </div>
    </div>
  );
}
