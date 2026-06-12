// 与后端 internal/model 与 internal/dto 一一对应的前端类型。

export type PaperStatus =
  | "uploaded"
  | "parsing"
  | "extracted"
  | "indexed"
  | "ready"
  | "failed";

export interface Paper {
  id: string;
  owner_id: string;
  title: string;
  file_name: string;
  file_uri: string;
  size: number;
  status: PaperStatus;
  fail_reason?: string;
  page_count: number;
  category?: string;
  progress: number;
  created_at: string;
  updated_at: string;
}

export interface PaperMeta {
  paper_id: string;
  authors: string[] | null;
  affiliations: string[] | null;
  abstract: string;
  keywords: string[] | null;
  research_questions: string[] | null;
  methods: string;
  experiments: string;
  results: string;
  innovations: string[] | null;
  limitations: string[] | null;
  future_work: string[] | null;
  updated_at: string;
}

export interface PaperSection {
  id: number;
  paper_id: string;
  level: number;
  title: string;
  page_no: number;
  order_idx: number;
}

export interface PaperDetail {
  paper: Paper;
  meta: PaperMeta | null;
  sections: PaperSection[] | null;
}

export interface Session {
  id: string;
  student_id: string;
  paper_id?: string;
  // 空为默认论文助教,"pioneer" 为小云雀会话(独立页 /pioneer)。
  agent_type?: string;
  title: string;
  created_at: string;
  updated_at: string;
}

export type IntentType = "fact" | "summary" | "method" | string;

export interface Message {
  id: number | string;
  session_id: string;
  role: "user" | "assistant" | "system";
  content: string;
  intent?: IntentType;
  created_at: string;
  // 助教消息可能带本轮引用出处等结构化信息。
  meta?: Record<string, unknown>;
  // 前端瞬态字段,仅 SSE 进行中的占位消息使用,不来自后端。
  streaming?: boolean;
}

// Reply.Meta["sources"] 透出的出处结构。
export interface Reference {
  source_file?: string;
  source_uri?: string;
  page_no?: number;
  chunk_index?: number;
  knowledge_scope?: string;
  doc_id?: string;
  block_type?: string; // image 时为图块,配合 img_name 渲染缩略图
  img_name?: string; // 图片文件名,与 doc_id 拼取图接口
  [k: string]: unknown;
}

export interface SendMessageResponse {
  message: Message;
  meta?: Record<string, unknown>;
}

export interface ChatResponse {
  intent: string;
  content: string;
  meta?: Record<string, unknown>;
}

export interface LoginResponse {
  token: string;
  student_id: string;
  name: string;
  email: string;
}

export interface AuthUser {
  student_id: string;
  name: string;
  email: string;
}

export type ReportType =
  | "quickread"
  | "method"
  | "result"
  | "innovation"
  | "compare"
  | "future";

export interface RegisterPayload {
  student_id: string;
  name: string;
  email: string;
  class_id: string;
  password: string;
  code: string;
}

// 后端统一响应信封 { code, message, data }。
export interface Envelope<T> {
  code: number;
  message: string;
  data?: T;
}
