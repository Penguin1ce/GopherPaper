import type { Paper, PaperStatus, Session } from "./types";

export const READY_STATUSES: PaperStatus[] = ["ready"];

export function isSettled(status: PaperStatus): boolean {
  return READY_STATUSES.includes(status) || status === "failed";
}

export function paperTitle(paper: Paper): string {
  return paper.title || paper.file_name || "未命名论文";
}

export function sessionTitle(session: Session): string {
  return session.title || "未命名会话";
}

function sessionTime(s: Session): number {
  const t = new Date(s.updated_at || s.created_at).getTime();
  return Number.isNaN(t) ? 0 : t;
}

// 主应用只展示论文助教会话,小云雀会话归独立页 /pioneer 管。
export function chatSessions(sessions: Session[]): Session[] {
  return sessions.filter((s) => s.agent_type !== "pioneer");
}

// 某篇论文的会话,按最近活动倒序(最新在前)。paperID 为空时原样返回全部。
export function sessionsForPaper(
  sessions: Session[],
  paperID: string,
): Session[] {
  if (!paperID) return sessions;
  return sessions
    .filter((s) => s.paper_id === paperID)
    .sort((a, b) => sessionTime(b) - sessionTime(a));
}

export function formatTime(value?: string): string {
  if (!value) return "";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "";
  return date.toLocaleString("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  });
}

export function formatSize(bytes?: number): string {
  if (!Number.isFinite(bytes) || (bytes ?? 0) <= 0) return "未知大小";
  const b = bytes as number;
  if (b < 1024 * 1024) return `${(b / 1024).toFixed(1)} KB`;
  return `${(b / 1024 / 1024).toFixed(1)} MB`;
}

const STATUS_LABELS: Record<string, string> = {
  uploaded: "待解析",
  parsing: "解析中",
  extracted: "抽取中",
  indexed: "索引中",
  ready: "完成",
  failed: "失败",
};

export function statusLabel(status?: string): string {
  return STATUS_LABELS[status ?? ""] || status || "未知";
}

export type StatusTone = "ready" | "working" | "failed";

export function statusTone(status?: PaperStatus): StatusTone {
  if (status === "ready") return "ready";
  if (status === "failed") return "failed";
  return "working";
}

const INTENT_LABELS: Record<string, string> = {
  chitchat: "闲聊",
  summary: "概括解释",
  method: "方法解读",
  pioneer: "小云雀",
};

export function intentLabel(intent?: string): string {
  if (!intent) return "";
  return INTENT_LABELS[intent] || intent;
}

// 解析进度状态机的步进序号,用于进度条。
const STATUS_STEP: Record<string, number> = {
  uploaded: 0,
  parsing: 1,
  extracted: 2,
  indexed: 3,
  ready: 4,
  failed: -1,
};

export const PARSE_STEPS = ["上传", "解析", "抽取", "索引", "就绪"];

export function statusStep(status?: PaperStatus): number {
  return STATUS_STEP[status ?? ""] ?? 0;
}
