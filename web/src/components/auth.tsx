import { createResource, createSignal, Show } from "solid-js";
import { api, type AuthUser } from "../lib/api";

// AuthState: loads /api/auth/me once; exposes login/logout + theme toggle.
function theme(): string {
  return localStorage.getItem("fs-theme") ||
    (matchMedia("(prefers-color-scheme: light)").matches ? "light" : "dark");
}

export function applyTheme(t: string) {
  document.documentElement.dataset.theme = t;
  localStorage.setItem("fs-theme", t);
}
applyTheme(theme());

export function ThemeToggle() {
  const [t, setT] = createSignal(theme());
  return (
    <button class="fs-iconbtn" title="Toggle light/dark" onClick={() => {
      const n = t() === "dark" ? "light" : "dark";
      setT(n); applyTheme(n);
    }}>
      {t() === "dark" ? "☀️" : "🌙"}
    </button>
  );
}

export function useAuth() {
  const [me, { refetch }] = createResource(async (): Promise<AuthUser | null> => {
    try { return (await api.me()).user; } catch { return null; }
  });
  return { me, refetch };
}

export function AuthButton(props: { me: AuthUser | null | undefined; onLogout: () => void }) {
  return (
    <Show when={props.me} fallback={
      <a class="fs-btn" href="/api/auth/start">Sign in with Google</a>
    }>
      {(u) => (
        <span class="fs-user">
          {u().avatar_url ? <img class="fs-avatar" src={u().avatar_url} alt="" /> : null}
          {u().name || u().email}
          <button class="fs-iconbtn" onClick={props.onLogout}>Logout</button>
        </span>
      )}
    </Show>
  );
}
