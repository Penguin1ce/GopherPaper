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
  keywords?: string[];
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
  // 自动归类的主题 ID,空表示尚未归类。仅小云雀会话归类。
  topic_id?: string;
  title: string;
  created_at: string;
  updated_at: string;
}

// Topic 是小云雀会话的自动归类主题,供侧边栏按主题分组。
export interface Topic {
  id: string;
  student_id: string;
  agent_type?: string;
  name: string;
  member_count: number;
  created_at: string;
  updated_at: string;
}

export type IntentType = "chitchat" | "summary" | "method" | "pioneer" | string;

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
  // 先锋者执行计划段,仅本轮内存保留(不入库、刷新即失),用于气泡内折叠回看。
  plan?: PlanStep[];
  // 论文思路图谱,小云雀调 generate_paper_flow 时经 SSE 推送,仅本轮内存保留(不入库、刷新即失)。
  flow?: PaperFlow;
}

// PaperFlowNode 思路图节点,type 取固定枚举供前端配色。
export interface PaperFlowNode {
  id: string;
  // problem/gap/idea/method/experiment/result/conclusion
  type: string;
  label: string;
  detail?: string;
}

// PaperFlowEdge 思路图有向边,label 表达节点间推进关系。
export interface PaperFlowEdge {
  from: string;
  to: string;
  label?: string;
}

// PaperFlowFigure 是挂在主节点旁的论文真实配图(缩略图),不参与主线推进。
export interface PaperFlowFigure {
  id: string;
  parent: string;
  doc_id: string;
  img_name: string;
  page_no?: number;
  caption?: string;
}

// PaperFlow 是 generate_paper_flow 工具推送的思路图骨架载荷(detail 与 figures 随后逐节点补齐)。
export interface PaperFlow {
  paper_id: string;
  title: string;
  nodes: PaperFlowNode[];
  edges: PaperFlowEdge[];
  figures?: PaperFlowFigure[];
}

// PaperFlowNodeDetail 是某节点补齐的 detail 与可选配图,逐条经 paper_flow_node 事件推达。
export interface PaperFlowNodeDetail {
  paper_id: string;
  node_id: string;
  detail: string;
  figure?: PaperFlowFigure;
}

// PlanStep 是先锋者 plan-execute 的一个阶段段落,phase 区分规划/动作/思考。
export interface PlanStep {
  phase: string;
  text: string;
}

// ReportRun 是某篇论文某类研读报告一次生成的实时进度:执行计划步 + 是否进行中 + 是否失败。
export interface ReportRun {
  steps: PlanStep[];
  live: boolean;
  failed: boolean;
}

export interface ReportRunStatus extends ReportRun {
  type: ReportType;
}

export interface ReportsStatus {
  ready: ReportType[];
  running: ReportRunStatus[];
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

export interface PaperDeleteConfirmPayload {
  paper_id: string;
  title: string;
  file_name?: string;
  status?: string;
  confirmation_token: string;
  message?: string;
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
  avatar_url?: string;
  class_id?: string;
}

export interface AuthUser {
  student_id: string;
  name: string;
  email: string;
  avatar_url?: string;
  class_id?: string;
}

export interface AvatarResponse {
  avatar_url: string;
}

export type UserProfile = AuthUser;

export interface UpdateProfilePayload {
  name: string;
}

export interface UpdateEmailPayload {
  email: string;
  code: string;
}

export interface PasswordResetCodePayload {
  student_id: string;
  email: string;
}

export interface ResetPasswordPayload extends PasswordResetCodePayload {
  code: string;
  password: string;
}

export type ReportType =
  | "quickread"
  | "method"
  | "result"
  | "innovation"
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

// ---- 知识图谱(Neo4j)----

// GraphStats 是当前用户图谱的规模总览,对应 /graph/overview。
export interface GraphStats {
  papers: number;
  authors: number;
  keywords: number;
  citations: number;
  min_year: number;
  max_year: number;
}

// YearCount 是某年的论文数,用于年度趋势。
export interface YearCount {
  year: number;
  count: number;
}

// KeywordYearCount 是某关键词在某年的出现次数,用于关键词热度演化。
export interface KeywordYearCount {
  keyword: string;
  year: number;
  count: number;
}

// NameCount 是名称与计数,用于热门关键词。
export interface NameCount {
  name: string;
  count: number;
}

// GraphTrends 对应 /graph/trends。
export interface GraphTrends {
  by_year: YearCount[];
  keyword_trend: KeywordYearCount[];
}

export interface EntityGraphNode {
  id: string;
  type: string;
  label: string;
  details?: Record<string, string>;
}

export interface EntityGraphEdge {
  id: string;
  source: string;
  target: string;
  type: string;
  label: string;
}

export interface EntityGraph {
  nodes: EntityGraphNode[];
  edges: EntityGraphEdge[];
}

// RelatedPaper 是与某篇论文相关的论文,vias 标关系类型 author/keyword/cocitation/cites。
export interface RelatedPaper {
  id: string;
  title: string;
  year: number;
  score: number;
  vias: string[];
}
