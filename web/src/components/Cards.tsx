import { For, Show } from "solid-js";
import { fmtIDR } from "../lib/api";
import { Citations } from "./Citations";

export function AccumulationTable(props: { rows: { ticker: string; net_sum: number; brokers: number }[] }) {
  return (
    <div class="fs-card">
      <h3>Top accumulation</h3>
      <table class="fs-table">
        <thead><tr><th>Ticker</th><th>Net</th><th>Brokers</th></tr></thead>
        <tbody>
          <For each={props.rows}>
            {(r) => <tr><td><a href={`/report/${r.ticker}`}>{r.ticker}</a></td><td>{fmtIDR(r.net_sum)}</td><td>{r.brokers}</td></tr>}
          </For>
        </tbody>
      </table>
      <Show when={!props.rows?.length}><p class="fs-muted">No accumulation snapshots yet.</p></Show>
    </div>
  );
}

export function RotationMap(props: { signal?: string; citations?: Parameters<typeof Citations>[0]["items"] }) {
  return (
    <div class="fs-card">
      <h3>Sector rotation</h3>
      <p>{props.signal || "No rotation flip detected."}</p>
      <Citations items={props.citations} />
    </div>
  );
}

export function ActivityFeed(props: { events: { id?: number; ticker?: string; message?: string; date?: string }[] }) {
  return (
    <div class="fs-card">
      <h3>Activity</h3>
      <Show when={props.events?.length} fallback={<p class="fs-muted">No events yet.</p>}>
        <ul>
          <For each={props.events}>
            {(e) => <li><strong>{e.ticker}</strong> — {e.message} <span class="fs-muted">{e.date}</span></li>}
          </For>
        </ul>
      </Show>
    </div>
  );
}
