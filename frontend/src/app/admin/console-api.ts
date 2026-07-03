import { dashboardRequest } from "./dashboard-api";

// ── 用户管理 ─────────────────────────────────────────────────

export type UserItem = {
  id: number;
  student_id: string;
  name: string;
  email: string;
  class_id: string;
  avatar_url?: string;
  paper_count: number;
  session_count: number;
  call_count: number;
  created_at: string;
  last_active_at?: string;
};

export type UserListResponse = {
  items: UserItem[];
  total: number;
  page: number;
  page_size: number;
};

export type UserStatusCount = { status: string; count: number };

export type UserActivityPoint = {
  date: string;
  papers: number;
  sessions: number;
  calls: number;
};

export type PaperItem = {
  id: string;
  title: string;
  file_name: string;
  status: string;
  page_count: number;
  size: number;
  created_at: string;
};

export type UserDetail = {
  user: UserItem;
  paper_status_distribution: UserStatusCount[];
  activity: UserActivityPoint[];
  recent_papers: PaperItem[];
};

export type ClassStat = {
  class_id: string;
  user_count: number;
  paper_count: number;
};

export type ClassStatsResponse = { items: ClassStat[] };

export function fetchUsers(
  token: string,
  params: { page?: number; page_size?: number; query?: string; class_id?: string } = {},
) {
  const q = new URLSearchParams();
  q.set("page", String(params.page ?? 1));
  q.set("page_size", String(params.page_size ?? 10));
  if (params.query?.trim()) q.set("query", params.query.trim());
  if (params.class_id?.trim()) q.set("class_id", params.class_id.trim());
  return dashboardRequest<UserListResponse>(`/admin/users?${q}`, token);
}

export function fetchUserDetail(token: string, id: number) {
  return dashboardRequest<UserDetail>(`/admin/users/${id}`, token);
}

export function fetchClassStats(token: string) {
  return dashboardRequest<ClassStatsResponse>("/admin/classes", token);
}

// ── 服务调用日志 ─────────────────────────────────────────────

export type LogItem = {
  id: number;
  service_type: string;
  actor_id?: string;
  paper_id?: string;
  session_id?: string;
  success: boolean;
  duration_ms: number;
  error_message?: string;
  created_at: string;
};

export type LogListResponse = {
  items: LogItem[];
  total: number;
  page: number;
  page_size: number;
};

export type LogStats = {
  service_types: string[];
  total: number;
  success: number;
  failed: number;
  avg_ms: number;
  max_ms: number;
};

export function fetchLogs(
  token: string,
  params: {
    page?: number;
    page_size?: number;
    service_type?: string;
    result?: string;
    actor?: string;
  } = {},
) {
  const q = new URLSearchParams();
  q.set("page", String(params.page ?? 1));
  q.set("page_size", String(params.page_size ?? 20));
  if (params.service_type?.trim()) q.set("service_type", params.service_type.trim());
  if (params.result?.trim()) q.set("result", params.result.trim());
  if (params.actor?.trim()) q.set("actor", params.actor.trim());
  return dashboardRequest<LogListResponse>(`/admin/logs?${q}`, token);
}

export function fetchLogStats(token: string) {
  return dashboardRequest<LogStats>("/admin/logs/stats", token);
}

// ── 高级分析 ─────────────────────────────────────────────────

export type LatencyBucket = { label: string; count: number };
export type HourPoint = { hour: number; count: number; success: number };
export type TopActor = { actor_id: string; calls: number };
export type PipelineStage = { stage: string; count: number };

export type AdvancedAnalytics = {
  latency_histogram: LatencyBucket[];
  hourly_distribution: HourPoint[];
  top_actors: TopActor[];
  pipeline: PipelineStage[];
};

export function fetchAdvancedAnalytics(token: string) {
  return dashboardRequest<AdvancedAnalytics>("/admin/analytics/advanced", token);
}

// 小工具:格式化 -----------------------------------------------

export function formatBytes(value: number): string {
  if (!value) return "—";
  const units = ["B", "KB", "MB", "GB"];
  let size = value;
  let unit = 0;
  while (size >= 1024 && unit < units.length - 1) {
    size /= 1024;
    unit += 1;
  }
  return `${size.toFixed(unit === 0 ? 0 : 1)} ${units[unit]}`;
}

export function formatDateTime(value: string): string {
  if (!value) return "—";
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return "—";
  return new Intl.DateTimeFormat("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(d);
}

export function relativeTime(value: string): string {
  const t = new Date(value).getTime();
  if (Number.isNaN(t)) return "";
  const diff = Date.now() - t;
  const min = Math.floor(diff / 60000);
  if (min < 1) return "刚刚";
  if (min < 60) return `${min}分钟前`;
  const h = Math.floor(min / 60);
  if (h < 24) return `${h}小时前`;
  return `${Math.floor(h / 24)}天前`;
}
