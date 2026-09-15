import { render } from "solid-js/web";
import { Router, Route, useLocation, useNavigate, type RouteSectionProps } from "@solidjs/router";
import { createResource, createSignal, Show } from "solid-js";
import { WatchlistDrawer, ChatSidebar } from "./components/WatchlistChat";
import { ThemeToggle, useAuth, AuthButton, ButuhLogin } from "./components/auth";
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

const NAV_ALL = [
  { href: "/", icon: "📊", label: "Dashboard" },
  { href: "/routines", icon: "🗓️", label: "Routines" },
  { href: "/screener", icon: "🔍", label: "Screener" },
  { href: "/alerts", icon: "🔔", label: "Alerts" },
  { href: "/portfolio", icon: "💼", label: "Portfolio" },
];

function Nav(props: { loggedIn: boolean }) {
  const loc = useLocation();
  // Belum login: hanya Dashboard yg terlihat (fitur lain di-hide).
  const items = () => props.loggedIn ? NAV_ALL : NAV_ALL.slice(0, 1);
  return (
    <nav class="flex w-56 shrink-0 flex-col gap-1 border-r bg-card p-4 max-lg:hidden">
      <div class="mb-4 px-2 text-xl font-bold tracking-tight">
        Flow<span class="text-primary">Sight</span>
      </div>
      {items().map((n) => (
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

function MobileNav(props: { loggedIn: boolean }) {
  const loc = useLocation();
  const items = () => props.loggedIn ? NAV_ALL : NAV_ALL.slice(0, 1);
  return (
    <nav class="fixed inset-x-0 bottom-0 z-40 flex border-t bg-card px-2 py-1 lg:hidden">
      {items().map((n) => (
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

function TickerSearch(props: { loggedIn: boolean }) {
  const nav = useNavigate();
  const [q, setQ] = createSignal("");
  const go = () => {
    const t = q().toUpperCase().trim();
    // Report butuh login: arahkan yg belum login ke /login.
    if (t) nav(props.loggedIn ? `/report/${t}` : "/login");
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
  const err = () => new URLSearchParams(window.location.search).get("error");
  const [mode, setMode] = createSignal<"masuk" | "daftar">("masuk");
  const [username, setUsername] = createSignal("");
  const [password, setPassword] = createSignal("");
  const [busy, setBusy] = createSignal(false);
  const [formErr, setFormErr] = createSignal("");
  const [version] = createResource(() => api.version().catch(() => null));
  async function submit(e: Event) {
    e.preventDefault();
    setFormErr(""); setBusy(true);
    try {
      if (mode() === "daftar") await api.signup(username().trim(), password());
      else await api.login(username().trim(), password());
      window.location.href = "/";
    } catch (e2) { setFormErr(String(e2)); }
    finally { setBusy(false); }
  }
  return (
    <div class="mx-auto max-w-md py-[8vh]">
      <Card>
        <CardHeader class="text-center">
          <CardTitle class="text-2xl">Masuk ke FlowSight 👋</CardTitle>
          <CardDescription>
            Daftar gratis pakai username + password — watchlist, notifikasi,
            dan report jadi milik kamu sendiri.
          </CardDescription>
        </CardHeader>
        <CardContent class="space-y-3">
          <Show when={err()}><p class="text-sm text-destructive">Login gagal: {err()}</p></Show>
          <div class="flex gap-1 rounded-md bg-muted p-1">
            <Button size="sm" variant={mode() === "masuk" ? "default" : "ghost"} class="flex-1" onClick={() => setMode("masuk")}>Masuk</Button>
            <Button size="sm" variant={mode() === "daftar" ? "default" : "ghost"} class="flex-1" onClick={() => setMode("daftar")}>Daftar baru</Button>
          </div>
          <form class="space-y-3" onSubmit={submit}>
            <TextField><TextFieldInput placeholder="username (mis. budi)" value={username()} onInput={(e) => setUsername(e.currentTarget.value)} autocomplete="username" /></TextField>
            <TextField><TextFieldInput type="password" placeholder="password (min. 8 karakter)" value={password()} onInput={(e) => setPassword(e.currentTarget.value)} autocomplete={mode() === "daftar" ? "new-password" : "current-password"} /></TextField>
            <Show when={formErr()}><p class="text-sm text-destructive">{formErr()}</p></Show>
            <Button type="submit" size="lg" class="w-full" disabled={busy()}>{busy() ? "…" : mode() === "daftar" ? "Daftar & Masuk" : "Masuk"}</Button>
          </form>
          <Show when={version()?.google_configured}>
            <div class="pt-1 text-center">
              <p class="mb-2 text-xs text-muted-foreground">atau</p>
              <a href="/api/auth/start"><Button size="lg" variant="outline" class="w-full">Sign in with Google</Button></a>
            </div>
          </Show>
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
      <Nav loggedIn={!!me()} />
      <div class="flex min-w-0 flex-1 flex-col">
        <header class="sticky top-0 z-30 flex items-center gap-3 border-b bg-background/95 px-4 py-2.5 backdrop-blur">
          <TickerSearch loggedIn={!!me()} />
          <span class="flex-1" />
          <ThemeToggle />
          <AuthButton me={me()} onLogout={logout} />
        </header>
        <main class="mx-auto w-full max-w-6xl flex-1 space-y-6 p-4 pb-20 lg:p-6">{props.children}</main>
      </div>
      <aside class="hidden w-80 shrink-0 flex-col gap-4 border-l bg-card p-4 xl:flex">
        <Show when={me()} fallback={
          <Card><CardHeader class="pb-2"><CardTitle class="text-base">🔒 Login dulu</CardTitle></CardHeader>
          <CardContent class="space-y-2"><p class="text-sm text-muted-foreground">Watchlist, filter, dan AI chat milikmu sendiri setelah login.</p>
          <a href="/login"><Button size="sm" class="w-full">Masuk / Daftar</Button></a></CardContent></Card>
        }>
          <WatchlistDrawer />
          <ChatSidebar />
        </Show>
      </aside>
      <MobileNav loggedIn={!!me()} />
    </div>
  );
}

// Gate: bungkus halaman fitur — belum login tampilkan ButuhLogin.
export function Gate(props: { fitur: string; children: import("solid-js").JSX.Element }) {
  const { me } = useAuth();
  return (
    <Show when={me() !== null} fallback={<p class="text-sm text-muted-foreground">Memeriksa login…</p>}>
      <Show when={me()} fallback={<ButuhLogin fitur={props.fitur} />}>
        {props.children}
      </Show>
    </Show>
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
