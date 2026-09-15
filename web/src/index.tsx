import { render } from "solid-js/web";
import { Router, Route, useLocation, useNavigate, useParams, type RouteSectionProps } from "@solidjs/router";
import { createSignal, Show } from "solid-js";
import { WatchlistDrawer, ChatSidebar } from "./components/WatchlistChat";
import { ThemeToggle, useAuth, AuthButton } from "./components/auth";
import { api } from "./lib/api";
import Dashboard from "./pages/Dashboard";
import Routines from "./pages/Routines";
import Screener from "./pages/Screener";
import Alerts from "./pages/Alerts";
import Report from "./pages/Report";
import Portfolio from "./pages/Portfolio";
import "./styles/tokens.css";

function Nav() {
  const loc = useLocation();
  const link = (href: string, icon: string, label: string) => (
    <a href={href} class={loc.pathname === href ? "active" : ""}><span>{icon}</span> {label}</a>
  );
  return (
    <nav class="fs-nav" id="fs-nav">
      <div class="fs-brand">Flow<span>Sight</span></div>
      {link("/", "📊", "Dashboard")}
      {link("/routines", "🗓️", "Routines")}
      {link("/screener", "🔍", "Screener")}
      {link("/alerts", "🔔", "Alerts")}
      {link("/portfolio", "💼", "Portfolio")}
      <div class="fs-nav-foot">
        <ThemeToggle />
      </div>
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
    <form class="fs-search" onSubmit={(e) => { e.preventDefault(); go(); }}>
      <input class="fs-input" placeholder="Ticker → report (e.g. BBCA)" value={q()}
        onInput={(e) => setQ(e.currentTarget.value)} style={{ flex: 1 }} />
      <button class="fs-btn" type="submit">Open</button>
    </form>
  );
}

function LoginPage() {
  const params = useParams<{ error?: string }>();
  const err = () => new URLSearchParams(window.location.search).get("error");
  void params;
  return (
    <div style={{ "max-width": "440px", margin: "8vh auto" }}>
      <div class="fs-card" style={{ padding: "28px", "text-align": "center" }}>
        <h1 style={{ margin: "0 0 8px" }}>Welcome to FlowSight</h1>
        <p class="fs-muted">Sign in with Google to get your own watchlist, routines, alerts and reports.</p>
        <Show when={err()}><p style={{ color: "var(--fs-avoid)" }}>Login failed: {err()}</p></Show>
        <a class="fs-btn" href="/api/auth/start" style={{ display: "inline-block", "margin-top": "12px" }}>Sign in with Google</a>
      </div>
    </div>
  );
}

function Shell(props: RouteSectionProps) {
  const { me, refetch } = useAuth();
  const logout = async () => { await api.logout(); refetch(); };
  return (
    <div class="fs-app">
      <Nav />
      <div class="fs-body">
        <header class="fs-topbar">
          <button id="fs-menu-btn" class="fs-iconbtn" onClick={() => document.getElementById("fs-nav")?.classList.toggle("open")}>☰</button>
          <TickerSearch />
          <span class="fs-spacer" />
          <ThemeToggle />
          <AuthButton me={me()} onLogout={logout} />
        </header>
        <main class="fs-main">{props.children}</main>
      </div>
      <aside class="fs-side">
        <WatchlistDrawer />
        <ChatSidebar />
      </aside>
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
