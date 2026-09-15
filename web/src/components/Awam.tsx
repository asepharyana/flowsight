import { For, Show } from "solid-js";
import { Badge } from "./ui/badge";
import { Tooltip, TooltipContent, TooltipTrigger } from "./ui/tooltip";
import { ISTILAH, type Verdict } from "../lib/awam";

// Kata teknis bergaris-titik + tooltip penjelasan santai.
export function Term(props: { kata: string }) {
  const key = props.kata.toLowerCase();
  const arti = ISTILAH[key];
  if (!arti) return <span>{props.kata}</span>;
  return (
    <Tooltip>
      <TooltipTrigger as="span" class="cursor-help underline decoration-dotted underline-offset-2">{props.kata}</TooltipTrigger>
      <TooltipContent class="max-w-xs">{arti}</TooltipContent>
    </Tooltip>
  );
}

export function VerdictBadge(props: { verdict: Verdict }) {
  const warna: Record<Verdict, "default" | "error" | "secondary"> = {
    "Dilirik": "default",
    "Dilepas": "error",
    "Netral": "secondary",
  };
  return <Badge variant={warna[props.verdict]}>{props.verdict}</Badge>;
}

// Bar alasan: seberapa kuat sinyal asing vs broker (0-100).
export function AlasanBar(props: { asing: number; broker: number }) {
  const total = Math.abs(props.asing) + Math.abs(props.broker) || 1;
  const pAsing = Math.round((Math.abs(props.asing) / total) * 100);
  return (
    <div>
      <div class="flex h-2 w-full overflow-hidden rounded-full bg-muted">
        <div class="bg-blue-500" style={{ width: `${pAsing}%` }} />
        <div class="bg-emerald-500" style={{ width: `${100 - pAsing}%` }} />
      </div>
      <div class="mt-1 flex justify-between text-xs text-muted-foreground">
        <span><span class="text-blue-500">■</span> Asing {props.asing >= 0 ? "masuk" : "keluar"}</span>
        <span><span class="text-emerald-500">■</span> Broker {props.broker >= 0 ? "borong" : "jualan"}</span>
      </div>
    </div>
  );
}

export function IstilahStrip() {
  const kunci = ["akumulasi", "foreign flow", "broker", "net", "briefing", "alert"];
  return (
    <p class="text-xs text-muted-foreground">
      Istilah asing? Arahkan kursor:{" "}
      <For each={kunci}>{(k, i) => <><Show when={i() > 0}>, </Show><Term kata={k} /></>}</For>
    </p>
  );
}
