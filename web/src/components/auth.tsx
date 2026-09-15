import { createResource, createSignal, Show } from "solid-js";
import { api, type AuthUser } from "../lib/api";
import { Button } from "./ui/button";
import { Avatar, AvatarFallback, AvatarImage } from "./ui/avatar";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "./ui/card";

// Theme: .dark class on <html> (shadcn convention). Default dark.
function theme(): string {
  return localStorage.getItem("fs-theme") || "dark";
}

export function applyTheme(t: string) {
  document.documentElement.classList.toggle("dark", t === "dark");
  localStorage.setItem("fs-theme", t);
}
applyTheme(theme());

export function ThemeToggle() {
  const [t, setT] = createSignal(theme());
  return (
    <Button variant="ghost" size="icon" title="Toggle light/dark" onClick={() => {
      const n = t() === "dark" ? "light" : "dark";
      setT(n); applyTheme(n);
    }}>
      {t() === "dark" ? "☀️" : "🌙"}
    </Button>
  );
}

export function useAuth() {
  const [me, { refetch }] = createResource(async (): Promise<AuthUser | null> => {
    try { return (await api.me()).user; } catch { return null; }
  });
  const [version] = createResource(async () => {
    try { return await api.version(); } catch { return null; }
  });
  return { me, refetch, version };
}

// ButuhLogin: kartu ajakan login untuk halaman fitur yg di-hide bila logout.
export function ButuhLogin(props: { fitur: string }) {
  return (
    <div class="mx-auto max-w-md py-[6vh]">
      <Card>
        <CardHeader class="text-center">
          <CardTitle class="text-2xl">🔒 {props.fitur} perlu login</CardTitle>
          <CardDescription>
            Biar datanya milik kamu sendiri (watchlist, notifikasi, report) —
            daftar gratis, cukup username + password.
          </CardDescription>
        </CardHeader>
        <CardContent class="text-center">
          <a href="/login"><Button size="lg">Masuk / Daftar</Button></a>
        </CardContent>
      </Card>
    </div>
  );
}

export function AuthButton(props: { me: AuthUser | null | undefined; onLogout: () => void }) {
  return (
    <Show when={props.me} fallback={
      <a href="/api/auth/start"><Button>Sign in with Google</Button></a>
    }>
      {(u) => (
        <span class="flex items-center gap-2">
          <Avatar class="size-8">
            {u().avatar_url ? <AvatarImage src={u().avatar_url} alt="" /> : null}
            <AvatarFallback>{(u().name || u().email || "?").slice(0, 1).toUpperCase()}</AvatarFallback>
          </Avatar>
          <span class="hidden text-sm font-medium md:inline">{u().name || u().email}</span>
          <Button variant="ghost" size="sm" onClick={props.onLogout}>Logout</Button>
        </span>
      )}
    </Show>
  );
}
