"use client";

import {
  ArrowLeft,
  CheckCircle2,
  FileText,
  Gauge,
  HardDrive,
  Loader2,
  LogOut,
  MessagesSquare,
  RefreshCw,
  ScrollText,
  Search,
  ShieldCheck,
  Trash2,
  Users,
} from "lucide-react";
import { AnimatePresence, motion, useReducedMotion } from "motion/react";
import Link from "next/link";
import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type Dispatch,
  type FormEvent,
  type ReactNode,
  type SetStateAction,
} from "react";

import {
  AdminApiError,
  ADMIN_AUTH_KEY,
  adminRequest,
  readSavedAuth,
  type AdminAuth,
  type AdminProfile,
} from "@/components/gopherpaper/admin-auth";
import { GlassSurface } from "@/components/reactbits/glass-surface";
import { Threads } from "@/components/reactbits/threads";
import { AdminDashboard } from "./dashboard";
import { UsersPanel } from "./users-panel";
import { LogsPanel } from "./logs-panel";
import { ExportBar } from "./export-bar";
import { SessionsPanel } from "./sessions-panel";
import { AdminsPanel } from "./admins-panel";
import { BatchPanel } from "./batch-panel";
import { StoragePanel } from "./storage-panel";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { PasswordInput } from "@/components/ui/password-input";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";

const ADMIN_AUTH_EASE = [0.16, 1, 0.3, 1] as const;

type AuthMode = "login" | "register";

type AdminRegisterForm = {
  username: string;
  email: string;
  name: string;
  password: string;
  code: string;
  registration_code: string;
};

type AdminPaper = {
  id: string;
  owner_id: string;
  owner_name?: string;
  owner_email?: string;
  owner_class?: string;
  title: string;
  file_name: string;
  size: number;
  status: string;
  fail_reason?: string;
  page_count: number;
  created_at: string;
  updated_at: string;
};

type PaperList = {
  items: AdminPaper[];
  total: number;
  page: number;
  page_size: number;
};

type DateRangeFilter = "" | "today" | "7d" | "month";
type TabKey =
  | "dashboard"
  | "users"
  | "papers"
  | "sessions"
  | "logs"
  | "storage"
  | "admins";

const ADMIN_TABS: Array<{ key: TabKey; label: string; icon: typeof Gauge }> = [
  { key: "dashboard", label: "数据看板", icon: Gauge },
  { key: "papers", label: "论文库", icon: FileText },
  { key: "storage", label: "存储", icon: HardDrive },
  { key: "users", label: "用户管理", icon: Users },
  { key: "sessions", label: "会话管理", icon: MessagesSquare },
  { key: "logs", label: "日志", icon: ScrollText },
  { key: "admins", label: "管理员", icon: ShieldCheck },
];
type PaperFilters = {
  query: string;
  status: string;
  date_range: DateRangeFilter;
};

const DEFAULT_PAPER_PAGE_SIZE = 10;

function emptyPaperList(page = 1, pageSize = DEFAULT_PAPER_PAGE_SIZE): PaperList {
  return {
    items: [],
    total: 0,
    page,
    page_size: pageSize,
  };
}

function normalizePaperList(
  data: PaperList | null | undefined,
  page = 1,
  pageSize = DEFAULT_PAPER_PAGE_SIZE,
): PaperList {
  return {
    items: Array.isArray(data?.items) ? data.items : [],
    total: typeof data?.total === "number" ? data.total : 0,
    page: typeof data?.page === "number" && data.page > 0 ? data.page : page,
    page_size:
      typeof data?.page_size === "number" && data.page_size > 0 ? data.page_size : pageSize,
  };
}

function AdminAuthBackdrop() {
  const reduce = useReducedMotion();
  const [mounted, setMounted] = useState(false);

  useEffect(() => {
    setMounted(true);
  }, []);

  return (
    <div aria-hidden className="pointer-events-none absolute inset-0 -z-10 overflow-hidden">
      {mounted && !reduce && (
        <div className="absolute inset-0 opacity-[0.18] mix-blend-multiply dark:opacity-[0.24] dark:mix-blend-screen">
          <Threads
            color={[0.08, 0.55, 0.6]}
            amplitude={1.08}
            distance={0.22}
            enableMouseInteraction
          />
        </div>
      )}
      <div className="absolute -top-[28%] left-1/2 h-[42rem] w-[58rem] -translate-x-1/2 rounded-full bg-primary/[0.085] blur-3xl" />
      <div className="absolute -right-[14%] top-[6%] h-[34rem] w-[34rem] rounded-full bg-foreground/[0.045] blur-3xl" />
      <div
        className="absolute inset-0 opacity-[0.5]"
        style={{
          backgroundImage:
            "linear-gradient(to right, color-mix(in oklch, var(--border) 60%, transparent) 1px, transparent 1px)",
          backgroundSize: "min(7.5rem, 12vw) 100%",
          maskImage: "radial-gradient(120% 80% at 50% 0%, #000 30%, transparent 78%)",
        }}
      />
      <div
        className="absolute inset-0 opacity-[0.18] mix-blend-soft-light"
        style={{
          backgroundImage:
            "radial-gradient(circle at 1px 1px, color-mix(in oklch, var(--foreground) 28%, transparent) 1px, transparent 0)",
          backgroundSize: "18px 18px",
        }}
      />
      <div className="absolute inset-x-0 top-0 h-px bg-gradient-to-r from-transparent via-sienna/40 to-transparent" />
    </div>
  );
}

function AdminBrand({ authed }: { authed: boolean }) {
  return (
    <Link
      href="/"
      className="group flex items-center gap-2.5 rounded-md outline-none focus-visible:ring-2 focus-visible:ring-ring/60"
    >
      <span
        className={
          authed
            ? "flex size-9 items-center justify-center rounded-lg bg-primary/10 text-primary"
            : "flex size-9 items-center justify-center overflow-hidden rounded-lg bg-primary/10 shadow-sm transition-transform group-hover:scale-105 group-active:scale-95"
        }
      >
        {authed ? (
          <ShieldCheck className="size-5" />
        ) : (
          <img src="/mascot-gopher.png" alt="GopherPaper" className="size-full object-cover" />
        )}
      </span>
      <span>
        <span className="block text-sm font-semibold tracking-tight">
          {authed ? "GopherPaper Admin" : "GopherPaper"}
        </span>
        <span className="block text-xs text-muted-foreground">
          {authed ? "论文库管理控制台" : "管理员后台"}
        </span>
      </span>
    </Link>
  );
}

export default function AdminPage() {
  const [auth, setAuth] = useState<AdminAuth | null>(null);
  const [mode, setMode] = useState<AuthMode>("login");
  const [loginForm, setLoginForm] = useState({ email: "", password: "" });
  const [registerForm, setRegisterForm] = useState<AdminRegisterForm>({
    username: "",
    email: "",
    name: "",
    password: "",
    code: "",
    registration_code: "",
  });
  const [papers, setPapers] = useState<PaperList>(() => emptyPaperList());
  const [filters, setFilters] = useState<PaperFilters>({
    query: "",
    status: "",
    date_range: "",
  });
  const [tab, setTab] = useState<TabKey>("dashboard");
  const [loading, setLoading] = useState(false);
  const [tableLoading, setTableLoading] = useState(false);
  const [busy, setBusy] = useState(false);
  const [codeBusy, setCodeBusy] = useState(false);
  const [message, setMessage] = useState<{ type: "ok" | "error"; text: string } | null>(
    null,
  );
  const [codeNotice, setCodeNotice] = useState<string | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<AdminPaper | null>(null);
  const [deleteConfirm, setDeleteConfirm] = useState("");
  const hydratedRef = useRef(false);
  const messageTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const codeNoticeTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const token = auth?.token || "";

  const persistAuth = useCallback((next: AdminAuth | null) => {
    setAuth(next);
    if (next) window.localStorage.setItem(ADMIN_AUTH_KEY, JSON.stringify(next));
    else window.localStorage.removeItem(ADMIN_AUTH_KEY);
  }, []);

  const clearMessage = useCallback(() => {
    if (messageTimerRef.current) {
      clearTimeout(messageTimerRef.current);
      messageTimerRef.current = null;
    }
    setMessage(null);
  }, []);

  const flash = useCallback((type: "ok" | "error", text: string) => {
    if (messageTimerRef.current) {
      clearTimeout(messageTimerRef.current);
    }
    setMessage({ type, text });
    messageTimerRef.current = setTimeout(() => {
      setMessage(null);
      messageTimerRef.current = null;
    }, 3200);
  }, []);

  const showCodeNotice = useCallback((text: string) => {
    if (codeNoticeTimerRef.current) {
      clearTimeout(codeNoticeTimerRef.current);
    }
    setCodeNotice(text);
    codeNoticeTimerRef.current = setTimeout(() => {
      setCodeNotice(null);
      codeNoticeTimerRef.current = null;
    }, 2400);
  }, []);

  const loadPapers = useCallback(
    async (page = papers.page, jwt = token, nextFilters = filters) => {
      if (!jwt) return;
      setTableLoading(true);
      try {
        const pageSize = DEFAULT_PAPER_PAGE_SIZE;
        const params = new URLSearchParams({
          page: String(page),
          page_size: String(pageSize),
        });
        if (nextFilters.query.trim()) params.set("query", nextFilters.query.trim());
        if (nextFilters.status) params.set("status", nextFilters.status);
        if (nextFilters.date_range) params.set("date_range", nextFilters.date_range);
        const data = await adminRequest<PaperList | null>(`/admin/papers?${params}`, jwt);
        setPapers(normalizePaperList(data, page, pageSize));
      } finally {
        setTableLoading(false);
      }
    },
    [filters, papers.page, token],
  );

  const refreshAll = useCallback(
    async (jwt = token) => {
      if (!jwt) return;
      setLoading(true);
      try {
        await loadPapers(1, jwt);
      } catch (err) {
        if (err instanceof AdminApiError && err.status === 401) {
          persistAuth(null);
        }
        flash("error", err instanceof Error ? err.message : "加载失败");
      } finally {
        setLoading(false);
      }
    },
    [flash, loadPapers, persistAuth, token],
  );

  useEffect(() => {
    if (hydratedRef.current) return;
    hydratedRef.current = true;
    const saved = readSavedAuth();
    if (!saved?.token) return;
    persistAuth(saved);
    void refreshAll(saved.token);
  }, [persistAuth, refreshAll]);

  useEffect(() => {
    return () => {
      if (messageTimerRef.current) {
        clearTimeout(messageTimerRef.current);
      }
      if (codeNoticeTimerRef.current) {
        clearTimeout(codeNoticeTimerRef.current);
      }
    };
  }, []);

  const onLogin = async (e: FormEvent) => {
    e.preventDefault();
    if (busy) return;
    setBusy(true);
    try {
      const data = await adminRequest<AdminAuth>("/admin/login", "", {
        method: "POST",
        body: JSON.stringify(loginForm),
      });
      persistAuth(data);
      flash("ok", "管理员登录成功");
      await refreshAll(data.token);
    } catch (err) {
      flash("error", err instanceof Error ? err.message : "登录失败");
    } finally {
      setBusy(false);
    }
  };

  const onRegister = async (e: FormEvent) => {
    e.preventDefault();
    if (busy) return;
    setBusy(true);
    try {
      await adminRequest<AdminProfile>("/admin/register", "", {
        method: "POST",
        body: JSON.stringify(registerForm),
      });
      flash("ok", "管理员注册成功，请登录");
      setMode("login");
      setLoginForm({ email: registerForm.email, password: registerForm.password });
    } catch (err) {
      flash("error", err instanceof Error ? err.message : "注册失败");
    } finally {
      setBusy(false);
    }
  };

  const sendAdminCode = async () => {
    if (codeBusy) return;
    const email = registerForm.email.trim();
    if (!email) {
      flash("error", "请先填写邮箱");
      return;
    }
    setCodeBusy(true);
    try {
      await adminRequest<null>("/admin/send-code", "", {
        method: "POST",
        body: JSON.stringify({ email }),
      });
      showCodeNotice("验证码已发送，请查看邮箱");
    } catch (err) {
      flash("error", err instanceof Error ? err.message : "验证码发送失败");
    } finally {
      setCodeBusy(false);
    }
  };

  const onLogout = () => {
    persistAuth(null);
    setPapers(emptyPaperList());
    setTab("dashboard");
    flash("ok", "已退出管理员后台");
  };

  const onSearch = async (e: FormEvent) => {
    e.preventDefault();
    try {
      await loadPapers(1);
    } catch (err) {
      flash("error", err instanceof Error ? err.message : "查询失败");
    }
  };

  const onDelete = async () => {
    if (!deleteTarget || deleteConfirm !== deleteTarget.id || !token) return;
    setBusy(true);
    try {
      await adminRequest<null>(`/admin/papers/${encodeURIComponent(deleteTarget.id)}`, token, {
        method: "DELETE",
      });
      flash("ok", "论文已删除");
      setDeleteTarget(null);
      setDeleteConfirm("");
      await loadPapers(papers.page);
    } catch (err) {
      flash("error", err instanceof Error ? err.message : "删除失败");
    } finally {
      setBusy(false);
    }
  };

  const pageCount = useMemo(
    () => Math.max(1, Math.ceil(papers.total / DEFAULT_PAPER_PAGE_SIZE)),
    [papers.total],
  );

  return (
    <main
      className={
        auth
          ? "min-h-dvh bg-background text-foreground"
          : "relative isolate min-h-dvh overflow-hidden bg-background text-foreground"
      }
    >
      {!auth && <AdminAuthBackdrop />}
      <div
        className={
          auth
            ? "mx-auto flex min-h-dvh w-full max-w-[92rem] flex-col px-4 py-5 sm:px-6 lg:px-8"
            : "mx-auto flex min-h-dvh w-full max-w-[78rem] flex-col px-5 sm:px-8"
        }
      >
        <header
          className={
            auth
              ? "flex flex-wrap items-center justify-between gap-3 border-b border-border pb-4"
              : "flex items-center justify-between py-6"
          }
        >
          <AdminBrand authed={Boolean(auth)} />
          {auth ? (
            <div className="flex items-center gap-2">
              <span className="hidden text-sm text-muted-foreground sm:inline">
                {auth.admin.username}
              </span>
              <Link
                href="/admin/analytics"
                className="hidden rounded-lg border border-border px-3 py-1.5 text-sm font-medium text-muted-foreground transition-colors hover:bg-muted hover:text-foreground sm:inline-flex"
              >
                数据分析
              </Link>
              <Link
                href="/admin/reports"
                className="hidden rounded-lg border border-border px-3 py-1.5 text-sm font-medium text-muted-foreground transition-colors hover:bg-muted hover:text-foreground sm:inline-flex"
              >
                统计报表
              </Link>
              <Button variant="outline" size="sm" onClick={() => void refreshAll()}>
                {loading ? <Loader2 className="size-4 animate-spin" /> : <RefreshCw className="size-4" />}
                刷新
              </Button>
              <Button variant="ghost" size="sm" onClick={onLogout}>
                <LogOut className="size-4" />
                退出
              </Button>
            </div>
          ) : (
            <GlassSurface className="rounded-full" contentClassName="p-1">
              <Link
                className="inline-flex h-8 items-center gap-1.5 rounded-full px-3 text-sm font-medium text-muted-foreground transition-colors hover:bg-background/70 hover:text-foreground"
                href="/"
              >
                <ArrowLeft className="size-4" />
                返回工作台
              </Link>
            </GlassSurface>
          )}
        </header>

        {message && (
          <button
            type="button"
            onClick={clearMessage}
            className={`mt-4 rounded-lg border px-3 py-2 text-left text-sm ${
              auth ? "" : "mx-auto w-full max-w-md"
            } ${
              message.type === "ok"
                ? "border-emerald-200 bg-emerald-50 text-emerald-800"
                : "border-red-200 bg-red-50 text-red-800"
            }`}
          >
            {message.text}
          </button>
        )}

        {auth ? (
          <section className="flex flex-1 flex-col gap-5 py-5">
            <Tabs value={tab} onValueChange={(v) => setTab(v as TabKey)}>
              <TabsList variant="line" className="flex-wrap">
                {ADMIN_TABS.map(({ key, label, icon: Icon }) => (
                  <TabsTrigger key={key} value={key}>
                    <Icon className="size-4" />
                    {label}
                  </TabsTrigger>
                ))}
              </TabsList>

              <TabsContent value="dashboard" className="flex flex-col gap-5">
                <AdminDashboard token={token} onChanged={() => void refreshAll()} />
              </TabsContent>

              <TabsContent value="users">
                <UsersPanel token={token} />
              </TabsContent>

              <TabsContent value="papers" className="flex flex-col gap-5">
                <section className="rounded-lg border border-border bg-card">
                  <div className="flex flex-wrap items-center justify-between gap-3 border-b border-border p-4">
                    <div>
                      <h1 className="text-base font-semibold">论文库管理</h1>
                      <p className="mt-1 text-sm text-muted-foreground">
                        全库搜索、筛选并删除论文及其派生数据。
                      </p>
                    </div>
                    <span className="text-sm text-muted-foreground">共 {papers.total} 篇</span>
                  </div>
                  <form
                    className="flex flex-col gap-3 border-b border-border p-4 sm:flex-row sm:items-center"
                    onSubmit={onSearch}
                  >
                    <label className="relative min-w-0 flex-1">
                      <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
                      <Input
                        className="pl-9"
                        placeholder="标题、文件名、论文 ID、作者账号"
                        value={filters.query}
                        onChange={(e) => setFilters((f) => ({ ...f, query: e.target.value }))}
                      />
                    </label>
                    <select
                      className="h-8 w-full rounded-lg border border-input bg-background px-2 text-sm outline-none focus-visible:ring-2 focus-visible:ring-ring sm:w-40"
                      value={filters.status}
                      onChange={(e) => setFilters((f) => ({ ...f, status: e.target.value }))}
                    >
                      <option value="">全部状态</option>
                      <option value="uploaded">uploaded</option>
                      <option value="parsing">parsing</option>
                      <option value="extracted">extracted</option>
                      <option value="indexed">indexed</option>
                      <option value="ready">ready</option>
                      <option value="failed">failed</option>
                    </select>
                    <select
                      className="h-8 w-full rounded-lg border border-input bg-background px-2 text-sm outline-none focus-visible:ring-2 focus-visible:ring-ring sm:w-44"
                      value={filters.date_range}
                      onChange={(e) =>
                        setFilters((f) => ({
                          ...f,
                          date_range: e.target.value as DateRangeFilter,
                        }))
                      }
                    >
                      <option value="">全部时间</option>
                      <option value="today">今天</option>
                      <option value="7d">近 7 天</option>
                      <option value="month">近 1 个月</option>
                    </select>
                    <Button className="w-full sm:w-24" type="submit" disabled={tableLoading}>
                      {tableLoading && <Loader2 className="size-4 animate-spin" />}
                      查询
                    </Button>
                  </form>
                  <PaperTable
                    items={papers.items}
                    loading={tableLoading}
                    onDelete={(paper) => {
                      setDeleteTarget(paper);
                      setDeleteConfirm("");
                    }}
                  />
                  <div className="flex flex-wrap items-center justify-between gap-3 border-t border-border p-4">
                    <span className="text-sm text-muted-foreground">
                      第 {papers.page} / {pageCount} 页
                    </span>
                    <div className="flex gap-2">
                      <Button
                        variant="outline"
                        disabled={papers.page <= 1 || tableLoading}
                        onClick={() => void loadPapers(papers.page - 1)}
                      >
                        上一页
                      </Button>
                      <Button
                        variant="outline"
                        disabled={papers.page >= pageCount || tableLoading}
                        onClick={() => void loadPapers(papers.page + 1)}
                      >
                        下一页
                      </Button>
                    </div>
                  </div>
                </section>
                <BatchPanel token={token} onChanged={() => void refreshAll()} />
              </TabsContent>

              <TabsContent value="sessions">
                <SessionsPanel token={token} />
              </TabsContent>

              <TabsContent value="logs">
                <LogsPanel token={token} />
              </TabsContent>

              <TabsContent value="storage">
                <StoragePanel token={token} />
              </TabsContent>

              <TabsContent value="admins" className="flex flex-col gap-5">
                <AdminsPanel token={token} />
                <ExportBar token={token} />
              </TabsContent>
            </Tabs>
          </section>
        ) : (
          <AuthPanel
            mode={mode}
            setMode={setMode}
            busy={busy}
            codeBusy={codeBusy}
            loginForm={loginForm}
            setLoginForm={setLoginForm}
            registerForm={registerForm}
            setRegisterForm={setRegisterForm}
            onSendCode={sendAdminCode}
            onLogin={onLogin}
            onRegister={onRegister}
          />
        )}
      </div>

      {deleteTarget && (
        <div className="fixed inset-0 z-50 grid place-items-center bg-black/20 p-4 backdrop-blur-sm">
          <div className="w-full max-w-md rounded-lg border border-border bg-popover p-5 shadow-panel">
            <div className="flex items-start gap-3">
              <span className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-destructive/10 text-destructive">
                <Trash2 className="size-5" />
              </span>
              <div>
                <h2 className="text-base font-semibold">确认删除论文</h2>
                <p className="mt-1 text-sm leading-6 text-muted-foreground">
                  将删除 MySQL 记录、向量库 chunks、知识图谱节点、会话、PDF 和图片目录。
                </p>
              </div>
            </div>
            <div className="mt-4 rounded-lg border border-border bg-muted/40 p-3 text-sm">
              <p className="font-medium">{paperTitle(deleteTarget)}</p>
              <p className="mt-1 font-mono text-xs text-muted-foreground">{deleteTarget.id}</p>
            </div>
            <Label className="mt-4 block" htmlFor="delete-confirm">
              输入完整论文 ID 确认删除
            </Label>
            <Input
              id="delete-confirm"
              className="mt-2 font-mono text-xs"
              value={deleteConfirm}
              onChange={(e) => setDeleteConfirm(e.target.value)}
            />
            <div className="mt-5 flex justify-end gap-2">
              <Button
                variant="outline"
                onClick={() => {
                  setDeleteTarget(null);
                  setDeleteConfirm("");
                }}
              >
                取消
              </Button>
              <Button
                variant="destructive"
                disabled={busy || deleteConfirm !== deleteTarget.id}
                onClick={() => void onDelete()}
              >
                {busy ? <Loader2 className="size-4 animate-spin" /> : <Trash2 className="size-4" />}
                删除
              </Button>
            </div>
          </div>
        </div>
      )}

      {codeNotice && (
        <div
          role="status"
          className="pointer-events-none fixed left-1/2 top-5 z-[60] flex -translate-x-1/2 items-center gap-2 rounded-lg border border-emerald-200 bg-emerald-50 px-4 py-2 text-sm font-medium text-emerald-800 shadow-panel"
        >
          <CheckCircle2 className="size-4" />
          {codeNotice}
        </div>
      )}
    </main>
  );
}

type AdminAuthPanelProps = {
  mode: AuthMode;
  setMode: (mode: AuthMode) => void;
  busy: boolean;
  codeBusy: boolean;
  loginForm: { email: string; password: string };
  setLoginForm: Dispatch<SetStateAction<{ email: string; password: string }>>;
  registerForm: AdminRegisterForm;
  setRegisterForm: Dispatch<SetStateAction<AdminRegisterForm>>;
  onSendCode: () => void;
  onLogin: (e: FormEvent) => void;
  onRegister: (e: FormEvent) => void;
};

function AuthPanel(props: AdminAuthPanelProps) {
  const { mode, setMode } = props;
  const isLogin = mode === "login";

  return (
    <section className="grid flex-1 place-items-center py-8 sm:py-10">
      <motion.div
        initial={{ opacity: 0, y: 18, scale: 0.98 }}
        animate={{ opacity: 1, y: 0, scale: 1 }}
        transition={{ duration: 0.5, ease: ADMIN_AUTH_EASE }}
        className="w-full max-w-md"
      >
        <GlassSurface className="rounded-2xl" contentClassName="p-7 sm:p-8">
        <div className="mb-6">
          <h1 className="font-serif text-[1.7rem] font-semibold tracking-tight">
            {isLogin ? "进入管理员后台" : "创建管理员账号"}
          </h1>
          <p className="mt-2 text-sm leading-6 text-muted-foreground">
            {isLogin
              ? "使用管理员邮箱登录，继续管理论文库。"
              : "使用注册码与邮箱验证码创建后台账号。"}
          </p>
        </div>

        <div className="mb-6 grid grid-cols-2 gap-1 rounded-lg border border-border bg-muted/50 p-1">
          {(["login", "register"] as const).map((nextMode) => (
            <button
              key={nextMode}
              type="button"
              className={`h-9 rounded-md text-sm font-medium transition-colors ${
                mode === nextMode
                  ? "bg-card text-foreground shadow-sm"
                  : "text-muted-foreground hover:text-foreground"
              }`}
              onClick={() => setMode(nextMode)}
            >
              {nextMode === "login" ? "登录" : "注册"}
            </button>
          ))}
        </div>

        <AnimatePresence mode="wait" initial={false}>
          {isLogin ? (
            <motion.div
              key="admin-login"
              initial={{ opacity: 0, x: -10 }}
              animate={{ opacity: 1, x: 0 }}
              exit={{ opacity: 0, x: -10 }}
              transition={{ duration: 0.24, ease: ADMIN_AUTH_EASE }}
            >
              <AdminLoginForm {...props} />
            </motion.div>
          ) : (
            <motion.div
              key="admin-register"
              initial={{ opacity: 0, x: 10 }}
              animate={{ opacity: 1, x: 0 }}
              exit={{ opacity: 0, x: 10 }}
              transition={{ duration: 0.24, ease: ADMIN_AUTH_EASE }}
            >
              <AdminRegisterFormPanel {...props} />
            </motion.div>
          )}
        </AnimatePresence>

        <p className="mt-6 text-center text-xs text-muted-foreground">
          {isLogin ? (
            <>
              还没有管理员账号？
              <button
                type="button"
                onClick={() => setMode("register")}
                className="ml-1 font-medium text-primary underline-offset-4 hover:underline"
              >
                立即注册
              </button>
            </>
          ) : (
            <>
              已有管理员账号？
              <button
                type="button"
                onClick={() => setMode("login")}
                className="ml-1 font-medium text-primary underline-offset-4 hover:underline"
              >
                直接登录
              </button>
            </>
          )}
        </p>
        </GlassSurface>
      </motion.div>
    </section>
  );
}

function AdminLoginForm({
  busy,
  loginForm,
  setLoginForm,
  onLogin,
}: AdminAuthPanelProps) {
  return (
    <form className="space-y-4" onSubmit={onLogin}>
      <Field label="邮箱">
        <Input
          className="h-10"
          type="email"
          value={loginForm.email}
          autoComplete="email"
          required
          onChange={(e) => setLoginForm((f) => ({ ...f, email: e.target.value }))}
        />
      </Field>
      <Field label="密码">
        <PasswordInput
          className="h-10"
          value={loginForm.password}
          autoComplete="current-password"
          required
          onChange={(e) => setLoginForm((f) => ({ ...f, password: e.target.value }))}
        />
      </Field>
      <Button className="h-11 w-full gap-2" type="submit" disabled={busy}>
        {busy && <Loader2 className="size-4 animate-spin" />}
        登录后台
      </Button>
    </form>
  );
}

function AdminRegisterFormPanel({
  busy,
  codeBusy,
  registerForm,
  setRegisterForm,
  onSendCode,
  onRegister,
}: AdminAuthPanelProps) {
  return (
    <form className="space-y-4" onSubmit={onRegister}>
      <div className="grid gap-4 sm:grid-cols-2">
        <Field label="姓名">
          <Input
            className="h-10"
            value={registerForm.name}
            autoComplete="name"
            onChange={(e) => setRegisterForm((f) => ({ ...f, name: e.target.value }))}
          />
        </Field>
        <Field label="用户名">
          <Input
            className="h-10"
            value={registerForm.username}
            autoComplete="username"
            required
            onChange={(e) => setRegisterForm((f) => ({ ...f, username: e.target.value }))}
          />
        </Field>
      </div>

      <div className="space-y-2">
        <Label htmlFor="admin-register-email">邮箱</Label>
        <Input
          id="admin-register-email"
          className="h-10"
          type="email"
          value={registerForm.email}
          autoComplete="email"
          required
          onChange={(e) => setRegisterForm((f) => ({ ...f, email: e.target.value }))}
        />
      </div>

      <div className="space-y-2">
        <Label htmlFor="admin-register-code">邮箱验证码</Label>
        <div className="grid gap-2 sm:grid-cols-[1fr_auto]">
          <Input
            id="admin-register-code"
            className="h-10"
            value={registerForm.code}
            autoComplete="one-time-code"
            inputMode="numeric"
            maxLength={6}
            required
            onChange={(e) =>
              setRegisterForm((f) => ({
                ...f,
                code: e.target.value.replace(/\D/g, "").slice(0, 6),
              }))
            }
          />
          <Button
            className="h-10 whitespace-nowrap"
            type="button"
            variant="outline"
            disabled={busy || codeBusy || !registerForm.email.trim()}
            onClick={() => void onSendCode()}
          >
            {codeBusy && <Loader2 className="size-4 animate-spin" />}
            {codeBusy ? "发送中" : "发送验证码"}
          </Button>
        </div>
      </div>

      <div className="grid gap-4 sm:grid-cols-2">
        <Field label="密码">
          <PasswordInput
            className="h-10"
            value={registerForm.password}
            autoComplete="new-password"
            minLength={6}
            required
            onChange={(e) => setRegisterForm((f) => ({ ...f, password: e.target.value }))}
          />
        </Field>
        <Field label="管理员注册码">
          <PasswordInput
            className="h-10"
            value={registerForm.registration_code}
            required
            onChange={(e) =>
              setRegisterForm((f) => ({ ...f, registration_code: e.target.value }))
            }
          />
        </Field>
      </div>

      <Button className="h-11 w-full gap-2" type="submit" disabled={busy}>
        {busy && <Loader2 className="size-4 animate-spin" />}
        创建管理员
      </Button>
    </form>
  );
}

function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <label className="block space-y-2">
      <span className="text-sm font-medium">{label}</span>
      {children}
    </label>
  );
}

function PaperTable({
  items,
  loading,
  onDelete,
}: {
  items: AdminPaper[];
  loading: boolean;
  onDelete: (paper: AdminPaper) => void;
}) {
  const rows = Array.isArray(items) ? items : [];

  return (
    <div className="overflow-x-auto">
      <table className="w-full min-w-[64rem] text-left text-sm">
        <thead className="bg-muted/50 text-xs uppercase text-muted-foreground">
          <tr>
            <th className="px-4 py-3 font-medium">论文</th>
            <th className="px-4 py-3 font-medium">Owner</th>
            <th className="px-4 py-3 font-medium">状态</th>
            <th className="px-4 py-3 font-medium">规模</th>
            <th className="px-4 py-3 font-medium">更新时间</th>
            <th className="px-4 py-3 text-right font-medium">操作</th>
          </tr>
        </thead>
        <tbody>
          {loading ? (
            <tr>
              <td className="px-4 py-10 text-center text-muted-foreground" colSpan={6}>
                <Loader2 className="mx-auto mb-2 size-5 animate-spin" />
                正在加载论文列表
              </td>
            </tr>
          ) : rows.length === 0 ? (
            <tr>
              <td className="px-4 py-10 text-center text-muted-foreground" colSpan={6}>
                暂无匹配论文
              </td>
            </tr>
          ) : (
            rows.map((paper) => (
              <tr key={paper.id} className="border-t border-border align-top">
                <td className="max-w-[28rem] px-4 py-3">
                  <p className="line-clamp-2 font-medium">{paperTitle(paper)}</p>
                  <p className="mt-1 truncate text-xs text-muted-foreground">{paper.file_name}</p>
                  <p className="mt-1 font-mono text-[11px] text-muted-foreground">{paper.id}</p>
                </td>
                <td className="px-4 py-3">
                  <p className="font-medium">{paper.owner_name || paper.owner_id}</p>
                  <p className="mt-1 text-xs text-muted-foreground">{paper.owner_email || paper.owner_id}</p>
                </td>
                <td className="px-4 py-3">
                  <span className={`inline-flex rounded-md px-2 py-1 text-xs font-medium ${statusClass(paper.status)}`}>
                    {paper.status}
                  </span>
                  {paper.fail_reason && (
                    <p className="mt-2 max-w-[12rem] text-xs text-destructive">{paper.fail_reason}</p>
                  )}
                </td>
                <td className="px-4 py-3 text-muted-foreground">
                  <p>{formatBytes(paper.size)}</p>
                  <p className="mt-1 text-xs">{paper.page_count || "—"} 页</p>
                </td>
                <td className="px-4 py-3 text-muted-foreground">{formatDate(paper.updated_at || paper.created_at)}</td>
                <td className="px-4 py-3 text-right">
                  <Button variant="destructive" size="sm" onClick={() => onDelete(paper)}>
                    <Trash2 className="size-4" />
                    删除
                  </Button>
                </td>
              </tr>
            ))
          )}
        </tbody>
      </table>
    </div>
  );
}

function paperTitle(paper: AdminPaper) {
  return paper.title?.trim() || paper.file_name || paper.id;
}

function statusClass(status: string) {
  switch (status) {
    case "ready":
      return "bg-emerald-50 text-emerald-700";
    case "failed":
      return "bg-red-50 text-red-700";
    case "parsing":
    case "extracted":
    case "indexed":
      return "bg-sky-50 text-sky-700";
    default:
      return "bg-muted text-muted-foreground";
  }
}

function formatBytes(value: number) {
  if (!value) return "—";
  const units = ["B", "KB", "MB", "GB"];
  let size = value;
  let unit = 0;
  while (size >= 1024 && unit < units.length - 1) {
    size /= 1024;
    unit += 1;
  }
  return `${size.toFixed(unit === 0 ? 0 : 1)} ${units[unit]}`;
}

function formatDate(value: string) {
  if (!value) return "—";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "—";
  return new Intl.DateTimeFormat("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}
