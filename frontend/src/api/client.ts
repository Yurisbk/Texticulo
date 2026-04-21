const base = (import.meta.env.VITE_API_URL || "").replace(/\/$/, "");

export function apiUrl(path: string): string {
  if (!path.startsWith("/")) path = "/" + path;
  if (base) return `${base}${path}`;
  return path;
}

export type ShortenResponse = { short_code: string; short_url: string };

export async function shortenUrl(
  url: string,
  token?: string | null
): Promise<ShortenResponse> {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
  };
  if (token) headers.Authorization = `Bearer ${token}`;
  const res = await fetch(apiUrl("/api/shorten"), {
    method: "POST",
    headers,
    body: JSON.stringify({ url }),
  });
  if (res.status === 429) throw new Error("rate_limit");
  if (res.status === 403) {
    const data = await res.json().catch(() => ({}));
    if (data.error === "link_limit") throw new Error("link_limit");
  }
  if (!res.ok) throw new Error("shorten_failed");
  return res.json() as Promise<ShortenResponse>;
}

export type LinkRow = {
  short_code: string;
  original_url: string;
  clicks: number;
  created_at: string;
  short_url: string;
};

export async function listLinks(token: string): Promise<LinkRow[]> {
  const res = await fetch(apiUrl("/api/links"), {
    headers: { Authorization: `Bearer ${token}` },
  });
  if (!res.ok) throw new Error("list_failed");
  const data = (await res.json()) as { links: LinkRow[] };
  return data.links;
}

export type StatsResponse = {
  short_code: string;
  original_url: string;
  clicks: number;
  created_at: string;
  short_url: string;
  recent_clicks: string[];
};

export async function linkStats(
  code: string,
  token: string
): Promise<StatsResponse> {
  const res = await fetch(apiUrl(`/api/links/${encodeURIComponent(code)}/stats`), {
    headers: { Authorization: `Bearer ${token}` },
  });
  if (!res.ok) throw new Error("stats_failed");
  return res.json() as Promise<StatsResponse>;
}

export async function deleteLink(code: string, token: string): Promise<void> {
  const res = await fetch(apiUrl(`/api/links/${encodeURIComponent(code)}`), {
    method: "DELETE",
    headers: { Authorization: `Bearer ${token}` },
  });
  if (!res.ok) throw new Error("delete_failed");
}

export type AuthResponse = { token: string; email: string };

export async function loginEmail(email: string, password: string): Promise<AuthResponse> {
  const res = await fetch(apiUrl("/api/auth/login"), {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ email, password }),
  });
  if (res.status === 429) throw new Error("too_many_attempts");
  if (res.status === 401) {
    const data = await res.json().catch(() => ({})) as { error?: string };
    throw new Error(data.error ?? "invalid_credentials");
  }
  if (!res.ok) throw new Error("login_failed");
  return res.json() as Promise<AuthResponse>;
}

export async function registerEmail(email: string, password: string): Promise<AuthResponse> {
  const res = await fetch(apiUrl("/api/auth/register"), {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ email, password }),
  });
  if (res.status === 409) throw new Error("email_taken");
  if (res.status === 400) {
    const data = await res.json().catch(() => ({})) as { error?: string };
    throw new Error(data.error ?? "register_failed");
  }
  if (!res.ok) throw new Error("register_failed");
  return res.json() as Promise<AuthResponse>;
}

export type MetricsResponse = {
  total_links: number;
  total_clicks: number;
  total_users: number;
  clicks_today: number;
  top_links: Array<{
    short_code: string;
    short_url: string;
    clicks: number;
  }>;
  clicks_last_7_days: Array<{
    date: string;
    count: number;
  }>;
};

export async function getMetrics(): Promise<MetricsResponse> {
  const res = await fetch(apiUrl("/api/metrics"));
  if (!res.ok) throw new Error("metrics_failed");
  return res.json() as Promise<MetricsResponse>;
}
