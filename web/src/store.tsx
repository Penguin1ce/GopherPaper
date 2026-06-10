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
  RegisterPayload,
  ReportType,
  Session,
} from "./types";
import { isSettled, paperTitle, sessionsForPaper } from "./utils";

const AUTH_KEY = "gopherpaper.auth";

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
  toasts: ToastItem[];
  activePaper: Paper | null;
  activeSession: Session | null;
  // 各论文已后台预生成就绪的研读报告类型,供报告面板免轮询直接拉缓存
  reportReady: Record<string, Partial<Record<ReportType, boolean>>>;
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
}

const AppContext = createContext<AppContextValue | null>(null);

function loadAuth(): PersistedAuth {
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
  const initial = useRef(loadAuth());
  const [user, setUser] = useState<AuthUser | null>(initial.current.user);
  const [token, setTokenState] = useState<string>(initial.current.token);
  const [papers, setPapers] = useState<Paper[]>([]);
  const [sessions, setSessions] = useState<Session[]>([]);
  const [messages, setMessages] = useState<Message[]>([]);
  const [activePaperID, setActivePaperID] = useState("");
  const [activeSessionID, setActiveSessionID] = useState("");
  const [sending, setSending] = useState(false);
  const [toasts, setToasts] = useState<ToastItem[]>([]);
  const [reportReady, setReportReady] = useState<
    Record<string, Partial<Record<ReportType, boolean>>>
  >({});

  const wsRef = useRef<WebSocket | null>(null);
  const wsRetryRef = useRef<number | null>(null);
  const pollRef = useRef<number | null>(null);
  const toastSeq = useRef(0);

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

  const disconnectWs = useCallback(() => {
    if (wsRetryRef.current) {
      window.clearTimeout(wsRetryRef.current);
      wsRetryRef.current = null;
    }
    if (wsRef.current) {
      const socket = wsRef.current;
      wsRef.current = null;
      try {
        socket.close();
      } catch {
        // 忽略关闭异常
      }
    }
  }, []);

  const logout = useCallback(() => {
    stopPolling();
    disconnectWs();
    persist(null, "");
    setPapers([]);
    setSessions([]);
    setMessages([]);
    setActivePaperID("");
    setActiveSessionID("");
    setReportReady({});
  }, [disconnectWs, persist, stopPolling]);

  // 401 统一登出。
  useEffect(() => {
    api.setUnauthorizedHandler(() => logout());
  }, [logout]);

  // ---- WS 推送进度 ----
  const applyStatusEvent = useCallback(
    (paperID: string, status: Paper["status"], detail?: string) => {
      setPapers((list) => {
        const existing = list.find((p) => p.id === paperID);
        if (!existing) {
          // 列表里还没有(刚上传未刷新),异步补齐。
          api
            .listPapers()
            .then((fresh) => setPapers(Array.isArray(fresh) ? fresh : []))
            .catch(() => {});
          return list;
        }
        if (existing.status === status) return list;
        if (status === "ready") {
          toast(`「${paperTitle(existing)}」已就绪,可提问`);
        } else if (status === "failed") {
          toast(`「${paperTitle(existing)}」解析失败:${detail || "未知原因"}`, "error");
        }
        return list.map((p) =>
          p.id === paperID
            ? { ...p, status, fail_reason: detail || p.fail_reason }
            : p,
        );
      });
    },
    [toast],
  );

  // 报告就绪:记入对应论文,报告面板据此免轮询直接拉缓存。
  const applyReportReady = useCallback(
    (paperID: string, reportType: ReportType) => {
      setReportReady((prev) => ({
        ...prev,
        [paperID]: { ...prev[paperID], [reportType]: true },
      }));
    },
    [],
  );

  const connectWs = useCallback(
    (jwt: string) => {
      disconnectWs();
      const socket = api.openStatusSocket(
        jwt,
        (e) => applyStatusEvent(e.paper_id, e.status, e.detail),
        (e) => applyReportReady(e.paper_id, e.report_type),
      );
      if (!socket) return;
      wsRef.current = socket;
      socket.addEventListener("close", () => {
        if (wsRef.current !== socket) return;
        wsRef.current = null;
        if (jwt) wsRetryRef.current = window.setTimeout(() => connectWs(jwt), 3000);
      });
      socket.addEventListener("error", () => socket.close());
    },
    [applyStatusEvent, applyReportReady, disconnectWs],
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
        setPapers((list) =>
          list.map((p) => {
            const u = updates.find((x) => x.id === p.id);
            return u ? { ...p, ...u } : p;
          }),
        );
      } catch {
        stopPolling();
      }
    }, 4200);
  }, [stopPolling]);

  // 列表变化时按需开/停轮询。
  useEffect(() => {
    if (!token) return;
    if (papers.some((p) => !isSettled(p.status))) {
      if (!pollRef.current) startPolling();
    } else {
      stopPolling();
    }
  }, [papers, token, startPolling, stopPolling]);

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
    setSessions(Array.isArray(list) ? list : []);
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

  const login = useCallback(
    async (studentID: string, password: string) => {
      const data = await api.login(studentID, password);
      persist(
        { student_id: data.student_id, name: data.name, email: data.email },
        data.token,
      );
      connectWs(data.token);
      const [pl, sl] = await Promise.all([
        api.listPapers(),
        api.listSessions(),
      ]);
      const paperList = Array.isArray(pl) ? pl : [];
      const sessionList = Array.isArray(sl) ? sl : [];
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
      toast("登录成功");
    },
    [connectWs, openSession, persist, toast],
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

        const data = await api.sendMessage(sessionID, query);
        if (data?.message) {
          const assistant: Message = { ...data.message };
          // 助教消息的真实 ID 落 Session 后才有,即时应答 ID 为空,
          // 这里补个本地唯一 ID 避免多轮渲染 key 冲突;重开会话时由 listMessages 还原真实 ID。
          if (!assistant.id) assistant.id = `local-a-${Date.now()}`;
          if (data.meta) assistant.meta = data.meta;
          setMessages((list) => [...list, assistant]);
        }
        await refreshSessions();
      } finally {
        setSending(false);
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
        const sessionList = Array.isArray(sl) ? sl : [];
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
    toasts,
    activePaper,
    activeSession,
    reportReady,
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
  };

  return <AppContext.Provider value={value}>{children}</AppContext.Provider>;
}

export function useApp(): AppContextValue {
  const ctx = useContext(AppContext);
  if (!ctx) throw new Error("useApp 必须在 AppProvider 内使用");
  return ctx;
}
