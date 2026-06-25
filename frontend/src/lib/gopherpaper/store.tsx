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

import * as api from "./api";
import type {
  AuthUser,
  Message,
  Paper,
  PlanStep,
  RegisterPayload,
  ReportRun,
  ReportType,
  Session,
} from "./types";
import { chatSessions, isSettled, paperTitle, sessionsForPaper } from "./utils";

const AUTH_KEY = "gopherpaper.auth";

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
  login: (studentID: string, password: string) => Promise<void>;
  registerAndLogin: (payload: RegisterPayload) => Promise<void>;
  sendCode: (email: string) => Promise<void>;
  logout: () => void;
  refreshPapers: (query?: string) => Promise<void>;
  uploadPaper: (file: File) => Promise<void>;
  selectPaper: (id: string) => void;
  refreshSessions: () => Promise<void>;
  openSession: (id: string) => Promise<void>;
  createSession: (title: string, paperID?: string) => Promise<Session>;
  removeSession: (id: string) => Promise<void>;
  sendMessage: (query: string) => Promise<void>;
  // 报告面板点击生成时调用,重置该报告的进度为「进行中」,后续阶段由 SSE 累积。
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

export function AppProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<AuthUser | null>(null);
  const [token, setTokenState] = useState<string>("");
  const [papers, setPapers] = useState<Paper[]>([]);
  const [sessions, setSessions] = useState<Session[]>([]);
  const [messages, setMessages] = useState<Message[]>([]);
  const [activePaperID, setActivePaperID] = useState("");
  const [activeSessionID, setActiveSessionID] = useState("");
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
  const pollRef = useRef<number | null>(null);
  const reportPollRef = useRef<number | null>(null);
  const toastSeq = useRef(0);
  const hydratedRef = useRef(false);
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

  api.setToken(token);

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
  const persist = useCallback((nextUser: AuthUser | null, nextToken: string) => {
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
  }, []);

  const stopPolling = useCallback(() => {
    if (pollRef.current) {
      window.clearInterval(pollRef.current);
      pollRef.current = null;
    }
  }, []);

  const stopReportPoll = useCallback(() => {
    if (reportPollRef.current) {
      window.clearInterval(reportPollRef.current);
      reportPollRef.current = null;
    }
  }, []);

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

  // notifyServer 为真时先通知后端清登录态与常驻缓存(趁 token 未清,fire-and-forget 不阻塞);
  // 401 被动登出时 token 已失效,传 false 跳过这次注定失败的请求。
  const logout = useCallback((notifyServer = true) => {
    if (notifyServer) void api.logout();
    stopPolling();
    stopReportPoll();
    disconnectWs();
    persist(null, "");
    setPapers([]);
    setSessions([]);
    setMessages([]);
    setActivePaperID("");
    setActiveSessionID("");
    setReportReady({});
    setReportProgress({});
  }, [disconnectWs, persist, stopPolling, stopReportPoll]);

  // 401 统一登出,被动登出不再回调后端(token 已失效)。
  useEffect(() => {
    api.setUnauthorizedHandler(() => logout(false));
  }, [logout]);

  // ---- WS 推送进度 ----
  const applyStatusEvent = useCallback(
    (paperID: string, status: Paper["status"], detail?: string) => {
      // 判重与 toast 都在 updater 之外做:SSE 与轮询兜底会就同一状态各调一次,
      // updater 必须纯,副作用留在这里只触发一次。
      const existing = papersRef.current.find((p) => p.id === paperID);
      if (!existing) {
        // 列表里还没有(刚上传未刷新),异步补齐。
        api
          .listPapers()
          .then((fresh) => setPapers(Array.isArray(fresh) ? fresh : []))
          .catch(() => {});
        return;
      }
      if (existing.status === status) return;
      if (status === "ready") {
        toast(`「${paperTitle(existing)}」已就绪,可提问`);
      } else if (status === "failed") {
        toast(`「${paperTitle(existing)}」解析失败:${detail || "未知原因"}`, "error");
      }
      // 先同步推进镜像,紧随其后的同状态事件(SSE/轮询)即被上面的判重拦掉。
      papersRef.current = papersRef.current.map((p) =>
        p.id === paperID
          ? { ...p, status, fail_reason: detail || p.fail_reason }
          : p,
      );
      setPapers((list) =>
        list.map((p) =>
          p.id === paperID
            ? { ...p, status, fail_reason: detail || p.fail_reason }
            : p,
        ),
      );
    },
    [toast],
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
          [paperID]: { ...prev[paperID], [reportType]: { ...run, live: false } },
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
    [],
  );

  // beginReport 在用户点生成时重置该报告的进度为「进行中、空步」,随后由 SSE 阶段事件累积。
  const beginReport = useCallback((paperID: string, type: ReportType) => {
    setReportProgress((prev) => ({
      ...prev,
      [paperID]: {
        ...prev[paperID],
        [type]: { steps: [], live: true, failed: false },
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
          return {
            ...prev,
            [paperID]: { ...paperMap, [type]: { ...cur, live: false, failed: true } },
          };
        }
        const steps = cur.steps.slice();
        const last = steps[steps.length - 1];
        if (last && last.phase === phase) {
          steps[steps.length - 1] = { ...last, text: last.text + (detail || "") };
        } else {
          steps.push({ phase, text: detail || "" });
        }
        return {
          ...prev,
          [paperID]: { ...paperMap, [type]: { steps, live: true, failed: false } },
        };
      });
    },
    [toast],
  );

  // 进入某篇论文时回填已落库报告的就绪态,让报告面板免点击自动展示历史报告。
  useEffect(() => {
    if (!token || !activePaperID) return;
    let cancelled = false;
    api
      .listReports(activePaperID)
      .then((types) => {
        if (cancelled || types.length === 0) return;
        setReportReady((prev) => {
          const next = { ...(prev[activePaperID] || {}) };
          for (const t of types) next[t] = true;
          return { ...prev, [activePaperID]: next };
        });
      })
      .catch(() => {});
    return () => {
      cancelled = true;
    };
  }, [token, activePaperID]);

  const connectWs = useCallback(
    (jwt: string) => {
      disconnectWs();
      const source = api.openStatusStream(
        jwt,
        (e) => applyStatusEvent(e.paper_id, e.status, e.detail),
        (e) => applyReportReady(e.paper_id, e.report_type),
        (e) => applyReportProgress(e.paper_id, e.report_type, e.phase, e.detail),
      );
      if (!source) return;
      wsRef.current = source;
      // EventSource 自带断线重连,无需手动重试;登出时经 disconnectWs 关闭即止。
    },
    [applyStatusEvent, applyReportReady, applyReportProgress, disconnectWs],
  );

  // ---- 轮询兜底 ----
  const startPolling = useCallback(() => {
    stopPolling();
    pollRef.current = window.setInterval(async () => {
      let pending: Paper[] = [];
      setPapers((list) => {
        pending = list.filter((p) => !isSettled(p.status));
        return list;
      });
      if (pending.length === 0) {
        stopPolling();
        return;
      }
      try {
        const updates = await Promise.all(
          pending.map((p) => api.paperStatus(p.id)),
        );
        // 经 applyStatusEvent 落地,实时通道(SSE)瞬断时轮询也能补「可提问」通知。
        for (const u of updates) {
          applyStatusEvent(u.id, u.status, u.fail_reason);
        }
      } catch {
        // 临时网络/代理错误不应永久停止兜底轮询,下一轮继续查。
      }
    }, 4200);
  }, [stopPolling, applyStatusEvent]);

  // 列表变化时按需开/停轮询。
  useEffect(() => {
    if (!token) return;
    if (papers.some((p) => !isSettled(p.status))) {
      if (!pollRef.current) startPolling();
    } else {
      stopPolling();
    }
  }, [papers, token, startPolling, stopPolling]);

  // 报告就绪轮询兜底:报告生成要一分多钟,期间 SSE 一旦卡顿/断开,report_ready 事件可能丢失,
  // 仅靠事件会把界面永远停在「生成中」。只要还有 live 报告就定时 listReports 补位,命中即置就绪并收尾。
  useEffect(() => {
    if (!token) return;
    const hasLive = Object.values(reportProgress).some((m) =>
      Object.values(m).some((r) => r?.live && !r.failed),
    );
    if (!hasLive) {
      stopReportPoll();
      return;
    }
    if (reportPollRef.current) return; // 已在轮询,避免重复起定时器
    reportPollRef.current = window.setInterval(async () => {
      // 闭包里读 ref 镜像取当前仍在生成的论文,避免读到起定时器那刻的陈旧集合。
      const livePaperIds = Object.entries(reportProgressRef.current)
        .filter(([, m]) => Object.values(m).some((r) => r?.live && !r.failed))
        .map(([pid]) => pid);
      if (livePaperIds.length === 0) {
        stopReportPoll();
        return;
      }
      try {
        await Promise.all(
          livePaperIds.map(async (pid) => {
            const types = await api.listReports(pid);
            for (const t of types) {
              // 该类报告本地还标 live 却已落库,说明事件丢了,补一次就绪(applyReportReady 内会收尾 live)。
              if (reportProgressRef.current[pid]?.[t]?.live) applyReportReady(pid, t);
            }
          }),
        );
      } catch {
        // 临时网络/代理错误不停轮询,下一轮继续补。
      }
    }, 5000);
  }, [token, reportProgress, stopReportPoll, applyReportReady]);

  // 后台标签挂起 SSE 与轮询:每条 EventSource 长期占一条 HTTP/1.1 连接(同源仅 ~6 条),
  // 多开标签会顶满连接池阻塞导航与接口。切到后台即 close 释放配额,回前台再重连补状态。
  useEffect(() => {
    if (typeof document === "undefined") return;
    const onVisibility = () => {
      if (document.hidden) {
        disconnectWs();
        stopPolling();
      } else if (token) {
        connectWs(token);
        // 后台期间可能漏掉解析进度,有未就绪论文就重启兜底轮询补回。
        if (papersRef.current.some((p) => !isSettled(p.status))) startPolling();
      }
    };
    document.addEventListener("visibilitychange", onVisibility);
    return () => document.removeEventListener("visibilitychange", onVisibility);
  }, [token, connectWs, disconnectWs, startPolling, stopPolling]);

  // ---- 数据加载 ----
  const refreshPapers = useCallback(
    async (query = "") => {
      const list = query.trim()
        ? await api.searchPapers(query.trim())
        : await api.listPapers();
      setPapers(Array.isArray(list) ? list : []);
    },
    [],
  );

  const refreshSessions = useCallback(async () => {
    const list = await api.listSessions();
    setSessions(chatSessions(Array.isArray(list) ? list : []));
  }, []);

  const openSession = useCallback(
    async (id: string) => {
      setActiveSessionID(id);
      setSessions((list) => {
        const s = list.find((x) => x.id === id);
        if (s?.paper_id) setActivePaperID(s.paper_id);
        return list;
      });
      const msgs = await api.listMessages(id);
      setMessages(Array.isArray(msgs) ? msgs : []);
    },
    [],
  );

  const bootstrapSession = useCallback(
    async (jwt: string) => {
      connectWs(jwt);
      const [pl, sl] = await Promise.all([
        api.listPapers(),
        api.listSessions(),
      ]);
      const paperList = Array.isArray(pl) ? pl : [];
      const sessionList = chatSessions(Array.isArray(sl) ? sl : []);
      setPapers(paperList);
      setSessions(sessionList);
      if (paperList.length > 0) {
        const first = paperList[0];
        setActivePaperID(first.id);
        const list = sessionsForPaper(sessionList, first.id);
        if (list.length > 0) await openSession(list[0].id);
      } else if (sessionList.length > 0) {
        await openSession(sessionList[0].id);
      }
    },
    [connectWs, openSession],
  );

  useEffect(() => {
    if (hydratedRef.current) return;
    hydratedRef.current = true;
    const saved = loadAuth();
    if (!saved.token) return;
    setUser(saved.user);
    setTokenState(saved.token);
    api.setToken(saved.token);
    bootstrapSession(saved.token).catch(() => logout(false));
  }, [bootstrapSession, logout]);

  const login = useCallback(
    async (studentID: string, password: string) => {
      const data = await api.login(studentID, password);
      persist(
        { student_id: data.student_id, name: data.name, email: data.email },
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

  const uploadPaper = useCallback(
    async (file: File) => {
      const paper = await api.uploadPaper(file);
      if (paper?.id) {
        setPapers((list) => [
          paper,
          ...list.filter((p) => p.id !== paper.id),
        ]);
        // 新论文还没有会话,切过去并进入欢迎态。
        setActivePaperID(paper.id);
        setActiveSessionID("");
        setMessages([]);
      }
    },
    [],
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
        setMessages([]);
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
      setMessages([]);
      return session;
    },
    [],
  );

  const removeSession = useCallback(
    async (id: string) => {
      await api.deleteSession(id);
      setSessions((list) => list.filter((s) => s.id !== id));
      setActiveSessionID((cur) => {
        if (cur === id) {
          setMessages([]);
          return "";
        }
        return cur;
      });
      toast("会话已删除");
    },
    [toast],
  );

  const sendMessage = useCallback(
    async (query: string) => {
      setSending(true);
      try {
        let sessionID = activeSessionID;
        if (!sessionID) {
          const paper = papers.find((p) => p.id === activePaperID) || null;
          const title = paper
            ? `${paperTitle(paper)} 问答`
            : query.slice(0, 24) || "新会话";
          const session = await createSession(title, paper?.id);
          sessionID = session.id;
        }
        const userMsg: Message = {
          id: `local-${Date.now()}`,
          session_id: sessionID,
          role: "user",
          content: query,
          created_at: new Date().toISOString(),
        };
        setMessages((list) => [...list, userMsg]);

        // SSE 流式占位:首个文本增量到达时上屏一条 streaming 助教消息,
        // done 后整体替换为最终消息;工具阶段由输入框上方的状态气泡呈现,不进消息流。
        const placeholderID = `stream-${Date.now()}`;
        let shown = false;
        const patch = (fn: (m: Message) => Message) => {
          if (!shown) {
            shown = true;
            setMessages((list) => [
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
          setMessages((list) =>
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
                setToolNote(`正在调用 ${tool} …`);
                // 首次工具调用前合成规划步(模型未输出时补全)
                if (!planSteps.some((s) => s.phase === "planning" || s.phase === "replanning")) {
                  planSteps.push({ phase: "planning", text: "分析问题，制定检索策略" });
                }
                const text = TOOL_STEP_TEXT[tool] || tool;
                const last = planSteps[planSteps.length - 1];
                if (!(last && last.phase === "action" && last.text === text)) {
                  planSteps.push({ phase: "action", text });
                }
                if (planRafID === null) planRafID = requestAnimationFrame(flushPlan);
              } else {
                setToolNote(`${tool} 已返回,正在继续…`);
                // 工具结果返回后合成思考步
                const last = planSteps[planSteps.length - 1];
                if (!last || last.phase !== "reasoning") {
                  planSteps.push({ phase: "reasoning", text: "综合检索结果，整理回答" });
                  if (planRafID === null) planRafID = requestAnimationFrame(flushPlan);
                }
              }
            },
            // 规划/检索/思考阶段文本:累积成 plan 步,实时流进「执行过程」活动条。
            onPlan: (phase, content) => {
              const last = planSteps[planSteps.length - 1];
              if (last && last.phase === phase) last.text += content;
              else planSteps.push({ phase, text: content });
              if (planRafID === null) planRafID = requestAnimationFrame(flushPlan);
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
            setMessages((list) => [
              ...list.filter((m) => m.id !== placeholderID),
              assistant,
            ]);
          } else {
            setMessages((list) => list.filter((m) => m.id !== placeholderID));
          }
        } catch (e) {
          cancelFlush();
          setMessages((list) => list.filter((m) => m.id !== placeholderID));
          throw e;
        }
        await refreshSessions();
      } finally {
        setSending(false);
        setToolNote("");
      }
    },
    [activePaperID, activeSessionID, createSession, papers, refreshSessions],
  );

  // 启动时若已有 token 自动恢复。
  useEffect(() => {
    if (!token) return;
    connectWs(token);
    Promise.all([api.listPapers(), api.listSessions()])
      .then(([pl, sl]) => {
        const paperList = Array.isArray(pl) ? pl : [];
        const sessionList = chatSessions(Array.isArray(sl) ? sl : []);
        setPapers(paperList);
        setSessions(sessionList);
        if (paperList.length > 0) {
          const first = paperList[0];
          setActivePaperID((cur) => cur || first.id);
          const list = sessionsForPaper(sessionList, first.id);
          if (list.length > 0) openSession(list[0].id);
        } else if (sessionList.length > 0) {
          openSession(sessionList[0].id);
        }
      })
      .catch((err) => toast(err?.message || "加载失败", "error"));
    return () => {
      disconnectWs();
      stopPolling();
      stopReportPoll();
    };
    // 仅在挂载时跑一次恢复流程。
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const activePaper = useMemo(
    () => papers.find((p) => p.id === activePaperID) || null,
    [papers, activePaperID],
  );
  const activeSession = useMemo(
    () => sessions.find((s) => s.id === activeSessionID) || null,
    [sessions, activeSessionID],
  );

  const value: AppContextValue = {
    user,
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
    logout,
    refreshPapers,
    uploadPaper,
    selectPaper,
    refreshSessions,
    openSession,
    createSession,
    removeSession,
    sendMessage,
    beginReport,
  };

  return <AppContext.Provider value={value}>{children}</AppContext.Provider>;
}

export function useApp(): AppContextValue {
  const ctx = useContext(AppContext);
  if (!ctx) throw new Error("useApp 必须在 AppProvider 内使用");
  return ctx;
}
