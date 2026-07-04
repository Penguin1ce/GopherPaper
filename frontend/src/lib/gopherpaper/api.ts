// 与后端 /api/v1 对接的类型化客户端。沿用统一信封 { code, message, data },
// code !== 0 或 HTTP 非 2xx 视为失败,401 触发登出回调。

import type {
  ChatResponse,
  EntityGraph,
  Envelope,
  GraphRebuildJob,
  GraphStats,
  GraphTrends,
  LoginResponse,
  MindMap,
  MindMapGraph,
  AvatarResponse,
  Message,
  NameCount,
  AnnotationRect,
  Paper,
  PaperAnnotation,
  PaperCompareReport,
  PaperDeleteConfirmPayload,
  PaperDetail,
  PaperFlow,
  PaperFlowNodeDetail,
  PaperProgressPayload,
  PaperProgressResponse,
  PasswordResetCodePayload,
  RegisterPayload,
  ReaderContext,
  RelatedPaper,
  ReportsStatus,
  ReportType,
  ResetPasswordPayload,
  SendMessageResponse,
  Session,
  Topic,
  UpdateEmailPayload,
  UpdateProfilePayload,
  UpdateUserPreferencePayload,
  UserPreference,
  UserProfile,
} from "./types";

const API_BASE = process.env.NEXT_PUBLIC_API_BASE || "/api/v1";

export class ApiError extends Error {
  status: number;
  constructor(message: string, status: number) {
    super(message);
    this.status = status;
  }
}

let token = "";
let onUnauthorized: (() => void) | null = null;

export function setToken(value: string) {
  token = value || "";
}

// figureUrl 拼出取图接口地址,带 query token(img 标签发不了 Authorization 头)。
export function figureUrl(docId: string, imgName: string): string {
  return `${API_BASE}/papers/${encodeURIComponent(docId)}/figures/${encodeURIComponent(
    imgName,
  )}?token=${encodeURIComponent(token)}`;
}

// paperFileUrl 拼出取原始 PDF 的地址,带 query token,供精读页 pdf.js 加载。
export function paperFileUrl(id: string): string {
  return `${API_BASE}/papers/${encodeURIComponent(id)}/file?token=${encodeURIComponent(token)}`;
}

// translate 把精读页选中的英文原文送后端小模型翻成中文。
export function translate(id: string, text: string) {
  return request<{ translation: string }>(
    `/papers/${encodeURIComponent(id)}/translate`,
    { method: "POST", body: JSON.stringify({ text }) },
  );
}

export function updatePaperProgress(id: string, payload: PaperProgressPayload) {
  return request<PaperProgressResponse>(`/papers/${encodeURIComponent(id)}/progress`, {
    method: "PATCH",
    body: JSON.stringify(payload),
  });
}

export function listAnnotations(id: string) {
  return request<PaperAnnotation[]>(`/papers/${encodeURIComponent(id)}/annotations`);
}

export function createAnnotation(
  id: string,
  payload: {
    page_no: number;
    kind?: string;
    text: string;
    note?: string;
    translation?: string;
    color?: string;
    bounding_rect: AnnotationRect;
    rects: AnnotationRect[];
    style_json?: Record<string, unknown>;
    content_json?: Record<string, unknown>;
  },
) {
  return request<PaperAnnotation>(`/papers/${encodeURIComponent(id)}/annotations`, {
    method: "POST",
    body: JSON.stringify(payload),
  });
}

export function updateAnnotation(
  id: string,
  annotationID: number,
  payload: {
    text?: string;
    note?: string;
    translation?: string;
    color?: string;
    bounding_rect?: AnnotationRect;
    rects?: AnnotationRect[];
    style_json?: Record<string, unknown>;
    content_json?: Record<string, unknown>;
  },
) {
  return request<PaperAnnotation>(
    `/papers/${encodeURIComponent(id)}/annotations/${encodeURIComponent(annotationID)}`,
    { method: "PATCH", body: JSON.stringify(payload) },
  );
}

export function deleteAnnotation(id: string, annotationID: number) {
  return request<null>(
    `/papers/${encodeURIComponent(id)}/annotations/${encodeURIComponent(annotationID)}`,
    { method: "DELETE" },
  );
}

export function getMindMap(id: string) {
  return request<MindMap>(`/papers/${encodeURIComponent(id)}/mind-maps`);
}

export function buildMindMap(id: string) {
  return request<MindMap>(`/papers/${encodeURIComponent(id)}/mind-maps/build`, {
    method: "POST",
  });
}

export function updateMindMap(id: number, graph: MindMapGraph) {
  return request<MindMap>(`/mind-maps/${encodeURIComponent(id)}`, {
    method: "PUT",
    body: JSON.stringify({ graph }),
  });
}

export function syncMindMap(id: number) {
  return request<MindMap>(`/mind-maps/${encodeURIComponent(id)}/sync`, {
    method: "POST",
  });
}

export function setUnauthorizedHandler(fn: (() => void) | null) {
  onUnauthorized = fn;
}

export function clearUnauthorizedHandler(fn: () => void) {
  if (onUnauthorized === fn) onUnauthorized = null;
}

function authHeaders(
  extra: Record<string, string> = {},
): Record<string, string> {
  const headers: Record<string, string> = { ...extra };
  if (token) headers.Authorization = `Bearer ${token}`;
  return headers;
}

async function readEnvelope<T>(res: Response): Promise<T> {
  const text = await res.text();
  let body: Envelope<T> = { code: 0, message: "" };
  if (text) {
    try {
      body = JSON.parse(text);
    } catch {
      body = { code: res.ok ? 0 : res.status, message: text };
    }
  }
  if (!res.ok || body.code !== 0) {
    const message = body.message || `请求失败 ${res.status}`;
    if (res.status === 401 && onUnauthorized) onUnauthorized();
    throw new ApiError(message, res.status);
  }
  return body.data as T;
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers: authHeaders({
      "Content-Type": "application/json",
      ...(options.headers as Record<string, string>),
    }),
  });
  return readEnvelope<T>(res);
}

// ---- 鉴权 ----

export function login(account: string, password: string) {
  return request<LoginResponse>("/user/login", {
    method: "POST",
    body: JSON.stringify({ account, password }),
  });
}

// logout best-effort 通知后端清登录态与该用户常驻的 agent/模型缓存。
// 故意不走 request/readEnvelope:失败静默(本地登出才是关键),也避免 401 触发
// 全局未授权处理形成「登出又登出」的循环。须在 setToken("") 清 token 前调用,才能带上 Authorization。
export async function logout(): Promise<void> {
  if (!token) return;
  try {
    await fetch(`${API_BASE}/user/logout`, {
      method: "POST",
      headers: authHeaders({ "Content-Type": "application/json" }),
    });
  } catch {
    // 忽略网络错误,本地登出照常进行
  }
}

export function register(payload: RegisterPayload) {
  return request<null>("/user/register", {
    method: "POST",
    body: JSON.stringify(payload),
  });
}

export function sendCode(email: string) {
  return request<null>("/user/send-code", {
    method: "POST",
    body: JSON.stringify({ email }),
  });
}

export function sendPasswordResetCode(payload: PasswordResetCodePayload) {
  return request<null>("/user/password-reset/send-code", {
    method: "POST",
    body: JSON.stringify(payload),
  });
}

export function resetPassword(payload: ResetPasswordPayload) {
  return request<null>("/user/password-reset", {
    method: "POST",
    body: JSON.stringify(payload),
  });
}

export function me() {
  return request<UserProfile>("/user/me");
}

export async function updateProfile(payload: UpdateProfilePayload) {
  const body = JSON.stringify(payload);
  try {
    return await request<UserProfile>("/user/profile", {
      method: "POST",
      body,
    });
  } catch (err) {
    if (err instanceof ApiError && err.status === 404) {
      return request<UserProfile>("/user/profile", {
        method: "PATCH",
        body,
      });
    }
    throw err;
  }
}

export function updateEmail(payload: UpdateEmailPayload) {
  return request<UserProfile>("/user/email", {
    method: "POST",
    body: JSON.stringify(payload),
  });
}

export function preferences() {
  return request<UserPreference>("/user/preferences");
}

export function updatePreferences(payload: UpdateUserPreferencePayload) {
  return request<UserPreference>("/user/preferences", {
    method: "POST",
    body: JSON.stringify(payload),
  });
}

export async function uploadAvatar(file: File): Promise<AvatarResponse> {
  const formData = new FormData();
  formData.append("file", file);
  const res = await fetch(`${API_BASE}/user/avatar`, {
    method: "POST",
    headers: authHeaders(),
    body: formData,
  });
  return readEnvelope<AvatarResponse>(res);
}

export function clearAvatar() {
  return request<AvatarResponse>("/user/avatar", {
    method: "DELETE",
  });
}

export function userModelConfigRequest<T>(
  path: string,
  options: RequestInit = {},
) {
  return request<T>(path, options);
}

// ---- 论文 ----

export function listPapers() {
  return request<Paper[]>("/papers");
}

export function searchPapers(q: string) {
  return request<Paper[]>(`/papers/search?q=${encodeURIComponent(q)}`);
}

export function comparePapers(paperIDs: string[]) {
  return request<PaperCompareReport>("/papers/compare", {
    method: "POST",
    body: JSON.stringify({ paper_ids: paperIDs }),
  });
}

export function listCompareReports() {
  return request<PaperCompareReport[]>("/papers/compare/reports");
}

export function deleteCompareReport(id: number) {
  return request<null>(`/papers/compare/reports/${encodeURIComponent(id)}`, {
    method: "DELETE",
  });
}

export function paperStatus(id: string) {
  return request<
    Pick<
      Paper,
      | "id"
      | "status"
      | "fail_reason"
      | "parse_progress"
      | "parsed_pages"
      | "total_pages"
    >
  >(`/papers/${encodeURIComponent(id)}/status`);
}

export function paperDetail(id: string) {
  return request<PaperDetail>(`/papers/${encodeURIComponent(id)}`);
}

export function rebuildPaperSections(id: string) {
  return request<NonNullable<PaperDetail["sections"]>>(
    `/papers/${encodeURIComponent(id)}/sections/rebuild`,
    { method: "POST" },
  );
}

export function deletePaper(id: string) {
  return request<null>(`/papers/${encodeURIComponent(id)}`, {
    method: "DELETE",
  });
}

export function reparsePaper(id: string) {
  return request<Paper>(`/papers/${encodeURIComponent(id)}/reparse`, {
    method: "POST",
  });
}

export function generateReport(id: string, type: ReportType) {
  return request<ChatResponse>(`/papers/${encodeURIComponent(id)}/report`, {
    method: "POST",
    body: JSON.stringify({ type }),
  });
}

// getPaperFlow 只读取已生成的思路图缓存,不会触发生成。
export function getPaperFlow(id: string) {
  return request<ChatResponse>(`/papers/${encodeURIComponent(id)}/flow`);
}

// generatePaperFlow 为某篇论文生成小云雀同款研究思路图,返回 meta.flow。
// 后端持久化缓存,论文未就绪时返回 409。
export async function generatePaperFlow(
  id: string,
  stream?: Pick<SendStreamHandlers, "onPaperFlow" | "onPaperFlowNode">,
) {
  if (!stream) {
    return request<ChatResponse>(`/papers/${encodeURIComponent(id)}/flow`, {
      method: "POST",
    });
  }
  const res = await fetch(`${API_BASE}/papers/${encodeURIComponent(id)}/flow`, {
    method: "POST",
    headers: authHeaders({ Accept: "text/event-stream" }),
  });
  const ctype = res.headers.get("content-type") || "";
  if (!res.ok || !ctype.includes("text/event-stream") || !res.body) {
    return readEnvelope<ChatResponse>(res);
  }

  const reader = res.body.getReader();
  const decoder = new TextDecoder();
  let buf = "";
  let result: ChatResponse | null = null;
  let errMsg = "";
  let finished = false;
  const handleFrame = (frame: string) => {
    const parsed = parseSSEFrame(frame);
    if (!parsed) return;
    const payload = parsed.payload as Record<string, unknown>;
    switch (parsed.event) {
      case "paper_flow":
        stream.onPaperFlow?.(parsed.payload as PaperFlow);
        break;
      case "paper_flow_node":
        stream.onPaperFlowNode?.(parsed.payload as PaperFlowNodeDetail);
        break;
      case "done":
        result = parsed.payload as ChatResponse;
        finished = true;
        break;
      case "error":
        errMsg = String(payload.message ?? "处理失败");
        finished = true;
        break;
    }
  };
  for (;;) {
    const { done, value } = await reader.read();
    if (done) {
      buf += decoder.decode();
      if (buf.trim()) handleFrame(buf);
      break;
    }
    buf += decoder.decode(value, { stream: true });
    let idx: number;
    while ((idx = buf.indexOf("\n\n")) >= 0) {
      handleFrame(buf.slice(0, idx));
      buf = buf.slice(idx + 2);
    }
    if (finished) {
      await reader.cancel().catch(() => {});
      break;
    }
  }
  if (errMsg) throw new ApiError(errMsg, res.status);
  if (!result) throw new ApiError("连接中断,请重试", res.status);
  return result;
}

// reportStatus 拉取某篇论文已生成与生成中的研读报告状态,只读,不触发生成。
export async function reportStatus(id: string): Promise<ReportsStatus> {
  const res = await request<Partial<ReportsStatus>>(
    `/papers/${encodeURIComponent(id)}/reports`,
  );
  return {
    ready: res.ready ?? [],
    running: res.running ?? [],
    flow_ready: Boolean(res.flow_ready),
  };
}

// listReports 拉取某篇论文已生成的研读报告类型,只读,不触发生成。
export async function listReports(id: string): Promise<ReportType[]> {
  return (await reportStatus(id)).ready;
}

export async function uploadPaper(file: File): Promise<Paper> {
  const formData = new FormData();
  formData.append("file", file);
  const res = await fetch(`${API_BASE}/papers`, {
    method: "POST",
    headers: authHeaders(),
    body: formData,
  });
  return readEnvelope<Paper>(res);
}

// ---- 知识图谱 ----

export function graphOverview() {
  return request<GraphStats>("/graph/overview");
}

export function graphTrends(keywordTop = 8) {
  return request<GraphTrends>(`/graph/trends?keyword_top=${keywordTop}`);
}

export function graphKeywords(top = 20) {
  return request<NameCount[]>(`/graph/keywords?top=${top}`);
}

export function graphNetwork() {
  return request<EntityGraph>("/graph/network");
}

export function graphNetworkEntities() {
  return request<EntityGraph>("/graph/network/entities");
}

export function rebuildGraphNetwork() {
  return request<GraphRebuildJob>("/graph/network/rebuild", { method: "POST" });
}

export function graphRebuildJob(id: string) {
  return request<GraphRebuildJob>(
    `/graph/network/rebuild/${encodeURIComponent(id)}`,
  );
}

export function paperEntityGraph(id: string) {
  return request<EntityGraph>(`/graph/papers/${encodeURIComponent(id)}`);
}

export function rebuildPaperEntityGraph(id: string) {
  return request<EntityGraph>(
    `/graph/papers/${encodeURIComponent(id)}/rebuild`,
    {
      method: "POST",
    },
  );
}

export function relatedPapers(id: string, limit = 10) {
  return request<RelatedPaper[]>(
    `/graph/papers/${encodeURIComponent(id)}/related?limit=${limit}`,
  );
}

// ---- 会话与消息 ----

export function listSessions() {
  return request<Session[]>("/sessions");
}

export function listTopics() {
  return request<Topic[]>("/topics");
}

// backfillTopics 触发存量小云雀会话的主题回填,返回待处理会话数。后端异步处理。
export function backfillTopics() {
  return request<{ count: number }>("/topics/backfill", { method: "POST" });
}

// clearTopics 清空当前用户的全部小云雀主题,会话退回未归类。演示重置归类用。
export function clearTopics() {
  return request<{ removed: number }>("/topics", { method: "DELETE" });
}

export function createSession(
  title: string,
  paperID?: string,
  agentType?: string,
) {
  const payload: { title: string; paper_id?: string; agent_type?: string } = {
    title,
  };
  if (paperID) payload.paper_id = paperID;
  if (agentType) payload.agent_type = agentType;
  return request<Session>("/sessions", {
    method: "POST",
    body: JSON.stringify(payload),
  });
}

export function deleteSession(id: string) {
  return request<null>(`/sessions/${encodeURIComponent(id)}`, {
    method: "DELETE",
  });
}

export function listMessages(sessionID: string) {
  return request<Message[]>(
    `/sessions/${encodeURIComponent(sessionID)}/messages`,
  );
}

// 发消息 SSE 的过程回调:onDelta 收应答文本增量,onTool 收工具调用状态(done=false 发起/true 返回),
// onPlan 收先锋者规划/动作阶段文本增量(phase 为 planning/replanning/action/reasoning)。
export interface SendStreamHandlers {
  // reset 为 true 时表示新一轮答案开始,调用方应先清空已流式正文再追加(先锋者多轮只展示末轮)。
  onDelta?: (text: string, reset?: boolean) => void;
  onTool?: (tool: string, done: boolean) => void;
  onPlan?: (phase: string, content: string) => void;
  onConfirmDeletePaper?: (payload: PaperDeleteConfirmPayload) => void;
  // onPaperFlow 收 generate_paper_flow 推送的思路图骨架,前端先画结构。
  onPaperFlow?: (payload: PaperFlow) => void;
  // onPaperFlowNode 收逐节点补齐的 detail,前端据此逐个点亮节点。
  onPaperFlowNode?: (payload: PaperFlowNodeDetail) => void;
}

// 解析一帧 SSE(event + data 行),返回事件名与 JSON 载荷,无 data 返回 null。
function parseSSEFrame(
  frame: string,
): { event: string; payload: unknown } | null {
  let event = "message";
  const dataLines: string[] = [];
  for (const line of frame.split("\n")) {
    if (line.startsWith("event:")) event = line.slice(6).trim();
    else if (line.startsWith("data:")) dataLines.push(line.slice(5).trim());
  }
  if (dataLines.length === 0) return null;
  try {
    return { event, payload: JSON.parse(dataLines.join("\n")) };
  } catch {
    return null;
  }
}

// extraHeaders 透传额外请求头,小云雀页用它带 X-Luckin-Token 等凭据头,服务端不落库。
// 后端以 SSE 推送生成过程:tool_call/tool_result/delta 实时回调,done 事件收尾返回完整应答;
// 开流前的错误仍是普通 JSON 信封,沿用统一错误处理。
export async function sendMessage(
  sessionID: string,
  query: string,
  extraHeaders?: Record<string, string>,
  stream?: SendStreamHandlers,
  readerContext?: ReaderContext,
  signal?: AbortSignal,
): Promise<SendMessageResponse> {
  const body: { query: string; reader_context?: ReaderContext } = { query };
  if (readerContext) body.reader_context = readerContext;
  const res = await fetch(
    `${API_BASE}/sessions/${encodeURIComponent(sessionID)}/messages`,
    {
      method: "POST",
      headers: authHeaders({
        "Content-Type": "application/json",
        Accept: "text/event-stream",
        ...extraHeaders,
      }),
      body: JSON.stringify(body),
      signal,
    },
  );
  const ctype = res.headers.get("content-type") || "";
  if (!res.ok || !ctype.includes("text/event-stream") || !res.body) {
    return readEnvelope<SendMessageResponse>(res);
  }

  const reader = res.body.getReader();
  const decoder = new TextDecoder();
  let buf = "";
  let result: SendMessageResponse | null = null;
  let errMsg = "";
  let finished = false;
  const handleFrame = (frame: string) => {
    const parsed = parseSSEFrame(frame);
    if (!parsed) return;
    const payload = parsed.payload as Record<string, unknown>;
    switch (parsed.event) {
      case "delta":
        stream?.onDelta?.(
          String(payload.content ?? ""),
          Boolean(payload.reset),
        );
        break;
      case "plan":
        stream?.onPlan?.(
          String(payload.phase ?? ""),
          String(payload.content ?? ""),
        );
        break;
      case "tool_call":
        stream?.onTool?.(String(payload.tool ?? ""), false);
        break;
      case "tool_result":
        stream?.onTool?.(String(payload.tool ?? ""), true);
        break;
      case "confirm_delete_paper":
        stream?.onConfirmDeletePaper?.(
          parsed.payload as PaperDeleteConfirmPayload,
        );
        break;
      case "paper_flow":
        stream?.onPaperFlow?.(parsed.payload as PaperFlow);
        break;
      case "paper_flow_node":
        stream?.onPaperFlowNode?.(parsed.payload as PaperFlowNodeDetail);
        break;
      case "done":
        result = parsed.payload as SendMessageResponse;
        finished = true;
        break;
      case "error":
        errMsg = String(payload.message ?? "处理失败");
        finished = true;
        break;
    }
  };
  for (;;) {
    const { done, value } = await reader.read();
    if (done) {
      buf += decoder.decode();
      if (buf.trim()) handleFrame(buf);
      break;
    }
    buf += decoder.decode(value, { stream: true });
    let idx: number;
    while ((idx = buf.indexOf("\n\n")) >= 0) {
      handleFrame(buf.slice(0, idx));
      buf = buf.slice(idx + 2);
    }
    if (finished) {
      await reader.cancel().catch(() => {});
      break;
    }
  }
  if (errMsg) throw new ApiError(errMsg, res.status);
  if (!result) throw new ApiError("连接中断,请重试", res.status);
  return result;
}

// ---- WebSocket 解析进度 ----

export interface PaperStatusEvent {
  type: string;
  paper_id: string;
  status: Paper["status"];
  detail?: string;
  parse_progress?: number;
  parsed_pages?: number;
  total_pages?: number;
}

// ReportReadyEvent 是研读报告生成完成的就绪通知。
export interface ReportReadyEvent {
  type: string;
  paper_id: string;
  report_type: ReportType;
}

// ReportProgressEvent 是研读报告生成过程的阶段进度,phase 包含 preparing/researching/writing/reviewing
// 以及 researcher 内部的 planning/action/reasoning/replanning;生成失败时为 failed。
export interface ReportProgressEvent {
  type: string;
  paper_id: string;
  report_type: ReportType;
  phase: string;
  detail?: string;
}

export interface CompareProgressEvent {
  type: string;
  paper_ids: string[];
  phase: string;
  detail?: string;
}

type WsMessage =
  | PaperStatusEvent
  | ReportReadyEvent
  | ReportProgressEvent
  | CompareProgressEvent;

// sseBase 给长连 SSE 选基址:开发期固定直连 Go 后端,绕开 Next/Nginx 所在页面源。
// EventSource 长期占用 HTTP/1.1 连接;如果和页面路由、HMR、普通 API 同源,快速切页时容易顶满
// 浏览器同源连接池。普通 API 仍走 API_BASE,只有 SSE 分流到 :8080。
// NEXT_PUBLIC_SSE_ORIGIN 可显式指定后端源;未设且 dev 时按当前主机推 :8080。
function sseBase(): string {
  const origin = process.env.NEXT_PUBLIC_SSE_ORIGIN;
  if (origin) return `${origin.replace(/\/+$/, "")}/api/v1`;
  if (process.env.NODE_ENV === "development" && typeof window !== "undefined") {
    return `${window.location.protocol}//${window.location.hostname}:8080/api/v1`;
  }
  return API_BASE;
}

// openStatusStream 用 SSE 订阅解析进度与报告就绪。
// 基址见 sseBase:dev 直连后端、prod 同源走代理;EventSource 自带断线重连,调用方登出时 close。
export function openStatusStream(
  jwt: string,
  onEvent: (e: PaperStatusEvent) => void,
  onReport?: (e: ReportReadyEvent) => void,
  onProgress?: (e: ReportProgressEvent) => void,
  onCompareProgress?: (e: CompareProgressEvent) => void,
): EventSource | null {
  if (!jwt || typeof EventSource === "undefined") return null;
  const url = `${sseBase()}/events?token=${encodeURIComponent(jwt)}`;
  let source: EventSource;
  try {
    source = new EventSource(url);
  } catch {
    return null;
  }
  source.addEventListener("message", (event) => {
    let msg: WsMessage;
    try {
      msg = JSON.parse(event.data) as WsMessage;
    } catch {
      return;
    }
    if (!msg) return;
    if (msg.type === "compare_progress") {
      onCompareProgress?.(msg as CompareProgressEvent);
      return;
    }
    if (!("paper_id" in msg) || !msg.paper_id) return;
    if (msg.type === "paper_status") onEvent(msg as PaperStatusEvent);
    else if (msg.type === "report_ready") onReport?.(msg as ReportReadyEvent);
    else if (msg.type === "report_progress")
      onProgress?.(msg as ReportProgressEvent);
  });
  return source;
}
