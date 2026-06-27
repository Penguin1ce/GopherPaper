"use client";

import {
  BarChart3,
  CheckCircle2,
  Database,
  FileCheck2,
  FileSearch,
  Loader2,
  LogOut,
  RefreshCw,
  Search,
  ShieldCheck,
  Trash2,
} from "lucide-react";
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

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

const API_BASE = process.env.NEXT_PUBLIC_API_BASE || "/api/v1";
const AUTH_KEY = "gopherpaper.admin.auth";

type AuthMode = "login" | "register";

type AdminRegisterForm = {
  username: string;
  email: string;
  name: string;
  password: string;
  code: string;
  registration_code: string;
};

type Envelope<T> = {
  code: number;
  message: string;
  data?: T;
};

type AdminProfile = {
  id: number;
  username: string;
  email: string;
  name: string;
};

type AdminAuth = {
  token: string;
  admin: AdminProfile;
};

type AdminOverview = {
  paper_count: number;
  service_call_count: number;
  service_success_rate: number | null;
  parse_success_rate: number | null;
  vector_count: number | null;
  vector_error?: string;
  service_success: number;
  service_failed: number;
  parse_ready: number;
  parse_failed: number;
  breakdown: Array<{
    service_type: string;
    total: number;
    success: number;
    failed: number;
  }>;
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

class AdminApiError extends Error {
  status: number;

  constructor(message: string, status: number) {
    super(message);
    this.status = status;
  }
}

async function adminRequest<T>(
  path: string,
  token: string,
  options: RequestInit = {},
): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers: {
      "Content-Type": "application/json",
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...(options.headers as Record<string, string> | undefined),
    },
  });
  const text = await res.text();
  let body: Envelope<T> = { code: 0, message: "" };
  if (text) {
    try {
      body = JSON.parse(text) as Envelope<T>;
    } catch {
      const contentType = res.headers.get("content-type") || "";
      body = {
        code: res.ok ? 0 : res.status,
        message: contentType.includes("text/html")
          ? `请求地址不存在或前端代理未生效：${API_BASE}${path}`
          : text,
      };
    }
  }
  if (!res.ok || body.code !== 0) {
    throw new AdminApiError(body.message || `请求失败 ${res.status}`, res.status);
  }
  return body.data as T;
}

function readSavedAuth(): AdminAuth | null {
  if (typeof window === "undefined") return null;
  try {
    const raw = window.localStorage.getItem(AUTH_KEY);
    return raw ? (JSON.parse(raw) as AdminAuth) : null;
  } catch {
    window.localStorage.removeItem(AUTH_KEY);
    return null;
  }
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
  const [overview, setOverview] = useState<AdminOverview | null>(null);
  const [papers, setPapers] = useState<PaperList>(() => emptyPaperList());
  const [filters, setFilters] = useState<{
    query: string;
    status: string;
    date_range: DateRangeFilter;
  }>({ query: "", status: "", date_range: "" });
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
    if (next) window.localStorage.setItem(AUTH_KEY, JSON.stringify(next));
    else window.localStorage.removeItem(AUTH_KEY);
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

  const loadOverview = useCallback(
    async (jwt = token) => {
      if (!jwt) return;
      const data = await adminRequest<AdminOverview>("/admin/overview", jwt);
      setOverview(data);
    },
    [token],
  );

  const loadPapers = useCallback(
    async (page = papers.page, jwt = token) => {
      if (!jwt) return;
      setTableLoading(true);
      try {
        const pageSize = DEFAULT_PAPER_PAGE_SIZE;
        const params = new URLSearchParams({
          page: String(page),
          page_size: String(pageSize),
        });
        if (filters.query.trim()) params.set("query", filters.query.trim());
        if (filters.status) params.set("status", filters.status);
        if (filters.date_range) params.set("date_range", filters.date_range);
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
        await Promise.all([loadOverview(jwt), loadPapers(1, jwt)]);
      } catch (err) {
        if (err instanceof AdminApiError && err.status === 401) {
          persistAuth(null);
        }
        flash("error", err instanceof Error ? err.message : "加载失败");
      } finally {
        setLoading(false);
      }
    },
    [flash, loadOverview, loadPapers, persistAuth, token],
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
    setOverview(null);
    setPapers(emptyPaperList());
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
      await Promise.all([loadOverview(), loadPapers(papers.page)]);
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
    <main className="min-h-dvh bg-background text-foreground">
      <div className="mx-auto flex min-h-dvh w-full max-w-[92rem] flex-col px-4 py-5 sm:px-6 lg:px-8">
        <header className="flex flex-wrap items-center justify-between gap-3 border-b border-border pb-4">
          <Link href="/" className="flex items-center gap-2 rounded-md outline-none focus-visible:ring-2 focus-visible:ring-ring">
            <span className="flex size-9 items-center justify-center rounded-lg bg-primary/10 text-primary">
              <ShieldCheck className="size-5" />
            </span>
            <span>
              <span className="block text-sm font-semibold tracking-tight">GopherPaper Admin</span>
              <span className="block text-xs text-muted-foreground">论文库运营控制台</span>
            </span>
          </Link>
          {auth ? (
            <div className="flex items-center gap-2">
              <span className="hidden text-sm text-muted-foreground sm:inline">
                {auth.admin.username}
              </span>
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
            <Link className="text-sm font-medium text-primary hover:underline" href="/">
              返回工作台
            </Link>
          )}
        </header>

        {message && (
          <button
            type="button"
            onClick={clearMessage}
            className={`mt-4 rounded-lg border px-3 py-2 text-left text-sm ${
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
            <OverviewGrid overview={overview} />
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
              <form className="flex flex-col gap-3 border-b border-border p-4 sm:flex-row sm:items-center" onSubmit={onSearch}>
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

function AuthPanel({
  mode,
  setMode,
  busy,
  codeBusy,
  loginForm,
  setLoginForm,
  registerForm,
  setRegisterForm,
  onSendCode,
  onLogin,
  onRegister,
}: {
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
}) {
  return (
    <section className="grid flex-1 place-items-center py-10">
      <div className="w-full max-w-md rounded-lg border border-border bg-card p-6 shadow-panel">
        <div className="mb-5">
          <p className="text-sm font-medium text-primary">Admin Console</p>
          <h1 className="mt-1 text-2xl font-semibold tracking-tight">
            {mode === "login" ? "管理员登录" : "注册管理员"}
          </h1>
          <p className="mt-2 text-sm leading-6 text-muted-foreground">
            管理员账号独立于普通用户账号，注册需要注册码和邮箱验证码。
          </p>
        </div>
        <div className="mb-5 grid grid-cols-2 gap-1 rounded-lg border border-border bg-muted p-1">
          <button
            type="button"
            className={`h-8 rounded-md text-sm font-medium ${mode === "login" ? "bg-card shadow-sm" : "text-muted-foreground"}`}
            onClick={() => setMode("login")}
          >
            登录
          </button>
          <button
            type="button"
            className={`h-8 rounded-md text-sm font-medium ${mode === "register" ? "bg-card shadow-sm" : "text-muted-foreground"}`}
            onClick={() => setMode("register")}
          >
            注册
          </button>
        </div>
        {mode === "login" ? (
          <form className="space-y-4" onSubmit={onLogin}>
            <Field label="邮箱">
              <Input
                type="email"
                value={loginForm.email}
                autoComplete="email"
                required
                onChange={(e) => setLoginForm((f) => ({ ...f, email: e.target.value }))}
              />
            </Field>
            <Field label="密码">
              <Input
                type="password"
                value={loginForm.password}
                autoComplete="current-password"
                required
                onChange={(e) => setLoginForm((f) => ({ ...f, password: e.target.value }))}
              />
            </Field>
            <Button className="h-10 w-full" type="submit" disabled={busy}>
              {busy && <Loader2 className="size-4 animate-spin" />}
              登录后台
            </Button>
          </form>
        ) : (
          <form className="space-y-4" onSubmit={onRegister}>
            <Field label="姓名">
              <Input
                value={registerForm.name}
                autoComplete="name"
                onChange={(e) => setRegisterForm((f) => ({ ...f, name: e.target.value }))}
              />
            </Field>
            <Field label="邮箱">
              <Input
                type="email"
                value={registerForm.email}
                autoComplete="email"
                required
                onChange={(e) => setRegisterForm((f) => ({ ...f, email: e.target.value }))}
              />
            </Field>
            <div className="space-y-2">
              <span className="text-sm font-medium">邮箱验证码</span>
              <div className="grid gap-2 sm:grid-cols-[1fr_auto]">
                <Input
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
                  type="button"
                  variant="outline"
                  disabled={busy || codeBusy || !registerForm.email.trim()}
                  onClick={() => void onSendCode()}
                >
                  {codeBusy && <Loader2 className="size-4 animate-spin" />}
                  发送验证码
                </Button>
              </div>
            </div>
            <Field label="用户名">
              <Input
                value={registerForm.username}
                autoComplete="username"
                required
                onChange={(e) => setRegisterForm((f) => ({ ...f, username: e.target.value }))}
              />
            </Field>
            <Field label="密码">
              <Input
                type="password"
                value={registerForm.password}
                autoComplete="new-password"
                minLength={6}
                required
                onChange={(e) => setRegisterForm((f) => ({ ...f, password: e.target.value }))}
              />
            </Field>
            <Field label="管理员注册码">
              <Input
                type="password"
                value={registerForm.registration_code}
                required
                onChange={(e) =>
                  setRegisterForm((f) => ({ ...f, registration_code: e.target.value }))
                }
              />
            </Field>
            <Button className="h-10 w-full" type="submit" disabled={busy}>
              {busy && <Loader2 className="size-4 animate-spin" />}
              创建管理员
            </Button>
          </form>
        )}
      </div>
    </section>
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

function OverviewGrid({ overview }: { overview: AdminOverview | null }) {
  const stats = [
    {
      label: "论文库数量",
      value: overview ? formatNumber(overview.paper_count) : "—",
      note: "当前全库论文",
      icon: FileSearch,
    },
    {
      label: "服务调用次数",
      value: overview ? formatNumber(overview.service_call_count) : "—",
      note: `${overview?.service_success ?? 0} 成功 / ${overview?.service_failed ?? 0} 失败`,
      icon: BarChart3,
    },
    {
      label: "服务成功率",
      value: overview ? formatPercent(overview.service_success_rate) : "—",
      note: "核心业务调用",
      icon: CheckCircle2,
    },
    {
      label: "解析成功率",
      value: overview ? formatPercent(overview.parse_success_rate) : "—",
      note: `${overview?.parse_ready ?? 0} ready / ${overview?.parse_failed ?? 0} failed`,
      icon: FileCheck2,
    },
    {
      label: "向量数据库条数",
      value: overview?.vector_count == null ? "—" : formatNumber(overview.vector_count),
      note: overview?.vector_error ? "Milvus 暂不可用" : "可查询有效 chunks",
      icon: Database,
    },
  ];
  return (
    <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-5">
      {stats.map((item) => {
        const Icon = item.icon;
        return (
          <article key={item.label} className="rounded-lg border border-border bg-card p-4 shadow-sm">
            <div className="flex items-center justify-between gap-3">
              <p className="text-sm text-muted-foreground">{item.label}</p>
              <Icon className="size-4 text-primary" />
            </div>
            <strong className="mt-3 block text-2xl font-semibold tracking-tight">{item.value}</strong>
            <span className="mt-1 block text-xs text-muted-foreground">{item.note}</span>
          </article>
        );
      })}
    </div>
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

function formatNumber(value: number) {
  return new Intl.NumberFormat("zh-CN").format(value);
}

function formatPercent(value: number | null) {
  if (value == null) return "—";
  return `${(value * 100).toFixed(1)}%`;
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
