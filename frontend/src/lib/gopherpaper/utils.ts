import type { Message, Paper, PaperStatus, PlanStep, Session } from "./types";
import { toolDisplayInfo } from "./tool-status";

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

export function metaPlanSteps(meta?: Record<string, unknown>): PlanStep[] {
  const raw = meta?.steps;
  if (!Array.isArray(raw)) return [];
  return raw
    .map((item) => {
      if (!item || typeof item !== "object") return null;
      const step = item as Record<string, unknown>;
      const phase = typeof step.phase === "string" ? step.phase.trim() : "";
      const text = typeof step.text === "string" ? step.text.trim() : "";
      const kind = step.kind === "tool" || step.kind === "plan" ? step.kind : undefined;
      const tool = typeof step.tool === "string" ? step.tool.trim() : "";
      const status =
        step.status === "running" || step.status === "done" || step.status === "failed"
          ? step.status
          : undefined;
      return phase && text
        ? {
            phase,
            text,
            ...(kind ? { kind } : {}),
            ...(tool ? { tool } : {}),
            ...(status ? { status } : {}),
          }
        : null;
    })
    .filter((step): step is PlanStep => Boolean(step));
}

export function messagePlan(message?: Message | null): PlanStep[] {
  if (!message) return [];
  if (message.plan && message.plan.length > 0) return message.plan;
  return metaPlanSteps(message.meta);
}

export function isToolPlanStep(step: PlanStep): boolean {
  if (step.kind === "tool") return true;
  if (step.phase !== "action") return false;
  return toolDisplayInfo(step.tool || step.text).matched;
}

export function processPlanSteps(steps: PlanStep[]): PlanStep[] {
  return steps.filter((step) => !isToolPlanStep(step));
}

export function toolPlanSteps(steps: PlanStep[]): PlanStep[] {
  return steps.filter(isToolPlanStep);
}

export function ensurePlanningStep(
  steps: PlanStep[],
  text = "分析问题，制定检索策略",
) {
  if (steps.some((s) => s.phase === "planning" || s.phase === "replanning")) return;
  steps.push({ phase: "planning", text, kind: "plan" });
}

function toolStepMatches(step: PlanStep, tool: string) {
  const info = toolDisplayInfo(tool);
  const raw = info.raw.trim();
  return (
    (raw && step.tool === raw) ||
    step.text.trim() === raw ||
    step.text.trim() === info.name
  );
}

export function pushToolCallStep(steps: PlanStep[], tool: string) {
  const info = toolDisplayInfo(tool);
  const last = steps[steps.length - 1];
  if (
    last &&
    last.phase === "action" &&
    last.kind === "tool" &&
    last.status === "running" &&
    toolStepMatches(last, info.raw)
  ) {
    return;
  }
  steps.push({
    phase: "action",
    text: info.name,
    kind: "tool",
    tool: info.raw,
    status: "running",
  });
}

export function finishToolCallStep(steps: PlanStep[], tool: string) {
  const info = toolDisplayInfo(tool);
  for (let i = steps.length - 1; i >= 0; i -= 1) {
    const step = steps[i];
    if (step.phase !== "action") continue;
    if (step.kind === "tool" && (toolStepMatches(step, info.raw) || !step.tool)) {
      steps[i] = { ...step, status: "done" };
      return;
    }
  }
  steps.push({
    phase: "action",
    text: info.name,
    kind: "tool",
    tool: info.raw,
    status: "done",
  });
}

export function pushReasoningStep(
  steps: PlanStep[],
  text = "综合检索结果，整理回答",
) {
  const last = steps[steps.length - 1];
  if (last && last.phase === "reasoning") return;
  if (steps.some((s) => s.phase === "reasoning" && s.text.trim() === text)) return;
  steps.push({ phase: "reasoning", text, kind: "plan" });
}

function sessionTime(s: Session): number {
  const t = new Date(s.updated_at || s.created_at).getTime();
  return Number.isNaN(t) ? 0 : t;
}

// 主应用只展示论文助教会话,小云雀与精读页小耄耋归各自入口管理。
export function chatSessions(sessions: Session[]): Session[] {
  return sessions.filter((s) => s.agent_type !== "pioneer" && s.agent_type !== "maodie");
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
  maodie: "小耄耋",
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
