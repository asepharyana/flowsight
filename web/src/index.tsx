import { render } from "solid-js/web";
import { Router, Route, useLocation } from "@solidjs/router";
import { WatchlistDrawer, ChatSidebar } from "./components/WatchlistChat";
import Dashboard from "./pages/Dashboard";
import Routines from "./pages/Routines";
import Screener from "./pages/Screener";
import Alerts from "./pages/Alerts";
import Report from "./pages/Report";
import Portfolio from "./pages/Portfolio";
import "./styles/tokens.css";

function Nav() {
  const loc = useLocation();
  const link = (href: string, label: string) => (
    <a href={href} class={loc.pathname === href ? "active" : ""}>{label}</a>
  );
  return (
    <nav class="fs-nav">
      <strong>FlowSight</strong>
      {link("/", "Dashboard")}
      {link("/routines", "Routines")}
      {link("/screener", "Screener")}
      {link("/alerts", "Alerts")}
      {link("/portfolio", "Portfolio")}
    </nav>
  );
}

render(
  () => (
    <Router root={(props) => (
      <div class="fs-app">
        <Nav />
        <main class="fs-main">{props.children}</main>
        <aside style={{ width: "300px", padding: "20px 16px 20px 0", display: "flex", "flex-direction": "column", gap: "12px" }}>
          <WatchlistDrawer />
          <ChatSidebar />
        </aside>
      </div>
    )}>
      <Route path="/" component={Dashboard} />
      <Route path="/routines" component={Routines} />
      <Route path="/screener" component={Screener} />
      <Route path="/alerts" component={Alerts} />
      <Route path="/report/:ticker" component={Report} />
      <Route path="/portfolio" component={Portfolio} />
    </Router>
  ),
  document.getElementById("root")!,
);
