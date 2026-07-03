const API_BASE = process.env.NEXT_PUBLIC_API_BASE || "/api/v1";

export type TrendPoint = {
  date: string;
  papers: number;
  calls: number;
  success: number;
  failed: number;
  users: number;
  sessions: number;
  avg_latency_ms: number;
};

export type StatusSlice = { status: string; count: number };

export type ServiceStat = {
  service_type: string;
  total: number;
  success: number;
  failed: number;
  avg_ms: number;
  max_ms: number;
};

export type Analytics = {
  days: number;
  trend: TrendPoint[];
  status_distribution: StatusSlice[];
  services: ServiceStat[];
  total_papers: number;
  total_users: number;
  total_calls: number;
  total_sessions: number;
};

export type HealthItem = {
  name: string;
  status: "up" | "down";
  latency_ms: number;
  detail?: string;
};

export type Health = {
  items: HealthItem[];
  checked_at: string;
  healthy: number;
  total: number;
};

export type ActivityItem = {
  type: "paper" | "user" | "call";
  title: string;
  subtitle?: string;
  status?: string;
  created_at: string;
};

export type Activity = { items: ActivityItem[] };

export type SeedResult = {
  users: number;
  papers: number;
  calls: number;
  sessions: number;
};

export async function dashboardRequest<T>(
  path: string,
  token: string,
  options: RequestInit = {},
): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers: {
      "Content-Type": "application/json",
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...(options.headers as Record<string, string> | undefined),
    },
  });
  const text = await res.text();
  let body: { code: number; message: string; data?: T } = { code: 0, message: "" };
  if (text) {
    try {
      body = JSON.parse(text);
    } catch {
      body = { code: res.ok ? 0 : res.status, message: text };
    }
  }
  if (!res.ok || body.code !== 0) {
    throw new Error(body.message || `请求失败 ${res.status}`);
  }
  return body.data as T;
}
