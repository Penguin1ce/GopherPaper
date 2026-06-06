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

export function createSession(title: string, paperID?: string) {
  const payload: { title: string; paper_id?: string } = { title };
  if (paperID) payload.paper_id = paperID;
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

export function sendMessage(sessionID: string, query: string) {
  return request<SendMessageResponse>(
    `/sessions/${encodeURIComponent(sessionID)}/messages`,
    { method: "POST", body: JSON.stringify({ query }) },
  );
}

// ---- WebSocket 解析进度 ----

export interface PaperStatusEvent {
  type: string;
  paper_id: string;
  status: Paper["status"];
  detail?: string;
}

export function openStatusSocket(
  jwt: string,
  onEvent: (e: PaperStatusEvent) => void,
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
    let msg: PaperStatusEvent;
    try {
      msg = JSON.parse(event.data);
    } catch {
      return;
    }
    if (!msg || msg.type !== "paper_status" || !msg.paper_id) return;
    onEvent(msg);
  });
  return socket;
}
