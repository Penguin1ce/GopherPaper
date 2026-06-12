// 与后端 /api/v1 对接的类型化客户端。沿用统一信封 { code, message, data },
// code !== 0 或 HTTP 非 2xx 视为失败,401 触发登出回调。

import type {
  ChatResponse,
  Envelope,
  LoginResponse,
  Message,
  Paper,
  PaperDetail,
  RegisterPayload,
  ReportType,
  SendMessageResponse,
  Session,
} from "./types";

const API_BASE = "/api/v1";

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

export function setUnauthorizedHandler(fn: () => void) {
  onUnauthorized = fn;
}

function authHeaders(extra: Record<string, string> = {}): Record<string, string> {
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

export function login(studentID: string, password: string) {
  return request<LoginResponse>("/user/login", {
    method: "POST",
    body: JSON.stringify({ student_id: studentID, password }),
  });
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

// ---- 论文 ----

export function listPapers() {
  return request<Paper[]>("/papers");
}

export function searchPapers(q: string) {
  return request<Paper[]>(`/papers/search?q=${encodeURIComponent(q)}`);
}

export function paperStatus(id: string) {
  return request<Pick<Paper, "id" | "status" | "fail_reason">>(
    `/papers/${encodeURIComponent(id)}/status`,
  );
}

export function paperDetail(id: string) {
  return request<PaperDetail>(`/papers/${encodeURIComponent(id)}`);
}

export function generateReport(id: string, type: ReportType) {
  return request<ChatResponse>(`/papers/${encodeURIComponent(id)}/report`, {
    method: "POST",
    body: JSON.stringify({ type }),
  });
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

// ---- 会话与消息 ----

export function listSessions() {
  return request<Session[]>("/sessions");
}

export function createSession(title: string, paperID?: string, agentType?: string) {
  const payload: { title: string; paper_id?: string; agent_type?: string } = { title };
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

// 发消息 SSE 的过程回调:onDelta 收应答文本增量,onTool 收工具调用状态(done=false 发起/true 返回)。
export interface SendStreamHandlers {
  onDelta?: (text: string) => void;
  onTool?: (tool: string, done: boolean) => void;
}

// 解析一帧 SSE(event + data 行),返回事件名与 JSON 载荷,无 data 返回 null。
function parseSSEFrame(frame: string): { event: string; payload: unknown } | null {
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

// extraHeaders 透传额外请求头,先锋者页用它带 X-Luckin-Token 等凭据头,服务端不落库。
// 后端以 SSE 推送生成过程:tool_call/tool_result/delta 实时回调,done 事件收尾返回完整应答;
// 开流前的错误仍是普通 JSON 信封,沿用统一错误处理。
export async function sendMessage(
  sessionID: string,
  query: string,
  extraHeaders?: Record<string, string>,
  stream?: SendStreamHandlers,
): Promise<SendMessageResponse> {
  const res = await fetch(
    `${API_BASE}/sessions/${encodeURIComponent(sessionID)}/messages`,
    {
      method: "POST",
      headers: authHeaders({
        "Content-Type": "application/json",
        Accept: "text/event-stream",
        ...extraHeaders,
      }),
      body: JSON.stringify({ query }),
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
  const handleFrame = (frame: string) => {
    const parsed = parseSSEFrame(frame);
    if (!parsed) return;
    const payload = parsed.payload as Record<string, unknown>;
    switch (parsed.event) {
      case "delta":
        stream?.onDelta?.(String(payload.content ?? ""));
        break;
      case "tool_call":
        stream?.onTool?.(String(payload.tool ?? ""), false);
        break;
      case "tool_result":
        stream?.onTool?.(String(payload.tool ?? ""), true);
        break;
      case "done":
        result = parsed.payload as SendMessageResponse;
        break;
      case "error":
        errMsg = String(payload.message ?? "处理失败");
        break;
    }
  };
  for (;;) {
    const { done, value } = await reader.read();
    if (done) break;
    buf += decoder.decode(value, { stream: true });
    let idx: number;
    while ((idx = buf.indexOf("\n\n")) >= 0) {
      handleFrame(buf.slice(0, idx));
      buf = buf.slice(idx + 2);
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
}

// ReportReadyEvent 是研读报告后台预生成完成的就绪通知。
export interface ReportReadyEvent {
  type: string;
  paper_id: string;
  report_type: ReportType;
}

type WsMessage = PaperStatusEvent | ReportReadyEvent;

export function openStatusSocket(
  jwt: string,
  onEvent: (e: PaperStatusEvent) => void,
  onReport?: (e: ReportReadyEvent) => void,
): WebSocket | null {
  if (!jwt || typeof WebSocket === "undefined") return null;
  const proto = location.protocol === "https:" ? "wss:" : "ws:";
  const url = `${proto}//${location.host}${API_BASE}/ws?token=${encodeURIComponent(jwt)}`;
  let socket: WebSocket;
  try {
    socket = new WebSocket(url);
  } catch {
    return null;
  }
  socket.addEventListener("message", (event) => {
    let msg: WsMessage;
    try {
      msg = JSON.parse(event.data) as WsMessage;
    } catch {
      return;
    }
    if (!msg || !msg.paper_id) return;
    if (msg.type === "paper_status") onEvent(msg as PaperStatusEvent);
    else if (msg.type === "report_ready") onReport?.(msg as ReportReadyEvent);
  });
  return socket;
}
