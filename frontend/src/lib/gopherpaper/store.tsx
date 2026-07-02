"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import {
  QueryClient,
  QueryClientProvider,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";

import * as api from "./api";
import type {
  AuthUser,
  Message,
  Paper,
  PaperFlow,
  PasswordResetCodePayload,
  PlanStep,
  RegisterPayload,
  ReportRun,
  ReportsStatus,
  ReportType,
  ResetPasswordPayload,
  Session,
  UpdateEmailPayload,
  UpdateProfilePayload,
  UpdateUserPreferencePayload,
  UserPreference,
} from "./types";
import { toolStatusText } from "./tool-status";
import { chatSessions, isSettled, paperTitle, sessionsForPaper } from "./utils";

const AUTH_KEY = "gopherpaper.auth";

// sessions/messages/papers Query 未就绪时的稳定空数组,避免每次 render 新建 [] 触发下游 memo 失效。
const EMPTY_SESSIONS: Session[] = [];
const EMPTY_MESSAGES: Message[] = [];
const EMPTY_PAPERS: Paper[] = [];

const DEFAULT_PREFERENCE: UserPreference = {
  nickname: "",
  answer_style: "concise",
  output_format: "conclusion_first",
  language: "auto",
  custom_instruction: "",
};

// 工具名 → 执行过程里「检索」步的检索对象文案(左列已是「检索」标签,这里只写对象避免重复);
// 未列出的工具回退到原始工具名。
const TOOL_STEP_TEXT: Record<string, string> = {
  search_paper: "论文知识库",
  find_figures: "图表与表格",
};

export interface ToastItem {
  id: number;
  message: string;
  type: "ok" | "error";
}

interface PersistedAuth {
  user: AuthUser | null;
  token: string;
}

interface AppContextValue {
  // 状态
  user: AuthUser | null;
  preference: UserPreference;
  authed: boolean;
  papers: Paper[];
  sessions: Session[];
  messages: Message[];
  activePaperID: string;
  activeSessionID: string;
  sending: boolean;
  // 当前轮工具调用状态文案,SSE 进行中显示在输入框上方的状态气泡,空串隐藏
  toolNote: string;
  toasts: ToastItem[];
  activePaper: Paper | null;
  activeSession: Session | null;
  // 各论文已生成就绪的研读报告类型,供报告面板免轮询直接拉缓存
  reportReady: Record<string, Partial<Record<ReportType, boolean>>>;
  // 各论文各类报告一次生成的实时进度(执行计划/进行中/失败),由 report_progress 事件累积
  reportProgress: Record<string, Partial<Record<ReportType, ReportRun>>>;
  // 动作
  toast: (message: string, type?: "ok" | "error") => void;
  dismissToast: (id: number) => void;
  login: (account: string, password: string) => Promise<void>;
  registerAndLogin: (payload: RegisterPayload) => Promise<void>;
  sendCode: (email: string) => Promise<void>;
  sendPasswordResetCode: (payload: PasswordResetCodePayload) => Promise<void>;
  resetPassword: (payload: ResetPasswordPayload) => Promise<void>;
  logout: (notifyServer?: boolean) => void;
  refreshUser: () => Promise<void>;
  updateProfile: (payload: UpdateProfilePayload) => Promise<void>;
  updateEmail: (payload: UpdateEmailPayload) => Promise<void>;
  refreshPreferences: () => Promise<void>;
  updatePreferences: (payload: UpdateUserPreferencePayload) => Promise<void>;
  updateAvatar: (file: File) => Promise<void>;
  clearAvatar: () => Promise<void>;
  refreshPapers: (query?: string) => Promise<void>;
  uploadPaper: (file: File) => Promise<void>;
  reparsePaper: (id: string) => Promise<void>;
  removePaper: (id: string) => Promise<void>;
  selectPaper: (id: string) => void;
  refreshSessions: () => Promise<void>;
  openSession: (id: string) => Promise<void>;
  createSession: (title: string, paperID?: string) => Promise<Session>;
  removeSession: (id: string) => Promise<void>;
  sendMessage: (query: string) => Promise<void>;
  // 后端确认报告进入生成队列后调用,把该报告的进度置为「进行中」,后续阶段由 SSE 累积。
  beginReport: (paperID: string, type: ReportType) => void;
}

const AppContext = createContext<AppContextValue | null>(null);

function loadAuth(): PersistedAuth {
  if (typeof window === "undefined") return { user: null, token: "" };
  try {
    const raw = localStorage.getItem(AUTH_KEY);
    if (!raw) return { user: null, token: "" };
    const saved = JSON.parse(raw) as PersistedAuth;
    return { user: saved.user ?? null, token: saved.token ?? "" };
  } catch {
    localStorage.removeItem(AUTH_KEY);
    return { user: null, token: "" };
  }
}

function AppProviderInner({ children }: { children: ReactNode }) {
  const queryClient = useQueryClient();
  const [user, setUser] = useState<AuthUser | null>(null);
  const [preference, setPreference] = useState<UserPreference>(DEFAULT_PREFERENCE);
  const [token, setTokenState] = useState<string>("");
  // papers 由 Query 接管:paperSearch 空→listPapers,非空→searchPapers,搜索词进 query key。
  const [paperSearch, setPaperSearch] = useState("");
  const papersQuery = useQuery({
    queryKey: ["papers", paperSearch],
    enabled: Boolean(token),
    queryFn: async () => {
      const list = paperSearch
        ? await api.searchPapers(paperSearch)
        : await api.listPapers();
      return Array.isArray(list) ? list : [];
    },
  });
  const papers = papersQuery.data ?? EMPTY_PAPERS;
  // setPapers 包成 setQueriesData,对所有 papers 变体(列表+各搜索缓存)套用函数式 updater,
  // 故 SSE 推送(applyStatusEvent/applyReportReady)与 removePaper 的乐观改写沿用原逻辑。
  const setPapers = useCallback(
    (updater: (old: Paper[]) => Paper[]) => {
      queryClient.setQueriesData<Paper[]>({ queryKey: ["papers"] }, (old) =>
        old ? updater(old) : old,
      );
    },
    [queryClient],
  );
  // sessions 由 Query 接管:listSessions 拉取 + chatSessions 过滤;乐观更新走 setQueryData。
  const sessionsQuery = useQuery({
    queryKey: ["sessions"],
    enabled: Boolean(token),
    queryFn: async () => chatSessions(await api.listSessions()),
  });
  const sessions = sessionsQuery.data ?? EMPTY_SESSIONS;
  // setSessions 包成 setQueryData,兼容原 useState setter 签名(值或函数式 updater),
  // 故各处乐观更新调用点(createSession/removeSession/removePaper/logout)无需改写。
  const setSessions = useCallback(
    (updater: Session[] | ((old: Session[]) => Session[])) => {
      queryClient.setQueryData<Session[]>(["sessions"], (old = []) =>
        typeof updater === "function" ? updater(old) : updater,
      );
    },
    [queryClient],
  );
  const [activePaperID, setActivePaperID] = useState("");
  const [activeSessionID, setActiveSessionID] = useState("");
  // messages 由 Query 接管:按 activeSessionID 分缓存,切会话自动拉取/复用。
  // staleTime 让新建会话预置的空消息与流式写入不被 background refetch 覆盖。
  const messagesQuery = useQuery({
    queryKey: ["messages", activeSessionID],
    enabled: Boolean(activeSessionID),
    staleTime: 30_000,
    queryFn: async () => {
      const msgs = await api.listMessages(activeSessionID);
      return Array.isArray(msgs) ? msgs : [];
    },
  });
  const messages = messagesQuery.data ?? EMPTY_MESSAGES;
  const [sending, setSending] = useState(false);
  const [toolNote, setToolNote] = useState("");
  const [toasts, setToasts] = useState<ToastItem[]>([]);
  const [reportReady, setReportReady] = useState<
    Record<string, Partial<Record<ReportType, boolean>>>
  >({});
  const [reportProgress, setReportProgress] = useState<
    Record<string, Partial<Record<ReportType, ReportRun>>>
  >({});

  const wsRef = useRef<EventSource | null>(null);
  const toastSeq = useRef(0);
  const hydratedRef = useRef(false);
  const mountedRef = useRef(true);
  // papers 的同步镜像,供 applyStatusEvent 在 setPapers 更新函数之外读现状判重,
  // 避免把 toast 等副作用写进 updater(StrictMode 会双调 updater 导致弹两次)。
  const papersRef = useRef<Paper[]>([]);
  useEffect(() => {
    papersRef.current = papers;
  }, [papers]);
  // reportProgress 的同步镜像,供报告轮询兜底在 interval 闭包里读当前 live 集合,避免闭包陈旧。
  const reportProgressRef = useRef(reportProgress);
  useEffect(() => {
    reportProgressRef.current = reportProgress;
  }, [reportProgress]);

  useEffect(() => {
    api.setToken(token);
  }, [token]);

  // ---- Toast ----
  const dismissToast = useCallback((id: number) => {
    setToasts((list) => list.filter((t) => t.id !== id));
  }, []);

  const toast = useCallback(
    (message: string, type: "ok" | "error" = "ok") => {
      const id = ++toastSeq.current;
      setToasts((list) => [...list, { id, message, type }]);
      window.setTimeout(() => dismissToast(id), 3200);
    },
    [dismissToast],
  );

  // ---- 鉴权持久化 ----
  const persist = useCallback(
    (nextUser: AuthUser | null, nextToken: string) => {
      setUser(nextUser);
      setTokenState(nextToken);
      api.setToken(nextToken);
      if (nextToken) {
        localStorage.setItem(
          AUTH_KEY,
          JSON.stringify({ user: nextUser, token: nextToken }),
        );
      } else {
        localStorage.removeItem(AUTH_KEY);
      }
    },
    [],
  );

  const disconnectWs = useCallback(() => {
    if (wsRef.current) {
      const source = wsRef.current;
      wsRef.current = null;
      try {
        source.close();
      } catch {
        // 忽略关闭异常
      }
    }
  }, []);

  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
      disconnectWs();
    };
  }, [disconnectWs]);

  // notifyServer 为真时先通知后端清登录态与常驻缓存(趁 token 未清,fire-and-forget 不阻塞);
  // 401 被动登出时 token 已失效,传 false 跳过这次注定失败的请求。
  const logout = useCallback(
    (notifyServer = true) => {
      if (notifyServer) void api.logout();
      disconnectWs();
      persist(null, "");
      // 清空所有 Query 缓存(sessions/messages/papers/轮询),与下方业务 state 一并归零。
      queryClient.clear();
      setPaperSearch("");
      setActivePaperID("");
      setActiveSessionID("");
      setReportReady({});
      setReportProgress({});
      setPreference(DEFAULT_PREFERENCE);
    },
    [disconnectWs, persist, queryClient],
  );

  // 401 统一登出,被动登出不再回调后端(token 已失效)。
  const handleUnauthorized = useCallback(() => logout(false), [logout]);

  useEffect(() => {
    api.setUnauthorizedHandler(handleUnauthorized);
    return () => api.clearUnauthorizedHandler(handleUnauthorized);
  }, [handleUnauthorized]);

  // ---- WS 推送进度 ----
  const applyStatusEvent = useCallback(
    (event: api.PaperStatusEvent) => {
      const paperID = event.paper_id;
      const status = event.status;
      const detail = event.detail;
      // 判重与 toast 都在 updater 之外做:SSE 与轮询兜底会就同一状态各调一次,
      // updater 必须纯,副作用留在这里只触发一次。
      const existing = papersRef.current.find((p) => p.id === paperID);
      if (!existing) {
        // 列表里还没有(刚上传未刷新),失效 papers 缓存重拉补齐。
        void queryClient.invalidateQueries({ queryKey: ["papers"] });
        return;
      }
      const statusDetail = detail || existing.status_detail;
      const patch = {
        status,
        fail_reason:
          status === "failed"
            ? detail || existing.fail_reason
            : existing.fail_reason,
        status_detail: statusDetail,
        parse_progress: event.parse_progress ?? existing.parse_progress,
        parsed_pages: event.parsed_pages ?? existing.parsed_pages,
        total_pages: event.total_pages ?? existing.total_pages,
      };
      const sameStatus = existing.status === status;
      const sameProgress =
        existing.parse_progress === patch.parse_progress &&
        existing.parsed_pages === patch.parsed_pages &&
        existing.total_pages === patch.total_pages;
      const sameDetail = existing.status_detail === patch.status_detail;
      if (sameStatus && sameProgress && sameDetail) return;
      if (status === "ready") {
        toast(`「${paperTitle(existing)}」已就绪,可提问`);
      } else if (status === "failed") {
        toast(
          `「${paperTitle(existing)}」解析失败:${detail || "未知原因"}`,
          "error",
        );
      }
      // 先同步推进镜像,紧随其后的同状态事件(SSE/轮询)即被上面的判重拦掉。
      papersRef.current = papersRef.current.map((p) =>
        p.id === paperID ? { ...p, ...patch } : p,
      );
      setPapers((list) =>
        list.map((p) => (p.id === paperID ? { ...p, ...patch } : p)),
      );
      if ((status === "indexed" && !sameStatus) || status === "ready") {
        void queryClient.invalidateQueries({ queryKey: ["papers"] });
      }
    },
    [toast, setPapers, queryClient],
  );

  // 报告就绪:记入对应论文,报告面板据此免轮询直接拉缓存;同时把该报告进度收尾(停 live)。
  const applyReportReady = useCallback(
    (paperID: string, reportType: ReportType) => {
      setReportReady((prev) => ({
        ...prev,
        [paperID]: { ...prev[paperID], [reportType]: true },
      }));
      setReportProgress((prev) => {
        const run = prev[paperID]?.[reportType];
        if (!run) return prev;
        return {
          ...prev,
          [paperID]: {
            ...prev[paperID],
            [reportType]: { ...run, live: false },
          },
        };
      });
      setPapers((list) =>
        list.map((p) =>
          p.id === paperID && !isSettled(p.status)
            ? { ...p, status: "ready" }
            : p,
        ),
      );
    },
    [setPapers],
  );

  // beginReport 在后端返回 202 后置该报告为「进行中、空步」,随后由 SSE 阶段事件累积。
  const beginReport = useCallback((paperID: string, type: ReportType) => {
    setReportProgress((prev) => ({
      ...prev,
      [paperID]: {
        ...prev[paperID],
        [type]:
          prev[paperID]?.[type]?.live && !prev[paperID]?.[type]?.failed
            ? prev[paperID]![type]
            : {
                steps: [
                  {
                    phase: "preparing",
                    text: "小囊鼠已接收生成任务，正在启动研读流水线。",
                  },
                ],
                live: true,
                failed: false,
              },
      },
    }));
  }, []);

  // 报告生成阶段进度:failed 标记失败并停 live;其余阶段按 phase 续接/新建执行计划步。
  const applyReportProgress = useCallback(
    (paperID: string, type: ReportType, phase: string, detail?: string) => {
      if (phase === "failed") {
        const paper = papersRef.current.find((p) => p.id === paperID);
        toast(`「${paper ? paperTitle(paper) : "论文"}」报告生成失败`, "error");
      }
      setReportProgress((prev) => {
        const paperMap = prev[paperID] || {};
        const cur = paperMap[type] || { steps: [], live: true, failed: false };
        if (phase === "failed") {
          const steps = cur.steps.slice();
          if (detail) {
            const last = steps[steps.length - 1];
            if (last && last.phase === phase) {
              steps[steps.length - 1] = { ...last, text: last.text + detail };
            } else {
              steps.push({ phase, text: detail });
            }
          }
          return {
            ...prev,
            [paperID]: {
              ...paperMap,
              [type]: { ...cur, steps, live: false, failed: true },
            },
          };
        }
        const steps = cur.steps.slice();
        const last = steps[steps.length - 1];
        if (last && last.phase === phase) {
          steps[steps.length - 1] = {
            ...last,
            text: last.text + (detail || ""),
          };
        } else {
          steps.push({ phase, text: detail || "" });
        }
        return {
          ...prev,
          [paperID]: {
            ...paperMap,
            [type]: { steps, live: true, failed: false },
          },
        };
      });
    },
    [toast],
  );

  // 报告状态快照:从 /reports 拉到 ready + running,用于 SSE 漏帧或重新进入页面时恢复计划栏。
  const applyReportStatus = useCallback(
    (paperID: string, status: ReportsStatus) => {
      const ready = status.ready ?? [];
      const readySet = new Set<ReportType>(ready);
      setReportReady((prev) => ({
        ...prev,
        [paperID]: Object.fromEntries(ready.map((t) => [t, true])) as Partial<
          Record<ReportType, boolean>
        >,
      }));
      const running = status.running ?? [];
      if (running.length === 0) return;
      setReportProgress((prev) => {
        const paperMap = { ...(prev[paperID] || {}) };
        for (const run of running) {
          if (!run.type || (readySet.has(run.type) && !run.failed)) continue;
          paperMap[run.type] = {
            steps: run.steps ?? [],
            live: Boolean(run.live),
            failed: Boolean(run.failed),
          };
        }
        return { ...prev, [paperID]: paperMap };
      });
    },
    [],
  );

  // 进入某篇论文时回填已落库报告的就绪态,让报告面板免点击自动展示历史报告。
  useEffect(() => {
    if (!token || !activePaperID) return;
    let cancelled = false;
    api
      .reportStatus(activePaperID)
      .then((status) => {
        if (cancelled) return;
        applyReportStatus(activePaperID, status);
      })
      .catch(() => {});
    return () => {
      cancelled = true;
    };
  }, [token, activePaperID, applyReportStatus]);

  const connectWs = useCallback(
    (jwt: string) => {
      disconnectWs();
      const source = api.openStatusStream(
        jwt,
        (e) => applyStatusEvent(e),
        (e) => applyReportReady(e.paper_id, e.report_type),
        (e) =>
          applyReportProgress(e.paper_id, e.report_type, e.phase, e.detail),
      );
      if (!source) return;
      wsRef.current = source;
      // EventSource 自带断线重连,无需手动重试;登出时经 disconnectWs 关闭即止。
    },
    [applyStatusEvent, applyReportReady, applyReportProgress, disconnectWs],
  );

  // ---- 轮询兜底(Query refetchInterval 接管手写 setInterval) ----
  // 实时通道(SSE)瞬断时,轮询补齐「可提问/失败」状态。enabled 仅在有未就绪论文时开,
  // 论文全部就绪自动停;标签转后台自动暂停(refetchIntervalInBackground 默认 false)。
  // queryFn 读 papersRef 取最新 pending,避免闭包陈旧;副作用经 applyStatusEvent 落地。
  const hasPendingPapers = papers.some((p) => !isSettled(p.status));
  useQuery({
    queryKey: ["paper-status-poll"],
    enabled: Boolean(token) && hasPendingPapers,
    refetchInterval: 4200,
    queryFn: async () => {
      const pending = papersRef.current.filter((p) => !isSettled(p.status));
      if (pending.length === 0) return null;
      const updates = await Promise.all(
        pending.map((p) => api.paperStatus(p.id)),
      );
      for (const u of updates) {
        applyStatusEvent({
          type: "paper_status",
          paper_id: u.id,
          status: u.status,
          detail: u.fail_reason,
          parse_progress: u.parse_progress,
          parsed_pages: u.parsed_pages,
          total_pages: u.total_pages,
        });
      }
      return null;
    },
  });

  // 报告就绪轮询兜底:报告生成要一分多钟,期间 SSE 一旦卡顿/断开 report_ready 事件可能丢失,
  // 仅靠事件会把界面永远停在「生成中」。只要还有 live 报告就由 Query 定时 listReports 补位,
  // 命中即置就绪并收尾。enabled 随 live 报告存在与否自动开停,后台标签自动暂停。
  // queryFn 读 reportProgressRef 取当前仍在生成的论文,避免闭包陈旧。
  const hasLiveReport = Object.values(reportProgress).some((m) =>
    Object.values(m).some((r) => r?.live && !r.failed),
  );
  useQuery({
    queryKey: ["report-ready-poll"],
    enabled: Boolean(token) && hasLiveReport,
    refetchInterval: 5000,
    queryFn: async () => {
      const livePaperIds = Object.entries(reportProgressRef.current)
        .filter(([, m]) => Object.values(m).some((r) => r?.live && !r.failed))
        .map(([pid]) => pid);
      if (livePaperIds.length === 0) return null;
      await Promise.all(
        livePaperIds.map(async (pid) => {
          const status = await api.reportStatus(pid);
          // 该类报告本地还标 live 却已落库,说明事件丢了,补一次就绪;running 则补齐漏掉的计划片段。
          applyReportStatus(pid, status);
        }),
      );
      return null;
    },
  });

  // 后台标签挂起 SSE:每条 EventSource 长期占一条 HTTP/1.1 连接(同源仅 ~6 条),
  // 多开标签会顶满连接池阻塞导航与接口。切到后台即 close 释放配额,回前台再重连补状态。
  // 兜底轮询由 Query 在后台自动暂停(refetchIntervalInBackground 默认 false),回前台
  // 主动 invalidate 触发立即补一次,复刻原「回前台即补状态」。
  useEffect(() => {
    if (typeof document === "undefined") return;
    const onVisibility = () => {
      if (document.hidden) {
        disconnectWs();
      } else if (token) {
        connectWs(token);
        void queryClient.invalidateQueries({ queryKey: ["paper-status-poll"] });
        void queryClient.invalidateQueries({ queryKey: ["report-ready-poll"] });
      }
    };
    document.addEventListener("visibilitychange", onVisibility);
    return () => document.removeEventListener("visibilitychange", onVisibility);
  }, [token, connectWs, disconnectWs, queryClient]);

  // ---- 数据加载 ----
  // 设搜索词切换 papers query key(空→全列表,非空→搜索);invalidate 让同词刷新也强制重拉。
  const refreshPapers = useCallback(
    async (query = "") => {
      const q = query.trim();
      setPaperSearch(q);
      await queryClient.invalidateQueries({ queryKey: ["papers", q] });
    },
    [queryClient],
  );

  // 失效 sessions 缓存触发重拉(发消息后刷新标题/排序);Query 自动管在飞与卸载。
  const refreshSessions = useCallback(async () => {
    await queryClient.invalidateQueries({ queryKey: ["sessions"] });
  }, [queryClient]);

  const openSession = useCallback(
    async (id: string) => {
      setActiveSessionID(id);
      // 从 sessions 缓存读最新一份,定位该会话所属论文(避免闭包陈旧);
      // messages 由 ["messages", id] query 随 activeSessionID 切换自动拉取。
      const list = queryClient.getQueryData<Session[]>(["sessions"]) ?? [];
      const s = list.find((x) => x.id === id);
      if (s?.paper_id) setActivePaperID(s.paper_id);
    },
    [queryClient],
  );

  const bootstrapSession = useCallback(
    async (jwt: string) => {
      connectWs(jwt);
      // sessions 经 fetchQuery 拉取并写入缓存(useQuery 随即反映,无需 setSessions);
      // 返回值供初始化时选中首篇论文的最新会话。
      const [paperList, sessionList] = await Promise.all([
        queryClient.fetchQuery({
          queryKey: ["papers", ""],
          queryFn: async () => {
            const l = await api.listPapers();
            return Array.isArray(l) ? l : [];
          },
        }),
        queryClient.fetchQuery({
          queryKey: ["sessions"],
          queryFn: async () => chatSessions(await api.listSessions()),
        }),
      ]);
      api.preferences()
        .then((data) => setPreference({ ...DEFAULT_PREFERENCE, ...data }))
        .catch(() => {});
      if (!mountedRef.current) return;
      if (paperList.length > 0) {
        const first = paperList[0];
        setActivePaperID(first.id);
        const list = sessionsForPaper(sessionList, first.id);
        if (list.length > 0) await openSession(list[0].id);
      } else if (sessionList.length > 0) {
        await openSession(sessionList[0].id);
      }
    },
    [connectWs, openSession, queryClient],
  );

  useEffect(() => {
    if (hydratedRef.current) return;
    hydratedRef.current = true;
    const saved = loadAuth();
    if (!saved.token) return;
    setUser(saved.user);
    setTokenState(saved.token);
    api.setToken(saved.token);
    api.me().then((profile) => persist(profile, saved.token)).catch(() => {});
    api.preferences()
      .then((data) => setPreference({ ...DEFAULT_PREFERENCE, ...data }))
      .catch(() => {});
    bootstrapSession(saved.token).catch(() => logout(false));
  }, [bootstrapSession, logout, persist]);

  const login = useCallback(
    async (account: string, password: string) => {
      const data = await api.login(account, password);
      persist(
        {
          student_id: data.student_id,
          name: data.name,
          email: data.email,
          avatar_url: data.avatar_url,
          class_id: data.class_id,
        },
        data.token,
      );
      await bootstrapSession(data.token);
      toast("登录成功");
    },
    [bootstrapSession, persist, toast],
  );

  const registerAndLogin = useCallback(
    async (payload: RegisterPayload) => {
      await api.register(payload);
      await login(payload.student_id, payload.password);
    },
    [login],
  );

  const sendCode = useCallback(
    async (email: string) => {
      await api.sendCode(email);
      toast("验证码已发送");
    },
    [toast],
  );

  const sendPasswordResetCode = useCallback(
    async (payload: PasswordResetCodePayload) => {
      await api.sendPasswordResetCode(payload);
      toast("如果账号与邮箱匹配，验证码已发送");
    },
    [toast],
  );

  const resetPassword = useCallback(
    async (payload: ResetPasswordPayload) => {
      await api.resetPassword(payload);
      toast("密码已重置，请重新登录");
    },
    [toast],
  );

  const refreshUser = useCallback(async () => {
    const profile = await api.me();
    persist(profile, token);
  }, [persist, token]);

  const refreshPreferences = useCallback(async () => {
    const data = await api.preferences();
    setPreference({ ...DEFAULT_PREFERENCE, ...data });
  }, []);

  const updateProfile = useCallback(
    async (payload: UpdateProfilePayload) => {
      const profile = await api.updateProfile(payload);
      persist(profile, token);
      toast("个人资料已更新");
    },
    [persist, toast, token],
  );

  const updateEmail = useCallback(
    async (payload: UpdateEmailPayload) => {
      await api.updateEmail(payload);
      toast("邮箱已更新，请重新登录");
    },
    [toast],
  );

  const updatePreferences = useCallback(
    async (payload: UpdateUserPreferencePayload) => {
      const data = await api.updatePreferences(payload);
      setPreference({ ...DEFAULT_PREFERENCE, ...data });
      toast("AI 回答偏好已更新");
    },
    [toast],
  );

  const updateAvatar = useCallback(
    async (file: File) => {
      const data = await api.uploadAvatar(file);
      const nextUser = user ? { ...user, avatar_url: data.avatar_url } : null;
      persist(nextUser, token);
      toast("头像已更新");
    },
    [persist, toast, token, user],
  );

  const clearAvatar = useCallback(async () => {
    await api.clearAvatar();
    const nextUser = user ? { ...user, avatar_url: "" } : null;
    persist(nextUser, token);
    toast("已恢复默认头像");
  }, [persist, toast, token, user]);

  const uploadPaper = useCallback(
    async (file: File) => {
      const paper = await api.uploadPaper(file);
      if (paper?.id) {
        // 回到全列表并乐观置顶新论文(搜索态下上传也立即可见)。
        setPaperSearch("");
        queryClient.setQueryData<Paper[]>(["papers", ""], (old = []) => [
          paper,
          ...old.filter((p) => p.id !== paper.id),
        ]);
        // 新论文还没有会话,切过去并进入欢迎态(messages 随 activeSessionID="" 自动清空)。
        setActivePaperID(paper.id);
        setActiveSessionID("");
      }
    },
    [queryClient],
  );

  const reparsePaper = useCallback(
    async (id: string) => {
      const paper = await api.reparsePaper(id);
      if (paper?.id) {
        const patch: Partial<Paper> = {
          ...paper,
          status: paper.status ?? "uploaded",
          fail_reason: "",
          status_detail: paper.fail_reason || "重新解析任务已提交",
          parse_progress: 0,
          parsed_pages: 0,
          total_pages: 0,
        };
        papersRef.current = papersRef.current.map((p) =>
          p.id === id ? { ...p, ...patch } : p,
        );
        setPapers((list) =>
          list.map((p) => (p.id === id ? { ...p, ...patch } : p)),
        );
      }
      await queryClient.invalidateQueries({ queryKey: ["paper-status-poll"] });
      toast("已提交重新解析任务");
    },
    [queryClient, setPapers, toast],
  );

  const removePaper = useCallback(
    async (id: string) => {
      const paperIndex = papers.findIndex((p) => p.id === id);
      const remainingPapers = papers.filter((p) => p.id !== id);
      const remainingSessions = sessions.filter((s) => s.paper_id !== id);
      const activeSessionDeleted = sessions.some(
        (s) => s.id === activeSessionID && s.paper_id === id,
      );

      await api.deletePaper(id);

      setPapers((list) => list.filter((p) => p.id !== id));
      setSessions(remainingSessions);
      setReportReady((prev) => {
        const next = { ...prev };
        delete next[id];
        return next;
      });

      if (activePaperID === id) {
        const nextPaper =
          remainingPapers[paperIndex] ||
          remainingPapers[paperIndex - 1] ||
          remainingPapers[0] ||
          null;
        if (nextPaper) {
          setActivePaperID(nextPaper.id);
          const paperSessions = sessionsForPaper(
            remainingSessions,
            nextPaper.id,
          );
          if (paperSessions.length > 0) {
            await openSession(paperSessions[0].id);
          } else {
            setActiveSessionID("");
          }
        } else {
          setActivePaperID("");
          setActiveSessionID("");
        }
      } else if (activeSessionDeleted) {
        setActiveSessionID("");
      }

      toast("论文已删除");
    },
    [
      activePaperID,
      activeSessionID,
      openSession,
      papers,
      sessions,
      toast,
      setSessions,
      setPapers,
    ],
  );

  // 选论文:同一篇保持当前会话不动;切到不同论文则跳到该论文最新会话,
  // 没有会话则清空进入欢迎态(会话因此永远归属当前论文)。
  const selectPaper = useCallback(
    (id: string) => {
      if (id === activePaperID) return;
      setActivePaperID(id);
      const list = sessionsForPaper(sessions, id);
      if (list.length > 0) {
        openSession(list[0].id);
      } else {
        setActiveSessionID("");
      }
    },
    [activePaperID, sessions, openSession],
  );

  const createSession = useCallback(
    async (title: string, paperID?: string) => {
      const session = await api.createSession(title, paperID);
      setSessions((list) => [
        session,
        ...list.filter((s) => s.id !== session.id),
      ]);
      setActiveSessionID(session.id);
      if (session.paper_id) setActivePaperID(session.paper_id);
      // 新会话必空:预置空消息缓存,免一次无谓 listMessages,也避开与后续流式写入的竞争。
      queryClient.setQueryData<Message[]>(["messages", session.id], []);
      return session;
    },
    [setSessions, queryClient],
  );

  const removeSession = useCallback(
    async (id: string) => {
      await api.deleteSession(id);
      setSessions((list) => list.filter((s) => s.id !== id));
      setActiveSessionID((cur) => (cur === id ? "" : cur));
      toast("会话已删除");
    },
    [toast, setSessions],
  );

  const sendMessage = useCallback(
    async (query: string) => {
      setSending(true);
      try {
        let sessionID = activeSessionID;
        if (!sessionID) {
          const paper = papers.find((p) => p.id === activePaperID) || null;
          const paperID = paper?.id || activePaperID || undefined;
          const title = paper
            ? `${paperTitle(paper)} 问答`
            : paperID
              ? "论文问答"
              : query.slice(0, 24) || "新会话";
          const session = await createSession(title, paperID);
          sessionID = session.id;
        }
        // 取消该会话在飞的 listMessages,避免乐观写入被随后到达的 fetch 结果覆盖(官方乐观更新模式)。
        await queryClient.cancelQueries({ queryKey: ["messages", sessionID] });
        // 流式写入定位到该会话的 messages 缓存(sessionID 可能是刚新建的,与 activeSessionID 一致)。
        const setMsg = (fn: (list: Message[]) => Message[]) =>
          queryClient.setQueryData<Message[]>(
            ["messages", sessionID],
            (old = []) => fn(old),
          );
        const userMsg: Message = {
          id: `local-${Date.now()}`,
          session_id: sessionID,
          role: "user",
          content: query,
          created_at: new Date().toISOString(),
        };
        setMsg((list) => [...list, userMsg]);

        // SSE 流式占位:首个文本增量到达时上屏一条 streaming 助教消息,
        // done 后整体替换为最终消息;工具阶段由输入框上方的状态气泡呈现,不进消息流。
        const placeholderID = `stream-${Date.now()}`;
        let shown = false;
        const patch = (fn: (m: Message) => Message) => {
          if (!shown) {
            shown = true;
            setMsg((list) => [
              ...list,
              fn({
                id: placeholderID,
                session_id: sessionID,
                role: "assistant",
                content: "",
                created_at: new Date().toISOString(),
                streaming: true,
              }),
            ]);
            return;
          }
          setMsg((list) =>
            list.map((m) => (m.id === placeholderID ? fn(m) : m)),
          );
        };
        // 增量按帧合并:逐 token 来的 delta 先攒进 pending,每帧最多 flush 一次,
        // 把重渲染频率从「每 token」降到「每帧」,长答案尾部不再掉帧。
        let pending = "";
        let rafID: number | null = null;
        const flush = () => {
          rafID = null;
          if (!pending) return;
          const chunk = pending;
          pending = "";
          setToolNote("");
          patch((m) => ({ ...m, content: m.content + chunk }));
        };
        // 执行过程(规划/检索/思考)累积:同 phase 续接、换 phase 新建一段;合帧后经 patch 写入
        // 占位消息的 plan 字段。plan 事件通常先于首个 delta 到达,patch 会让占位提前上屏,
        // 用户在答案写出前即看到流程在动(消除多轮检索的等待焦虑)。
        const planSteps: PlanStep[] = [];
        // 思路图谱:generate_paper_flow 经 SSE 推来后即上屏到占位消息,done 时一并挂最终消息。
        let capturedFlow: PaperFlow | undefined;
        let planRafID: number | null = null;
        const flushPlan = () => {
          planRafID = null;
          patch((m) => ({ ...m, plan: planSteps.map((s) => ({ ...s })) }));
        };
        const cancelFlush = () => {
          if (rafID !== null) {
            cancelAnimationFrame(rafID);
            rafID = null;
          }
          if (planRafID !== null) {
            cancelAnimationFrame(planRafID);
            planRafID = null;
          }
        };
        try {
          const data = await api.sendMessage(sessionID, query, undefined, {
            onDelta: (text) => {
              pending += text;
              if (rafID === null) rafID = requestAnimationFrame(flush);
            },
            // 工具状态:输入框上方的状态气泡 + 执行过程步。
            // 模型(doubao)未必稳定输出 planner 标签,工具调用是框架确定性事件——
            // 据此合成规划/检索/思考三类步,保证执行过程稳定显示。
            onTool: (tool, done) => {
              if (!done) {
                setToolNote(toolStatusText(tool, done));
                // 首次工具调用前合成规划步(模型未输出时补全)
                if (
                  !planSteps.some(
                    (s) => s.phase === "planning" || s.phase === "replanning",
                  )
                ) {
                  planSteps.push({
                    phase: "planning",
                    text: "分析问题，制定检索策略",
                  });
                }
                const text = TOOL_STEP_TEXT[tool] || tool;
                const last = planSteps[planSteps.length - 1];
                if (!(last && last.phase === "action" && last.text === text)) {
                  planSteps.push({ phase: "action", text });
                }
                if (planRafID === null)
                  planRafID = requestAnimationFrame(flushPlan);
              } else {
                setToolNote(toolStatusText(tool, done));
                // 工具结果返回后合成思考步
                const last = planSteps[planSteps.length - 1];
                if (!last || last.phase !== "reasoning") {
                  planSteps.push({
                    phase: "reasoning",
                    text: "综合检索结果，整理回答",
                  });
                  if (planRafID === null)
                    planRafID = requestAnimationFrame(flushPlan);
                }
              }
            },
            // 规划/检索/思考阶段文本:累积成 plan 步,实时流进「执行过程」活动条。
            onPlan: (phase, content) => {
              const last = planSteps[planSteps.length - 1];
              if (last && last.phase === phase) last.text += content;
              else planSteps.push({ phase, text: content });
              if (planRafID === null)
                planRafID = requestAnimationFrame(flushPlan);
            },
            // 思路图骨架推达:立即挂到占位消息,气泡内先画出结构(节点待点亮)。
            // 兜底防重:同一论文本轮已收到骨架则忽略后续重复推送,避免把已点亮的图打回占位再重画
            // (清空重画闪烁)。后端已对重复调用幂等,这里再防一层任何来源的重复骨架。
            onPaperFlow: (payload) => {
              if (capturedFlow && capturedFlow.paper_id === payload.paper_id) return;
              capturedFlow = payload;
              patch((m) => ({ ...m, flow: payload }));
            },
            // 逐节点 detail 推达:更新对应节点点亮;带配图则追加到 figures。
            onPaperFlowNode: (payload) => {
              if (!capturedFlow) return;
              const figures = payload.figure
                ? [...(capturedFlow.figures ?? []), payload.figure]
                : capturedFlow.figures;
              capturedFlow = {
                ...capturedFlow,
                nodes: capturedFlow.nodes.map((n) =>
                  n.id === payload.node_id ? { ...n, detail: payload.detail } : n,
                ),
                figures,
              };
              const flow = capturedFlow;
              patch((m) => ({ ...m, flow }));
            },
          });
          // 收尾:取消待处理的帧回调,最终消息直接整体替换占位。
          cancelFlush();
          if (data?.message) {
            const assistant: Message = { ...data.message };
            // 助教消息的真实 ID 落 Session 后才有,即时应答 ID 为空,
            // 这里补个本地唯一 ID 避免多轮渲染 key 冲突;重开会话时由 listMessages 还原真实 ID。
            if (!assistant.id) assistant.id = `local-a-${Date.now()}`;
            if (data.meta) assistant.meta = data.meta;
            // 把本轮累积的执行过程挂到最终消息,供答后折叠回看;瞬态不入库,刷新即失。
            if (planSteps.length) assistant.plan = planSteps;
            // 思路图谱同样挂到最终消息,瞬态不入库,刷新即失。
            if (capturedFlow) assistant.flow = capturedFlow;
            setMsg((list) => [
              ...list.filter((m) => m.id !== placeholderID),
              assistant,
            ]);
          } else {
            setMsg((list) => list.filter((m) => m.id !== placeholderID));
          }
        } catch (e) {
          cancelFlush();
          setMsg((list) => list.filter((m) => m.id !== placeholderID));
          throw e;
        }
        await refreshSessions();
      } finally {
        setSending(false);
        setToolNote("");
      }
    },
    [
      activePaperID,
      activeSessionID,
      createSession,
      papers,
      refreshSessions,
      queryClient,
    ],
  );

  const activePaper = useMemo(
    () => papers.find((p) => p.id === activePaperID) || null,
    [papers, activePaperID],
  );
  const activeSession = useMemo(
    () => sessions.find((s) => s.id === activeSessionID) || null,
    [sessions, activeSessionID],
  );

  const value: AppContextValue = useMemo(
    () => ({
      user,
      preference,
      authed: Boolean(token),
      papers,
      sessions,
      messages,
      activePaperID,
      activeSessionID,
      sending,
      toolNote,
      toasts,
      activePaper,
      activeSession,
      reportReady,
      reportProgress,
      toast,
      dismissToast,
      login,
      registerAndLogin,
      sendCode,
      sendPasswordResetCode,
      resetPassword,
      logout,
      refreshUser,
      updateProfile,
      updateEmail,
      refreshPreferences,
      updatePreferences,
      updateAvatar,
      clearAvatar,
      refreshPapers,
      uploadPaper,
      reparsePaper,
      removePaper,
      selectPaper,
      refreshSessions,
      openSession,
      createSession,
      removeSession,
      sendMessage,
      beginReport,
    }),
    [
      user,
      preference,
      token,
      papers,
      sessions,
      messages,
      activePaperID,
      activeSessionID,
      sending,
      toolNote,
      toasts,
      activePaper,
      activeSession,
      reportReady,
      reportProgress,
      toast,
      dismissToast,
      login,
      registerAndLogin,
      sendCode,
      sendPasswordResetCode,
      resetPassword,
      logout,
      refreshUser,
      updateProfile,
      updateEmail,
      refreshPreferences,
      updatePreferences,
      updateAvatar,
      clearAvatar,
      refreshPapers,
      uploadPaper,
      reparsePaper,
      removePaper,
      selectPaper,
      refreshSessions,
      openSession,
      createSession,
      removeSession,
      sendMessage,
      beginReport,
    ],
  );

  return <AppContext.Provider value={value}>{children}</AppContext.Provider>;
}

// 包级单例 QueryClient 不可取(SSR/多实例会串数据),按 Provider 实例建一次。
// 服务端数据由 SSE 推送驱动刷新,故关掉窗口聚焦自动重拉,避免和推送重复;
// poll 类 query 自带 refetchInterval,后台标签自动暂停。
export function AppProvider({ children }: { children: ReactNode }) {
  const [queryClient] = useState(
    () =>
      new QueryClient({
        defaultOptions: {
          queries: { refetchOnWindowFocus: false, retry: 1 },
        },
      }),
  );
  return (
    <QueryClientProvider client={queryClient}>
      <AppProviderInner>{children}</AppProviderInner>
    </QueryClientProvider>
  );
}

export function useApp(): AppContextValue {
  const ctx = useContext(AppContext);
  if (!ctx) throw new Error("useApp 必须在 AppProvider 内使用");
  return ctx;
}
