import { dashboardRequest } from "./dashboard-api";

const API_BASE = process.env.NEXT_PUBLIC_API_BASE || "/api/v1";

// downloadExport 带鉴权头拉取 CSV 并触发浏览器下载。
export async function downloadExport(token: string, kind: "papers" | "users" | "logs") {
  const res = await fetch(`${API_BASE}/admin/export/${kind}`, {
    headers: { Authorization: `Bearer ${token}` },
  });
  if (!res.ok) throw new Error("导出失败");
  const blob = await res.blob();
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = `${kind}.csv`;
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(url);
}

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

// ── 操作审计 ─────────────────────────────────────────────────

export type AuditItem = {
  id: number;
  admin_id: number;
  admin_name: string;
  method: string;
  path: string;
  status: number;
  ip: string;
  created_at: string;
};

export type AuditListResponse = {
  items: AuditItem[];
  total: number;
  page: number;
  page_size: number;
};

export function fetchAudit(token: string, params: { page?: number; page_size?: number } = {}) {
  const q = new URLSearchParams();
  q.set("page", String(params.page ?? 1));
  q.set("page_size", String(params.page_size ?? 20));
  return dashboardRequest<AuditListResponse>(`/admin/audit?${q}`, token);
}

// ── 系统设置 ─────────────────────────────────────────────────

export type SettingItem = { key: string; value: string; group: string; label: string; type: string };
export type SettingsResponse = { items: SettingItem[] };

export function fetchSettings(token: string) {
  return dashboardRequest<SettingsResponse>("/admin/settings", token);
}

export function updateSettings(token: string, items: { key: string; value: string }[]) {
  return dashboardRequest<null>("/admin/settings", token, {
    method: "PUT",
    body: JSON.stringify({ items }),
  });
}

// ── 站内公告 ─────────────────────────────────────────────────

export type AnnouncementItem = {
  id: number;
  title: string;
  content: string;
  level: string;
  published: boolean;
  author_name: string;
  created_at: string;
  updated_at: string;
};
export type AnnouncementListResponse = {
  items: AnnouncementItem[];
  total: number;
  page: number;
  page_size: number;
};
export type AnnouncementBody = { title: string; content: string; level: string; published: boolean };

export function fetchAnnouncements(token: string, params: { page?: number; page_size?: number } = {}) {
  const q = new URLSearchParams();
  q.set("page", String(params.page ?? 1));
  q.set("page_size", String(params.page_size ?? 10));
  return dashboardRequest<AnnouncementListResponse>(`/admin/announcements?${q}`, token);
}

export function createAnnouncement(token: string, body: AnnouncementBody) {
  return dashboardRequest<AnnouncementItem>("/admin/announcements", token, {
    method: "POST",
    body: JSON.stringify(body),
  });
}

export function updateAnnouncement(token: string, id: number, body: AnnouncementBody) {
  return dashboardRequest<AnnouncementItem>(`/admin/announcements/${id}`, token, {
    method: "PUT",
    body: JSON.stringify(body),
  });
}

export function deleteAnnouncement(token: string, id: number) {
  return dashboardRequest<null>(`/admin/announcements/${id}`, token, { method: "DELETE" });
}

// ── 用户反馈 ─────────────────────────────────────────────────

export type FeedbackItem = {
  id: number;
  student_id: string;
  category: string;
  content: string;
  status: string;
  reply: string;
  handler_name: string;
  created_at: string;
  updated_at: string;
};
export type FeedbackListResponse = {
  items: FeedbackItem[];
  total: number;
  page: number;
  page_size: number;
};

export function fetchFeedbacks(
  token: string,
  params: { page?: number; page_size?: number; status?: string } = {},
) {
  const q = new URLSearchParams();
  q.set("page", String(params.page ?? 1));
  q.set("page_size", String(params.page_size ?? 10));
  if (params.status?.trim()) q.set("status", params.status.trim());
  return dashboardRequest<FeedbackListResponse>(`/admin/feedbacks?${q}`, token);
}

export function updateFeedback(token: string, id: number, body: { status: string; reply: string }) {
  return dashboardRequest<null>(`/admin/feedbacks/${id}`, token, {
    method: "PUT",
    body: JSON.stringify(body),
  });
}

// ── 标签管理 ─────────────────────────────────────────────────

export type TagItem = { id: number; owner_id: string; name: string; paper_count: number };
export type TagListResponse = { items: TagItem[]; total: number };

export function fetchTags(token: string) {
  return dashboardRequest<TagListResponse>("/admin/tags", token);
}
export function renameTag(token: string, id: number, name: string) {
  return dashboardRequest<null>(`/admin/tags/${id}`, token, {
    method: "PUT",
    body: JSON.stringify({ name }),
  });
}
export function deleteTag(token: string, id: number) {
  return dashboardRequest<null>(`/admin/tags/${id}`, token, { method: "DELETE" });
}
export function mergeTags(token: string, source: number, target: number) {
  return dashboardRequest<null>(`/admin/tags/merge?source=${source}&target=${target}`, token, {
    method: "POST",
  });
}

// ── 会话管理 ─────────────────────────────────────────────────

export type SessionItem = {
  id: string;
  student_id: string;
  paper_id?: string;
  agent_type?: string;
  title: string;
  created_at: string;
};
export type SessionListResponse = {
  items: SessionItem[];
  total: number;
  page: number;
  page_size: number;
};

export function fetchSessions(
  token: string,
  params: { page?: number; page_size?: number; agent_type?: string } = {},
) {
  const q = new URLSearchParams();
  q.set("page", String(params.page ?? 1));
  q.set("page_size", String(params.page_size ?? 15));
  if (params.agent_type?.trim()) q.set("agent_type", params.agent_type.trim());
  return dashboardRequest<SessionListResponse>(`/admin/sessions?${q}`, token);
}
export function deleteSession(token: string, id: string) {
  return dashboardRequest<null>(`/admin/sessions/${id}`, token, { method: "DELETE" });
}

// ── 管理员账号 ───────────────────────────────────────────────

export type AdminAccount = {
  id: number;
  username: string;
  email: string;
  name: string;
  status: string;
  last_login_at?: string;
  created_at: string;
};
export type AdminAccountListResponse = { items: AdminAccount[] };

export function fetchAdmins(token: string) {
  return dashboardRequest<AdminAccountListResponse>("/admin/admins", token);
}
export function setAdminStatus(token: string, id: number, status: string) {
  return dashboardRequest<null>(`/admin/admins/${id}/status`, token, {
    method: "PUT",
    body: JSON.stringify({ status }),
  });
}

// ── 论文批量操作 ─────────────────────────────────────────────

export type BatchPaper = {
  id: string;
  title: string;
  file_name: string;
  status: string;
  owner_id: string;
  created_at: string;
};
export type BatchPaperList = {
  items: BatchPaper[];
  total: number;
  page: number;
  page_size: number;
};
export type BatchResult = {
  requested: number;
  succeeded: number;
  failed: number;
  failed_ids?: string[];
};

export function fetchPapersForBatch(token: string, params: { page?: number; page_size?: number; query?: string } = {}) {
  const q = new URLSearchParams();
  q.set("page", String(params.page ?? 1));
  q.set("page_size", String(params.page_size ?? 20));
  if (params.query?.trim()) q.set("query", params.query.trim());
  return dashboardRequest<BatchPaperList>(`/admin/papers?${q}`, token);
}

export function batchDeletePapers(token: string, ids: string[]) {
  return dashboardRequest<BatchResult>("/admin/papers/batch-delete", token, {
    method: "POST",
    body: JSON.stringify({ ids }),
  });
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
