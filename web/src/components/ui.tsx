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

export function RecBadge(props: { rec: string }) {
  const v = () =>
    /buy/i.test(props.rec) ? { variant: "success" as const, icon: "▲" }
    : /avoid|sell/i.test(props.rec) ? { variant: "error" as const, icon: "▼" }
    : { variant: "warning" as const, icon: "●" };
  return <Badge variant={v().variant}>{v().icon} {props.rec}</Badge>;
}

export function BreakdownBars(props: { breakdown: Record<string, unknown> }) {
  const entries = () => Object.entries(props.breakdown)
    .filter(([, v]) => typeof v === "number")
    .sort((a, b) => Math.abs(b[1] as number) - Math.abs(a[1] as number))
    .slice(0, 6);
  return (
    <div class="space-y-1.5">
      <For each={entries()}>
        {([k, v]) => {
          const n = v as number;
          const pct = Math.min(100, Math.abs(n));
          return (
            <div class="flex items-center gap-2 text-xs">
              <span class="w-24 shrink-0 truncate text-muted-foreground">{k}</span>
              <div class="h-2 flex-1 overflow-hidden rounded-full bg-secondary">
                <div
                  class="h-full rounded-full"
                  classList={{ "bg-success": n >= 0, "bg-error": n < 0 }}
                  style={{ width: `${pct}%`, "margin-left": n < 0 ? "auto" : undefined }}
                />
              </div>
              <span class="w-12 shrink-0 text-right font-mono">{n.toFixed(1)}</span>
            </div>
          );
        }}
      </For>
    </div>
  );
}

export function PageHead(props: { title: string; sub?: string }) {
  return (
    <div>
      <h1 class="text-2xl font-bold tracking-tight">{props.title}</h1>
      {props.sub ? <p class="text-sm text-muted-foreground">{props.sub}</p> : null}
    </div>
  );
}

export function EmptyState(props: { icon: string; title: string; hint?: string; action?: import("solid-js").JSX.Element }) {
  return (
    <div class="flex flex-col items-center gap-1 rounded-lg border border-dashed px-6 py-10 text-center">
      <div class="text-3xl">{props.icon}</div>
      <p class="font-medium">{props.title}</p>
      {props.hint ? <p class="text-sm text-muted-foreground">{props.hint}</p> : null}
      {props.action}
    </div>
  );
}
