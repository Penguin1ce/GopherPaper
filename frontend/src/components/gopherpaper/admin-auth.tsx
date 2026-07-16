"use client";

// 管理员登录/注册卡片 — 首页 / 与 /admin 控制台共用,避免两套会走样的登录 UI。
// 自带内联提示与验证码状态，登录态由后端 HttpOnly Cookie 承载。

import { CheckCircle2, Loader2 } from "lucide-react";
import { AnimatePresence, motion } from "motion/react";
import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type Dispatch,
  type FormEvent,
  type ReactNode,
  type SetStateAction,
} from "react";

import { GlassSurface } from "@/components/reactbits/glass-surface";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { PasswordInput } from "@/components/ui/password-input";

const API_BASE = process.env.NEXT_PUBLIC_API_BASE || "/api/v1";
export const ADMIN_AUTH_KEY = "gopherpaper.admin.auth";
const EASE = [0.16, 1, 0.3, 1] as const;

type Mode = "login" | "register";

export type AdminProfile = {
  id: number;
  username: string;
  email: string;
  name: string;
};

export type AdminAuth = {
  admin: AdminProfile;
};

type AdminLoginResponse = AdminAuth;

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

export class AdminApiError extends Error {
  status: number;

  constructor(message: string, status: number) {
    super(message);
    this.status = status;
  }
}

export async function adminRequest<T>(
  path: string,
  _token: string,
  options: RequestInit = {},
): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    ...options,
    credentials: "same-origin",
    headers: {
      "Content-Type": "application/json",
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
    throw new AdminApiError(
      body.message || `请求失败 ${res.status}`,
      res.status,
    );
  }
  return body.data as T;
}

export function readSavedAuth(): AdminAuth | null {
  if (typeof window === "undefined") return null;
  try {
    const raw = window.localStorage.getItem(ADMIN_AUTH_KEY);
    if (!raw) return null;
    const saved = JSON.parse(raw) as Partial<AdminAuth> & { token?: string };
    if (!saved.admin) {
      window.localStorage.removeItem(ADMIN_AUTH_KEY);
      return null;
    }
    const sanitized: AdminAuth = { admin: saved.admin };
    window.localStorage.setItem(ADMIN_AUTH_KEY, JSON.stringify(sanitized));
    return sanitized;
  } catch {
    window.localStorage.removeItem(ADMIN_AUTH_KEY);
    return null;
  }
}

export function AdminAuthCard({
  onAuthed,
}: {
  onAuthed: (auth: AdminAuth) => void;
}) {
  const [mode, setMode] = useState<Mode>("login");
  const [loginForm, setLoginForm] = useState({ email: "", password: "" });
  const [registerForm, setRegisterForm] = useState<AdminRegisterForm>({
    username: "",
    email: "",
    name: "",
    password: "",
    code: "",
    registration_code: "",
  });
  const [busy, setBusy] = useState(false);
  const [codeBusy, setCodeBusy] = useState(false);
  const [message, setMessage] = useState<{
    type: "ok" | "error";
    text: string;
  } | null>(null);
  const [codeNotice, setCodeNotice] = useState<string | null>(null);
  const messageTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const codeNoticeTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const flash = useCallback((type: "ok" | "error", text: string) => {
    if (messageTimerRef.current) clearTimeout(messageTimerRef.current);
    setMessage({ type, text });
    messageTimerRef.current = setTimeout(() => {
      setMessage(null);
      messageTimerRef.current = null;
    }, 3200);
  }, []);

  const showCodeNotice = useCallback((text: string) => {
    if (codeNoticeTimerRef.current) clearTimeout(codeNoticeTimerRef.current);
    setCodeNotice(text);
    codeNoticeTimerRef.current = setTimeout(() => {
      setCodeNotice(null);
      codeNoticeTimerRef.current = null;
    }, 2400);
  }, []);

  useEffect(() => {
    return () => {
      if (messageTimerRef.current) clearTimeout(messageTimerRef.current);
      if (codeNoticeTimerRef.current) clearTimeout(codeNoticeTimerRef.current);
    };
  }, []);

  const onLogin = async (e: FormEvent) => {
    e.preventDefault();
    if (busy) return;
    setBusy(true);
    try {
      const data = await adminRequest<AdminLoginResponse>("/admin/login", "", {
        method: "POST",
        body: JSON.stringify(loginForm),
      });
      onAuthed({ admin: data.admin });
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
      setLoginForm({
        email: registerForm.email,
        password: registerForm.password,
      });
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

  const isLogin = mode === "login";

  return (
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

      <AnimatePresence initial={false}>
        {message && (
          <motion.button
            type="button"
            onClick={() => setMessage(null)}
            initial={{ opacity: 0, y: -6 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -6 }}
            transition={{ duration: 0.2, ease: EASE }}
            className={`mb-4 block w-full rounded-lg border px-3 py-2 text-left text-sm ${
              message.type === "ok"
                ? "border-emerald-200 bg-emerald-50 text-emerald-800"
                : "border-red-200 bg-red-50 text-red-800"
            }`}
          >
            {message.text}
          </motion.button>
        )}
      </AnimatePresence>

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
            transition={{ duration: 0.24, ease: EASE }}
          >
            <AdminLoginForm
              busy={busy}
              loginForm={loginForm}
              setLoginForm={setLoginForm}
              onLogin={onLogin}
            />
          </motion.div>
        ) : (
          <motion.div
            key="admin-register"
            initial={{ opacity: 0, x: 10 }}
            animate={{ opacity: 1, x: 0 }}
            exit={{ opacity: 0, x: 10 }}
            transition={{ duration: 0.24, ease: EASE }}
          >
            <AdminRegisterFormPanel
              busy={busy}
              codeBusy={codeBusy}
              registerForm={registerForm}
              setRegisterForm={setRegisterForm}
              onSendCode={sendAdminCode}
              onRegister={onRegister}
            />
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

      <AnimatePresence>
        {codeNotice && (
          <motion.div
            role="status"
            initial={{ opacity: 0, y: 6 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: 6 }}
            transition={{ duration: 0.2, ease: EASE }}
            className="mt-4 flex items-center gap-2 rounded-lg border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm font-medium text-emerald-800"
          >
            <CheckCircle2 className="size-4" />
            {codeNotice}
          </motion.div>
        )}
      </AnimatePresence>
    </GlassSurface>
  );
}

type AdminLoginFormProps = {
  busy: boolean;
  loginForm: { email: string; password: string };
  setLoginForm: Dispatch<SetStateAction<{ email: string; password: string }>>;
  onLogin: (e: FormEvent) => void;
};

function AdminLoginForm({
  busy,
  loginForm,
  setLoginForm,
  onLogin,
}: AdminLoginFormProps) {
  return (
    <form className="space-y-4" onSubmit={onLogin}>
      <Field label="邮箱">
        <Input
          className="h-10"
          type="email"
          value={loginForm.email}
          autoComplete="email"
          required
          onChange={(e) =>
            setLoginForm((f) => ({ ...f, email: e.target.value }))
          }
        />
      </Field>
      <Field label="密码">
        <PasswordInput
          className="h-10"
          value={loginForm.password}
          autoComplete="current-password"
          required
          onChange={(e) =>
            setLoginForm((f) => ({ ...f, password: e.target.value }))
          }
        />
      </Field>
      <Button className="h-11 w-full gap-2" type="submit" disabled={busy}>
        {busy && <Loader2 className="size-4 animate-spin" />}
        登录后台
      </Button>
    </form>
  );
}

type AdminRegisterFormProps = {
  busy: boolean;
  codeBusy: boolean;
  registerForm: AdminRegisterForm;
  setRegisterForm: Dispatch<SetStateAction<AdminRegisterForm>>;
  onSendCode: () => void;
  onRegister: (e: FormEvent) => void;
};

function AdminRegisterFormPanel({
  busy,
  codeBusy,
  registerForm,
  setRegisterForm,
  onSendCode,
  onRegister,
}: AdminRegisterFormProps) {
  return (
    <form className="space-y-4" onSubmit={onRegister}>
      <div className="grid gap-4 sm:grid-cols-2">
        <Field label="姓名">
          <Input
            className="h-10"
            value={registerForm.name}
            autoComplete="name"
            onChange={(e) =>
              setRegisterForm((f) => ({ ...f, name: e.target.value }))
            }
          />
        </Field>
        <Field label="用户名">
          <Input
            className="h-10"
            value={registerForm.username}
            autoComplete="username"
            required
            onChange={(e) =>
              setRegisterForm((f) => ({ ...f, username: e.target.value }))
            }
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
          onChange={(e) =>
            setRegisterForm((f) => ({ ...f, email: e.target.value }))
          }
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
            onChange={(e) =>
              setRegisterForm((f) => ({ ...f, password: e.target.value }))
            }
          />
        </Field>
        <Field label="管理员注册码">
          <PasswordInput
            className="h-10"
            value={registerForm.registration_code}
            required
            onChange={(e) =>
              setRegisterForm((f) => ({
                ...f,
                registration_code: e.target.value,
              }))
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
