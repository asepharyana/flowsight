import { render } from "solid-js/web";
import { Router, Route, useLocation, useNavigate, useParams, type RouteSectionProps } from "@solidjs/router";
import { createSignal, Show } from "solid-js";
import { WatchlistDrawer, ChatSidebar } from "./components/WatchlistChat";
import { ThemeToggle, useAuth, AuthButton } from "./components/auth";
import { Button } from "./components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "./components/ui/card";
import { TextField, TextFieldInput } from "./components/ui/text-field";
import { api } from "./lib/api";
import Dashboard from "./pages/Dashboard";
import Routines from "./pages/Routines";
import Screener from "./pages/Screener";
import Alerts from "./pages/Alerts";
import Report from "./pages/Report";
import Portfolio from "./pages/Portfolio";
import "./styles/globals.css";

const NAV = [
  { href: "/", icon: "📊", label: "Dashboard" },
  { href: "/routines", icon: "🗓️", label: "Routines" },
  { href: "/screener", icon: "🔍", label: "Screener" },
  { href: "/alerts", icon: "🔔", label: "Alerts" },
  { href: "/portfolio", icon: "💼", label: "Portfolio" },
];

function Nav() {
  const loc = useLocation();
  return (
    <nav class="flex w-56 shrink-0 flex-col gap-1 border-r bg-card p-4 max-lg:hidden">
      <div class="mb-4 px-2 text-xl font-bold tracking-tight">
        Flow<span class="text-primary">Sight</span>
      </div>
      {NAV.map((n) => (
        <a
          href={n.href}
          class={`flex items-center gap-2 rounded-md px-3 py-2 text-sm font-medium transition-colors hover:bg-accent hover:text-accent-foreground ${loc.pathname === n.href ? "bg-accent text-accent-foreground" : "text-muted-foreground"}`}
        >
          <span>{n.icon}</span> {n.label}
        </a>
      ))}
    </nav>
  );
}

function MobileNav() {
  const loc = useLocation();
  return (
    <nav class="fixed inset-x-0 bottom-0 z-40 flex border-t bg-card px-2 py-1 lg:hidden">
      {NAV.map((n) => (
        <a
          href={n.href}
          class={`flex flex-1 flex-col items-center rounded-md py-1.5 text-[11px] font-medium ${loc.pathname === n.href ? "text-primary" : "text-muted-foreground"}`}
        >
          <span class="text-base">{n.icon}</span> {n.label}
        </a>
      ))}
    </nav>
  );
}

function TickerSearch() {
  const nav = useNavigate();
  const [q, setQ] = createSignal("");
  const go = () => {
    const t = q().toUpperCase().trim();
    if (t) nav(`/report/${t}`);
  };
  return (
    <form class="flex max-w-sm flex-1 items-center gap-2" onSubmit={(e) => { e.preventDefault(); go(); }}>
      <TextField class="flex-1">
        <TextFieldInput placeholder="Ticker → report (e.g. BBCA)" value={q()}
          onInput={(e) => setQ(e.currentTarget.value)} />
      </TextField>
      <Button type="submit" variant="secondary">Open</Button>
    </form>
  );
}

function LoginPage() {
  const params = useParams<{ error?: string }>();
  const err = () => new URLSearchParams(window.location.search).get("error");
  void params;
  return (
    <div class="mx-auto max-w-md py-[8vh]">
      <Card>
        <CardHeader class="text-center">
          <CardTitle class="text-2xl">Welcome to FlowSight</CardTitle>
          <CardDescription>Sign in with Google to get your own watchlist, routines, alerts and reports.</CardDescription>
        </CardHeader>
        <CardContent class="text-center">
          <Show when={err()}><p class="mb-3 text-sm text-destructive">Login failed: {err()}</p></Show>
          <a href="/api/auth/start"><Button size="lg">Sign in with Google</Button></a>
        </CardContent>
      </Card>
    </div>
  );
}

function Shell(props: RouteSectionProps) {
  const { me, refetch } = useAuth();
  const logout = async () => { await api.logout(); refetch(); };
  return (
    <div class="flex min-h-screen bg-background text-foreground">
      <Nav />
      <div class="flex min-w-0 flex-1 flex-col">
        <header class="sticky top-0 z-30 flex items-center gap-3 border-b bg-background/95 px-4 py-2.5 backdrop-blur">
          <TickerSearch />
          <span class="flex-1" />
          <ThemeToggle />
          <AuthButton me={me()} onLogout={logout} />
        </header>
        <main class="mx-auto w-full max-w-6xl flex-1 space-y-6 p-4 pb-20 lg:p-6">{props.children}</main>
      </div>
      <aside class="hidden w-80 shrink-0 flex-col gap-4 border-l bg-card p-4 xl:flex">
        <WatchlistDrawer />
        <ChatSidebar />
      </aside>
      <MobileNav />
    </div>
  );
}

render(
  () => (
    <Router root={Shell}>
      <Route path="/" component={Dashboard} />
      <Route path="/routines" component={Routines} />
      <Route path="/screener" component={Screener} />
      <Route path="/alerts" component={Alerts} />
      <Route path="/report/:ticker" component={Report} />
      <Route path="/portfolio" component={Portfolio} />
      <Route path="/login" component={LoginPage} />
    </Router>
  ),
  document.getElementById("root")!,
);
