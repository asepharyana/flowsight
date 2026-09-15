import { For, Show, type JSX } from "solid-js";

// Shared building blocks: StatCard, EmptyState, Badge, SignalBar, Skeleton.

export function StatCard(props: { title: string; value: string; sub?: string; children?: JSX.Element }) {
  return (
    <div class="fs-card">
      <h3>{props.title}</h3>
      <p class="fs-stat">{props.value}</p>
      <Show when={props.sub}><p class="fs-muted" style={{ margin: 0 }}>{props.sub}</p></Show>
      {props.children}
    </div>
  );
}

export function EmptyState(props: { icon: string; title: string; hint?: string; action?: JSX.Element }) {
  return (
    <div class="fs-card fs-empty">
      <div class="big">{props.icon}</div>
      <strong>{props.title}</strong>
      <Show when={props.hint}><p class="fs-muted">{props.hint}</p></Show>
      <Show when={props.action}><div style={{ "margin-top": "10px" }}>{props.action}</div></Show>
    </div>
  );
}

export function Skeleton(props: { rows?: number }) {
  const n = props.rows ?? 3;
  return (
    <div class="fs-stack">
      <For each={Array.from({ length: n })}>{() => <div class="fs-sk" style={{ height: "56px" }} />}</For>
    </div>
  );
}

export function RecBadge(props: { rec: string }) {
  const cls = props.rec === "BUY" ? "buy" : props.rec === "HOLD" ? "hold" : "avoid";
  return <span class={`fs-badge ${cls}`}>{props.rec}</span>;
}

// SignalBar renders one -100..+100 signal as a labeled bar.
export function SignalBar(props: { label: string; value: number }) {
  const v = Math.max(-100, Math.min(100, props.value));
  const left = v < 0 ? `${50 + v / 2}%` : "50%";
  const width = `${Math.abs(v) / 2}%`;
  const cls = v < 0 ? "neg" : v > 0 ? "pos" : "";
  return (
    <div class="fs-sig">
      <span class="fs-muted">{props.label}</span>
      <span class="bar"><i class={cls} style={{ left, width }} /></span>
      <b>{v}</b>
    </div>
  );
}

// BreakdownBars renders a screen/report signal breakdown map as bars.
// Unknown numeric fields are shown; booleans/strings are skipped except flags.
export function BreakdownBars(props: { breakdown: Record<string, unknown> }) {
  const entries = () =>
    Object.entries(props.breakdown || {}).filter(([, v]) => typeof v === "number") as [string, number][];
  return (
    <Show when={entries().length} fallback={<span class="fs-muted">no signals</span>}>
      <For each={entries()}>{([k, v]) => <SignalBar label={k.replace(/_/g, " ")} value={v} />}</For>
    </Show>
  );
}
