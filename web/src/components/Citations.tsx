import { For } from "solid-js";
import type { Citation } from "../lib/api";
import { Badge } from "./ui/badge";
import { Tooltip, TooltipContent, TooltipTrigger } from "./ui/tooltip";

export function Citations(props: { items?: Citation[] | string }) {
  const list = (): Citation[] => {
    if (!props.items) return [];
    if (typeof props.items === "string") {
      try { return JSON.parse(props.items); } catch { return []; }
    }
    return props.items;
  };
  return (
    <span class="inline-flex flex-wrap gap-1">
      <For each={list()}>
        {(c) => (
          <Tooltip>
            <TooltipTrigger>
              <Badge variant={c.stale ? "secondary" : "outline"} class={c.stale ? "opacity-70" : ""}>
                {c.endpoint} {c.ticker || ""} @ {(c.snapshot_at || "").slice(0, 10)}{c.stale ? " · stale" : ""}
              </Badge>
            </TooltipTrigger>
            <TooltipContent>{c.endpoint} @ {c.snapshot_at}{c.stale ? " (stale)" : ""}</TooltipContent>
          </Tooltip>
        )}
      </For>
    </span>
  );
}
