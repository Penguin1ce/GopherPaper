"use client";

import {
  ArrowLeft,
  ArrowRight,
  FileText,
  Layers,
  Loader2,
  MessageSquareText,
  Quote,
  Upload,
} from "lucide-react";
import {
  AnimatePresence,
  motion,
  useMotionValue,
  useReducedMotion,
  useSpring,
} from "motion/react";
import { useEffect, useRef, useState } from "react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useApp } from "@/lib/gopherpaper/store";
import { useGuard } from "./app-ui";

type AuthMode = "landing" | "login" | "register";

const EASE_OUT = [0.16, 1, 0.3, 1] as const;

const PIPELINE = [
  { key: "upload", label: "上传", icon: Upload },
  { key: "parse", label: "解析", icon: FileText },
  { key: "extract", label: "抽取", icon: Layers },
  { key: "answer", label: "问答", icon: MessageSquareText },
] as const;

const STEP_INTERVAL = 1500;
const STEP_HOLD = 2400;

const AGENT_NODES = [
  {
    name: "小云鸮",
    output: "证据链.md",
    icon: Quote,
    className: "left-[18.25rem] top-5",
    delay: 0,
  },
  {
    name: "小云雀",
    output: "工具结果.json",
    icon: MessageSquareText,
    className: "left-[19.15rem] top-32",
    delay: 0.35,
  },
  {
    name: "小囊鼠",
    output: "研读报告.pdf",
    icon: FileText,
    className: "left-[18.65rem] top-[15rem]",
    delay: 0.7,
  },
] as const;

/* 磁吸 — 元素向光标轻微靠拢,motion value 走在 render 之外 */
function Magnetic({
  children,
  strength = 0.3,
  className,
}: {
  children: React.ReactNode;
  strength?: number;
  className?: string;
}) {
  const reduce = useReducedMotion();
  const ref = useRef<HTMLDivElement>(null);
  const x = useMotionValue(0);
  const y = useMotionValue(0);
  const sx = useSpring(x, { stiffness: 220, damping: 18, mass: 0.4 });
  const sy = useSpring(y, { stiffness: 220, damping: 18, mass: 0.4 });

  if (reduce) return <div className={className}>{children}</div>;

  return (
    <motion.div
      ref={ref}
      className={className}
      style={{ x: sx, y: sy }}
      onMouseMove={(e) => {
        const r = ref.current?.getBoundingClientRect();
        if (!r) return;
        x.set((e.clientX - (r.left + r.width / 2)) * strength);
        y.set((e.clientY - (r.top + r.height / 2)) * strength);
      }}
      onMouseLeave={() => {
        x.set(0);
        y.set(0);
      }}
    >
      {children}
    </motion.div>
  );
}

/* 视差倾斜 — 卡片随光标做轻微 3D 偏转,克制在 ±6deg */
function Tilt({ children, className }: { children: React.ReactNode; className?: string }) {
  const reduce = useReducedMotion();
  const ref = useRef<HTMLDivElement>(null);
  const rx = useMotionValue(0);
  const ry = useMotionValue(0);
  const srx = useSpring(rx, { stiffness: 150, damping: 18, mass: 0.5 });
  const sry = useSpring(ry, { stiffness: 150, damping: 18, mass: 0.5 });

  if (reduce) return <div className={className}>{children}</div>;

  return (
    <motion.div
      ref={ref}
      className={className}
      style={{ rotateX: srx, rotateY: sry, transformPerspective: 1100, transformStyle: "preserve-3d" }}
      onMouseMove={(e) => {
        const r = ref.current?.getBoundingClientRect();
        if (!r) return;
        const px = (e.clientX - r.left) / r.width - 0.5;
        const py = (e.clientY - r.top) / r.height - 0.5;
        ry.set(px * 6);
        rx.set(-py * 6);
      }}
      onMouseLeave={() => {
        rx.set(0);
        ry.set(0);
      }}
    >
      {children}
    </motion.div>
  );
}

export function AuthView() {
  const { login, registerAndLogin, sendCode } = useApp();
  const guard = useGuard();
  const [mode, setMode] = useState<AuthMode>("landing");
  const [busy, setBusy] = useState(false);
  const [codeBusy, setCodeBusy] = useState(false);
  const [loginForm, setLoginForm] = useState({ student_id: "", password: "" });
  const [reg, setReg] = useState({
    student_id: "",
    name: "",
    email: "",
    class_id: "",
    password: "",
    code: "",
  });

  const onLogin = (e: React.FormEvent) => {
    e.preventDefault();
    if (busy) return;
    setBusy(true);
    guard(() => login(loginForm.student_id.trim(), loginForm.password)).finally(
      () => setBusy(false),
    );
  };

  const onRegister = (e: React.FormEvent) => {
    e.preventDefault();
    if (busy) return;
    setBusy(true);
    guard(() =>
      registerAndLogin({
        student_id: reg.student_id.trim(),
        name: reg.name.trim(),
        email: reg.email.trim(),
        class_id: reg.class_id.trim(),
        password: reg.password,
        code: reg.code.trim(),
      }),
    ).finally(() => setBusy(false));
  };

  const onSendCode = () => {
    if (!reg.email.trim()) return;
    setCodeBusy(true);
    guard(() => sendCode(reg.email.trim())).finally(() =>
      window.setTimeout(() => setCodeBusy(false), 900),
    );
  };

  return (
    <main className="relative isolate min-h-dvh overflow-hidden bg-background">
      <Backdrop />

      <div className="mx-auto flex min-h-dvh w-full max-w-[78rem] flex-col px-5 sm:px-8">
        <TopNav mode={mode} setMode={setMode} />

        <AnimatePresence mode="wait">
          {mode === "landing" ? (
            <Landing key="landing" onStart={() => setMode("login")} onRegister={() => setMode("register")} />
          ) : (
            <AuthPanel
              key="auth"
              mode={mode}
              setMode={setMode}
              busy={busy}
              codeBusy={codeBusy}
              loginForm={loginForm}
              setLoginForm={setLoginForm}
              reg={reg}
              setReg={setReg}
              onLogin={onLogin}
              onRegister={onRegister}
              onSendCode={onSendCode}
            />
          )}
        </AnimatePresence>
      </div>
    </main>
  );
}

/* ---------------- 背景层 — 纸感晕染 + 学术格线 ---------------- */

function Backdrop() {
  return (
    <div aria-hidden className="pointer-events-none absolute inset-0 -z-10 overflow-hidden">
      {/* 墨蓝 + 赭石的柔光晕,定调而不抢戏 */}
      <div className="absolute -top-[28%] left-1/2 h-[42rem] w-[58rem] -translate-x-1/2 rounded-full bg-primary/[0.07] blur-3xl" />
      <div className="absolute -right-[14%] top-[6%] h-[34rem] w-[34rem] rounded-full bg-sienna/[0.08] blur-3xl" />
      {/* 论文稿纸基线,极淡 */}
      <div
        className="absolute inset-0 opacity-[0.5]"
        style={{
          backgroundImage:
            "linear-gradient(to right, color-mix(in oklch, var(--border) 60%, transparent) 1px, transparent 1px)",
          backgroundSize: "min(7.5rem, 12vw) 100%",
          maskImage: "radial-gradient(120% 80% at 50% 0%, #000 30%, transparent 78%)",
        }}
      />
      <div className="absolute inset-x-0 top-0 h-px bg-gradient-to-r from-transparent via-sienna/40 to-transparent" />
    </div>
  );
}

/* ---------------- 顶栏 ---------------- */

function BrandMark({ onClick }: { onClick?: () => void }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="group flex items-center gap-2.5 rounded-md outline-none focus-visible:ring-2 focus-visible:ring-ring/60"
    >
      <span className="flex size-9 items-center justify-center rounded-lg bg-primary font-serif text-lg font-semibold text-primary-foreground shadow-sm transition-transform group-active:scale-95">
        G
      </span>
      <span className="font-serif text-lg font-semibold tracking-tight">GopherPaper</span>
    </button>
  );
}

function TopNav({ mode, setMode }: { mode: AuthMode; setMode: (m: AuthMode) => void }) {
  return (
    <motion.header
      initial={{ opacity: 0, y: -12 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.6, ease: EASE_OUT }}
      className="flex items-center justify-between py-6"
    >
      <BrandMark onClick={() => setMode("landing")} />
      {mode === "landing" ? (
        <nav className="flex items-center gap-1.5 sm:gap-2">
          <Button variant="ghost" size="sm" onClick={() => setMode("login")}>
            登录
          </Button>
          <Button size="sm" onClick={() => setMode("register")}>
            注册
          </Button>
        </nav>
      ) : (
        <Button variant="ghost" size="sm" onClick={() => setMode("landing")} className="gap-1.5">
          <ArrowLeft className="size-4" />
          返回
        </Button>
      )}
    </motion.header>
  );
}

/* ---------------- 落地页 ---------------- */

function Landing({ onStart, onRegister }: { onStart: () => void; onRegister: () => void }) {
  const reduce = useReducedMotion();
  const fade = (delay: number) =>
    reduce
      ? { initial: { opacity: 0 }, animate: { opacity: 1 }, transition: { duration: 0.3, delay } }
      : {
          initial: { opacity: 0, y: 18 },
          animate: { opacity: 1, y: 0 },
          transition: { duration: 0.7, ease: EASE_OUT, delay },
        };

  return (
    <motion.section
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      exit={{ opacity: 0, transition: { duration: 0.25 } }}
      className="grid flex-1 items-center gap-10 py-8 lg:grid-cols-[0.92fr_1.08fr] lg:gap-16 lg:py-4"
    >
      <div className="max-w-xl">
        <h1 className="sr-only">GopherPaper 科研文献智能解析与问答</h1>
        <motion.div {...fade(0.05)}>
          <ResearchSignal />
        </motion.div>

        <motion.div {...fade(0.22)} className="mt-8 flex flex-wrap items-center gap-3">
          <Magnetic strength={0.35}>
            <Button size="lg" onClick={onStart} className="group h-12 gap-2 px-6 text-base">
              登录进入工作台
              <ArrowRight className="size-4 transition-transform group-hover:translate-x-0.5" />
            </Button>
          </Magnetic>
          <Button
            size="lg"
            variant="outline"
            onClick={onRegister}
            className="h-12 px-6 text-base"
          >
            创建账号
          </Button>
        </motion.div>
      </div>

      {/* 产品演示 console */}
      <motion.div
        initial={reduce ? { opacity: 0 } : { opacity: 0, y: 26, scale: 0.97 }}
        animate={{ opacity: 1, y: 0, scale: 1 }}
        transition={
          reduce
            ? { duration: 0.4, delay: 0.1 }
            : { type: "spring", stiffness: 90, damping: 18, mass: 0.8, delay: 0.18 }
        }
        className="lg:justify-self-end"
      >
        <Tilt>
          <Console />
        </Tilt>
      </motion.div>
    </motion.section>
  );
}

function ResearchSignal() {
  const reduce = useReducedMotion();
  const drift = reduce
    ? {}
    : {
        y: [0, -8, 0],
        rotate: [-0.6, 0.8, -0.6],
      };
  const pulse = reduce
    ? {}
    : {
        scale: [1, 1.08, 1],
        opacity: [0.72, 1, 0.72],
      };

  return (
    <div aria-hidden className="relative h-[23rem] w-full max-w-[36rem]">
      <motion.div
        className="absolute left-2 top-14 h-56 w-40 rounded-xl border border-border bg-card p-4 shadow-panel"
        animate={drift}
        transition={{ duration: 7, repeat: Infinity, ease: "easeInOut" }}
      >
        <div className="mb-4 flex items-center gap-2">
          <span className="flex size-7 items-center justify-center rounded-md bg-primary/10 text-primary">
            <FileText className="size-4" />
          </span>
          <span className="h-2 w-16 rounded-full bg-foreground/15" />
        </div>
        <div className="space-y-2.5">
          <span className="block h-2 w-full rounded-full bg-muted" />
          <span className="block h-2 w-[88%] rounded-full bg-muted" />
          <span className="block h-2 w-[72%] rounded-full bg-muted" />
          <span className="block h-2 w-[94%] rounded-full bg-muted" />
          <span className="block h-2 w-[64%] rounded-full bg-muted" />
        </div>
        <div className="mt-5 grid grid-cols-3 gap-2">
          <span className="h-9 rounded-md bg-sienna/10" />
          <span className="h-9 rounded-md bg-primary/10" />
          <span className="h-9 rounded-md bg-muted" />
        </div>
        {!reduce && (
          <motion.span
            className="absolute inset-x-4 top-5 h-px bg-primary/80 shadow-[0_0_20px_oklch(0.48_0.1_200/0.42)]"
            animate={{ y: [0, 176, 0], opacity: [0, 1, 0] }}
            transition={{ duration: 3.2, repeat: Infinity, ease: "easeInOut" }}
          />
        )}
      </motion.div>

      <motion.div
        className="absolute left-28 top-4 h-44 w-32 rounded-xl border border-border/80 bg-card/75 p-3 shadow-sm backdrop-blur"
        animate={reduce ? {} : { y: [0, 7, 0], rotate: [1.5, -0.4, 1.5] }}
        transition={{ duration: 8, repeat: Infinity, ease: "easeInOut" }}
      >
        <span className="mb-3 block h-16 rounded-lg bg-muted" />
        <span className="mb-2 block h-1.5 w-full rounded-full bg-foreground/15" />
        <span className="mb-2 block h-1.5 w-[78%] rounded-full bg-foreground/15" />
        <span className="block h-1.5 w-[52%] rounded-full bg-foreground/15" />
      </motion.div>

      <svg className="absolute inset-0 h-full w-full" viewBox="0 0 540 352" fill="none">
        {[
          "M154 164 C218 92 262 102 312 146",
          "M178 214 C230 228 270 218 326 190",
          "M306 164 C370 112 410 112 482 132",
          "M310 188 C372 230 418 240 492 218",
        ].map((d, i) => (
          <motion.path
            key={d}
            d={d}
            stroke="currentColor"
            className="text-primary/35"
            strokeWidth="1.5"
            strokeLinecap="round"
            strokeDasharray="4 8"
            animate={reduce ? {} : { strokeDashoffset: [0, -48] }}
            transition={{ duration: 2.8 + i * 0.35, repeat: Infinity, ease: "linear" }}
          />
        ))}
      </svg>

      <motion.div
        className="absolute left-[39%] top-[43%] flex size-28 -translate-x-1/2 -translate-y-1/2 items-center justify-center rounded-2xl border border-primary/30 bg-card shadow-[0_22px_60px_-24px_oklch(0.21_0.02_230/0.35)]"
        animate={pulse}
        transition={{ duration: 3.6, repeat: Infinity, ease: "easeInOut" }}
      >
        <span className="absolute size-40 rounded-full border border-primary/10" />
        <span className="absolute size-28 rounded-full border border-primary/15" />
        <span className="flex size-14 items-center justify-center rounded-xl bg-primary text-primary-foreground shadow-sm">
          <Layers className="size-6" />
        </span>
      </motion.div>

      {AGENT_NODES.map((agent, i) => {
        const Icon = agent.icon;
        return (
          <motion.div
            key={agent.name}
            className={`absolute z-10 w-[8.75rem] rounded-xl border border-border bg-card/90 px-3 py-2.5 shadow-sm backdrop-blur ${agent.className}`}
            animate={reduce ? {} : { x: [0, i % 2 === 0 ? 6 : -5, 0], y: [0, i === 1 ? 5 : -4, 0] }}
            transition={{ duration: 5 + i, repeat: Infinity, ease: "easeInOut", delay: agent.delay }}
          >
            <motion.span
              className="absolute -left-10 -top-4 flex items-center gap-1 rounded-full border border-primary/20 bg-background/90 px-2 py-1 text-[10px] font-medium text-primary shadow-sm"
              animate={reduce ? {} : { rotate: [0, 2, 0], y: [0, -3, 0] }}
              transition={{ duration: 4.2 + i * 0.3, repeat: Infinity, ease: "easeInOut", delay: agent.delay }}
            >
              Tools
              <span className="size-1 rounded-full bg-primary/60" />
            </motion.span>
            <motion.span
              className="absolute left-[4.35rem] -top-4 rounded-full border border-sienna/20 bg-background/90 px-2 py-1 text-[10px] font-medium text-sienna shadow-sm"
              animate={reduce ? {} : { x: [0, 3, 0] }}
              transition={{ duration: 4.8 + i * 0.2, repeat: Infinity, ease: "easeInOut", delay: agent.delay + 0.2 }}
            >
              Skills
            </motion.span>
            <motion.span
              className="absolute -bottom-4 left-10 rounded-full border border-border bg-background/95 px-2 py-1 text-[10px] font-medium text-muted-foreground shadow-sm"
              animate={reduce ? {} : { y: [0, 3, 0] }}
              transition={{ duration: 5 + i * 0.25, repeat: Infinity, ease: "easeInOut", delay: agent.delay + 0.4 }}
            >
              MCP
            </motion.span>
            <motion.span
              className="absolute left-[8.2rem] top-1 grid w-[6.75rem] gap-1 rounded-lg border border-primary/15 bg-background/95 px-2 py-1.5 text-[10px] shadow-sm backdrop-blur"
              animate={reduce ? {} : { x: [0, 4, 0], opacity: [0.86, 1, 0.86] }}
              transition={{ duration: 4.6 + i * 0.25, repeat: Infinity, ease: "easeInOut", delay: agent.delay + 0.5 }}
            >
              <span className="absolute -left-3 top-1/2 h-px w-3 bg-primary/30" />
              <span className="font-medium text-muted-foreground">产出</span>
              <span className="truncate font-mono text-[11px] text-foreground">
                {agent.output}
              </span>
            </motion.span>

            <span className="flex items-center gap-2">
              <span className="flex size-7 shrink-0 items-center justify-center rounded-lg bg-sienna/10 text-sienna">
                <Icon className="size-3.5" />
              </span>
              <span className="min-w-0">
                <span className="block truncate text-sm font-semibold tracking-tight text-foreground">
                  {agent.name}
                </span>
                <span className="mt-1 block h-1.5 w-16 rounded-full bg-foreground/12" />
              </span>
            </span>
          </motion.div>
        );
      })}

      {!reduce && (
        <>
          {[0, 1, 2, 3, 4].map((i) => (
            <motion.span
              key={i}
              className="absolute size-1.5 rounded-full bg-primary"
              style={{
                left: `${34 + i * 9}%`,
                top: `${35 + (i % 3) * 12}%`,
              }}
              animate={{ opacity: [0, 1, 0], scale: [0.5, 1.3, 0.5] }}
              transition={{ duration: 2.4, repeat: Infinity, delay: i * 0.36, ease: "easeInOut" }}
            />
          ))}
        </>
      )}
    </div>
  );
}

/* ---------------- 拟真工作台演示 ---------------- */

function Console() {
  const reduce = useReducedMotion();
  const [step, setStep] = useState(reduce ? PIPELINE.length - 1 : 0);

  useEffect(() => {
    if (reduce) {
      setStep(PIPELINE.length - 1);
      return;
    }
    const last = PIPELINE.length - 1;
    const delay = step >= last ? STEP_HOLD : STEP_INTERVAL;
    const t = window.setTimeout(
      () => setStep((s) => (s >= last ? 0 : s + 1)),
      delay,
    );
    return () => window.clearTimeout(t);
  }, [step, reduce]);

  const answered = step >= PIPELINE.length - 1;

  return (
    <div className="relative w-full max-w-[26rem]">
      {/* 背后浮起的薄卡,做层次 */}
      <div
        aria-hidden
        className="absolute -right-3 -top-3 hidden h-full w-full rounded-2xl border border-border/70 bg-card/40 sm:block"
      />

      <div className="relative overflow-hidden rounded-2xl border border-border bg-card shadow-[0_24px_70px_-24px_oklch(0.255_0.012_56/0.35)]">
        {/* 流水线进度条 */}
        <div className="flex items-center gap-1.5 border-b border-border/70 bg-muted/40 px-5 py-3.5">
          {PIPELINE.map((p, i) => {
            const Icon = p.icon;
            const active = i === step;
            const done = i < step;
            return (
              <div key={p.key} className="flex flex-1 items-center gap-1.5">
                <div className="flex items-center gap-1.5">
                  <span
                    className={`relative flex size-6 items-center justify-center rounded-full border transition-colors duration-500 ${
                      done || active
                        ? "border-primary bg-primary text-primary-foreground"
                        : "border-border bg-card text-muted-foreground"
                    }`}
                  >
                    <Icon className="size-3" />
                    {active && !reduce && (
                      <motion.span
                        className="absolute inset-0 rounded-full ring-2 ring-primary/40"
                        animate={{ scale: [1, 1.5], opacity: [0.6, 0] }}
                        transition={{ duration: 1.4, repeat: Infinity, ease: "easeOut" }}
                      />
                    )}
                  </span>
                  <span
                    className={`text-[11px] font-medium transition-colors duration-500 ${
                      done || active ? "text-foreground" : "text-muted-foreground"
                    }`}
                  >
                    {p.label}
                  </span>
                </div>
                {i < PIPELINE.length - 1 && (
                  <span className="relative h-px flex-1 overflow-hidden bg-border">
                    <motion.span
                      className="absolute inset-0 bg-primary"
                      initial={false}
                      animate={{ scaleX: done ? 1 : 0 }}
                      style={{ transformOrigin: "left" }}
                      transition={{ duration: 0.5, ease: EASE_OUT }}
                    />
                  </span>
                )}
              </div>
            );
          })}
        </div>

        {/* 论文卡 */}
        <div className="space-y-3 px-5 pb-4 pt-5">
          <h3 className="font-serif text-[15px] font-semibold leading-snug text-foreground">
            L-茶氨酸的认知效应：人类临床试验的系统综述
          </h3>
          <div className="space-y-2">
            <span className="block h-2 w-full rounded-full bg-muted" />
            <span className="block h-2 w-[92%] rounded-full bg-muted" />
            <span className="block h-2 w-[78%] rounded-full bg-muted" />
          </div>
          <div className="flex flex-wrap gap-1.5 pt-0.5">
            {["n = 150", "p. 4", "图 2", "双盲 RCT"].map((c) => (
              <span
                key={c}
                className="rounded-md border border-sienna/30 bg-sienna/[0.07] px-2 py-0.5 text-[11px] font-medium text-sienna tabular-nums"
              >
                {c}
              </span>
            ))}
          </div>
        </div>

        {/* 证据问答 */}
        <div className="space-y-2.5 border-t border-border/70 bg-muted/30 px-5 py-4">
          <div className="flex justify-end">
            <span className="max-w-[80%] rounded-2xl rounded-br-sm bg-primary px-3 py-1.5 text-[12.5px] text-primary-foreground">
              这篇论文用了什么实验方法？
            </span>
          </div>

          <AnimatePresence mode="wait">
            {answered ? (
              <motion.div
                key="reply"
                initial={{ opacity: 0, y: 6 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ duration: 0.4, ease: EASE_OUT }}
                className="max-w-[88%] space-y-2 rounded-2xl rounded-bl-sm border border-border bg-card px-3 py-2.5"
              >
                <p className="text-[12.5px] leading-relaxed text-foreground">
                  采用双盲随机对照设计，受试者被分配至 L-茶氨酸或安慰剂组，以注意力与压力相关指标作为主要结局。
                </p>
                <div className="flex items-center gap-1.5 border-t border-border/70 pt-2 text-[11px] text-sienna">
                  <Quote className="size-3" />
                  来源 §3.2 方法 · 第 7 页
                </div>
              </motion.div>
            ) : (
              <motion.div
                key="typing"
                exit={{ opacity: 0 }}
                className="flex w-fit items-center gap-1 rounded-2xl rounded-bl-sm border border-border bg-card px-3 py-2.5"
              >
                {[0, 1, 2].map((i) => (
                  <motion.span
                    key={i}
                    className="size-1.5 rounded-full bg-muted-foreground/60"
                    animate={reduce ? {} : { opacity: [0.3, 1, 0.3] }}
                    transition={{ duration: 1, repeat: Infinity, delay: i * 0.18 }}
                  />
                ))}
              </motion.div>
            )}
          </AnimatePresence>
        </div>
      </div>
    </div>
  );
}

/* ---------------- 登录 / 注册面板 ---------------- */

type AuthPanelProps = {
  mode: AuthMode;
  setMode: (m: AuthMode) => void;
  busy: boolean;
  codeBusy: boolean;
  loginForm: { student_id: string; password: string };
  setLoginForm: React.Dispatch<React.SetStateAction<{ student_id: string; password: string }>>;
  reg: {
    student_id: string;
    name: string;
    email: string;
    class_id: string;
    password: string;
    code: string;
  };
  setReg: React.Dispatch<
    React.SetStateAction<{
      student_id: string;
      name: string;
      email: string;
      class_id: string;
      password: string;
      code: string;
    }>
  >;
  onLogin: (e: React.FormEvent) => void;
  onRegister: (e: React.FormEvent) => void;
  onSendCode: () => void;
};

function AuthPanel(props: AuthPanelProps) {
  const { mode, setMode } = props;
  const isLogin = mode === "login";

  return (
    <motion.section
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      exit={{ opacity: 0, transition: { duration: 0.2 } }}
      className="grid flex-1 place-items-center py-8"
    >
      <motion.div
        initial={{ opacity: 0, y: 16 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.55, ease: EASE_OUT }}
        className="w-full max-w-md rounded-2xl border border-border bg-card p-7 shadow-[0_24px_70px_-30px_oklch(0.255_0.012_56/0.4)] sm:p-8"
      >
        <div className="mb-6">
          <h1 className="font-serif text-[1.7rem] font-semibold tracking-tight">
            {isLogin ? "进入工作台" : "创建账号"}
          </h1>
          <p className="mt-1.5 text-sm text-muted-foreground">
            {isLogin ? "使用学号登录,继续你的论文研读。" : "用邮箱验证码注册,开启你的研究工作台。"}
          </p>
        </div>

        <div className="mb-6 grid grid-cols-2 gap-1 rounded-lg border border-border bg-muted/50 p-1">
          {(["login", "register"] as const).map((m) => (
            <button
              key={m}
              type="button"
              onClick={() => setMode(m)}
              className={`h-9 rounded-md text-sm font-medium transition-colors ${
                mode === m
                  ? "bg-card text-foreground shadow-sm"
                  : "text-muted-foreground hover:text-foreground"
              }`}
            >
              {m === "login" ? "登录" : "注册"}
            </button>
          ))}
        </div>

        <AnimatePresence mode="wait">
          {isLogin ? (
            <motion.div
              key="login"
              initial={{ opacity: 0, x: -8 }}
              animate={{ opacity: 1, x: 0 }}
              exit={{ opacity: 0, x: -8 }}
              transition={{ duration: 0.25, ease: EASE_OUT }}
            >
              <LoginForm {...props} />
            </motion.div>
          ) : (
            <motion.div
              key="register"
              initial={{ opacity: 0, x: 8 }}
              animate={{ opacity: 1, x: 0 }}
              exit={{ opacity: 0, x: 8 }}
              transition={{ duration: 0.25, ease: EASE_OUT }}
            >
              <RegisterForm {...props} />
            </motion.div>
          )}
        </AnimatePresence>

        <p className="mt-6 text-center text-xs text-muted-foreground">
          {isLogin ? (
            <>
              还没有账号？
              <button
                type="button"
                onClick={() => setMode("register")}
                className="ml-1 font-medium text-sienna hover:underline"
              >
                立即注册
              </button>
            </>
          ) : (
            <>
              已有账号？
              <button
                type="button"
                onClick={() => setMode("login")}
                className="ml-1 font-medium text-sienna hover:underline"
              >
                直接登录
              </button>
            </>
          )}
        </p>
      </motion.div>
    </motion.section>
  );
}

function LoginForm({ busy, loginForm, setLoginForm, onLogin }: AuthPanelProps) {
  return (
    <form className="space-y-4" onSubmit={onLogin}>
      <div className="space-y-2">
        <Label htmlFor="student_id">学号</Label>
        <Input
          id="student_id"
          value={loginForm.student_id}
          autoComplete="username"
          required
          onChange={(e) => setLoginForm({ ...loginForm, student_id: e.target.value })}
        />
      </div>
      <div className="space-y-2">
        <Label htmlFor="password">密码</Label>
        <Input
          id="password"
          type="password"
          value={loginForm.password}
          autoComplete="current-password"
          required
          onChange={(e) => setLoginForm({ ...loginForm, password: e.target.value })}
        />
      </div>
      <Button className="h-11 w-full gap-2" type="submit" disabled={busy}>
        {busy && <Loader2 className="size-4 animate-spin" />}
        登录
        {!busy && <ArrowRight className="size-4" />}
      </Button>
    </form>
  );
}

function RegisterForm({ busy, codeBusy, reg, setReg, onRegister, onSendCode }: AuthPanelProps) {
  return (
    <form className="space-y-4" onSubmit={onRegister}>
      <div className="space-y-2">
        <Label htmlFor="reg_student">学号</Label>
        <Input
          id="reg_student"
          value={reg.student_id}
          autoComplete="username"
          required
          onChange={(e) => setReg({ ...reg, student_id: e.target.value })}
        />
      </div>
      <div className="grid grid-cols-2 gap-3">
        <div className="space-y-2">
          <Label htmlFor="name">姓名</Label>
          <Input
            id="name"
            value={reg.name}
            autoComplete="name"
            onChange={(e) => setReg({ ...reg, name: e.target.value })}
          />
        </div>
        <div className="space-y-2">
          <Label htmlFor="class_id">班级</Label>
          <Input
            id="class_id"
            value={reg.class_id}
            onChange={(e) => setReg({ ...reg, class_id: e.target.value })}
          />
        </div>
      </div>
      <div className="space-y-2">
        <Label htmlFor="email">邮箱</Label>
        <div className="flex gap-2">
          <Input
            id="email"
            type="email"
            value={reg.email}
            autoComplete="email"
            required
            onChange={(e) => setReg({ ...reg, email: e.target.value })}
          />
          <Button
            type="button"
            variant="secondary"
            onClick={onSendCode}
            disabled={codeBusy || !reg.email.trim()}
          >
            {codeBusy ? "已发送" : "验证码"}
          </Button>
        </div>
      </div>
      <div className="grid grid-cols-2 gap-3">
        <div className="space-y-2">
          <Label htmlFor="reg_password">密码</Label>
          <Input
            id="reg_password"
            type="password"
            value={reg.password}
            autoComplete="new-password"
            minLength={6}
            required
            onChange={(e) => setReg({ ...reg, password: e.target.value })}
          />
        </div>
        <div className="space-y-2">
          <Label htmlFor="code">验证码</Label>
          <Input
            id="code"
            value={reg.code}
            inputMode="numeric"
            maxLength={6}
            required
            onChange={(e) => setReg({ ...reg, code: e.target.value })}
          />
        </div>
      </div>
      <Button className="h-11 w-full gap-2" type="submit" disabled={busy}>
        {busy && <Loader2 className="size-4 animate-spin" />}
        创建账号
      </Button>
    </form>
  );
}
