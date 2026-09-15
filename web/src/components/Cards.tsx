import { For, Show } from "solid-js";
import { fmtIDR } from "../lib/api";
import { Citations } from "./Citations";
import { Badge } from "./ui/badge";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "./ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "./ui/table";

export function AccumulationTable(props: { rows: { ticker: string; net_sum: number; brokers: number }[] }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Top accumulation</CardTitle>
        <CardDescription>Net broker accumulation per ticker.</CardDescription>
      </CardHeader>
      <CardContent>
        <Table>
          <TableHeader><TableRow><TableHead>Ticker</TableHead><TableHead class="text-right">Net</TableHead><TableHead class="text-right">Brokers</TableHead></TableRow></TableHeader>
          <TableBody>
            <For each={props.rows}>
              {(r) => <TableRow><TableCell><a class="font-semibold text-primary hover:underline" href={`/report/${r.ticker}`}>{r.ticker}</a></TableCell><TableCell class="text-right font-mono">{fmtIDR(r.net_sum)}</TableCell><TableCell class="text-right">{r.brokers}</TableCell></TableRow>}
            </For>
          </TableBody>
        </Table>
        <Show when={!props.rows?.length}><p class="mt-3 text-sm text-muted-foreground">No accumulation snapshots yet.</p></Show>
      </CardContent>
    </Card>
  );
}

export function RotationMap(props: { signal?: string; citations?: Parameters<typeof Citations>[0]["items"] }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Sector rotation</CardTitle>
      </CardHeader>
      <CardContent>
        <p class="text-sm">{props.signal || "No rotation flip detected."}</p>
        <Citations items={props.citations} />
      </CardContent>
    </Card>
  );
}

export function ActivityFeed(props: { events: { id?: number; ticker?: string; message?: string; date?: string }[] }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Activity</CardTitle>
      </CardHeader>
      <CardContent>
        <Show when={props.events?.length} fallback={<p class="text-sm text-muted-foreground">No events yet.</p>}>
          <ul class="space-y-2 text-sm">
            <For each={props.events}>
              {(e) => <li><Badge variant="secondary">{e.ticker}</Badge> {e.message} <span class="text-muted-foreground">{e.date}</span></li>}
            </For>
          </ul>
        </Show>
      </CardContent>
    </Card>
  );
}
