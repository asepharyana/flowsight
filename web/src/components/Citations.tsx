import { For } from "solid-js";
import type { Citation } from "../lib/api";

export function Citations(props: { items?: Citation[] | string }) {
  const list = (): Citation[] => {
    if (!props.items) return [];
    if (typeof props.items === "string") {
      try { return JSON.parse(props.items); } catch { return []; }
    }
    return props.items;
  };
  return (
    <span>
      <For each={list()}>
        {(c) => (
          <span class={`fs-cite${c.stale ? " stale" : ""}`} title={`${c.endpoint} @ ${c.snapshot_at}${c.stale ? " (stale)" : ""}`}>
            {c.endpoint} {c.ticker || ""} @ {(c.snapshot_at || "").slice(0, 10)}{c.stale ? " · stale" : ""}
          </span>
        )}
      </For>
    </span>
  );
}
