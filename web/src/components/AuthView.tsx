import { useEffect, useState } from "react";

import { useApp } from "../store";
import { useGuard } from "./ui";

type AuthMode = "landing" | "login" | "register";

const PIPELINE_STEPS = ["Gather", "Screen", "Extract", "Report"];

const STEP_INTERVAL = 760;
const STEP_HOLD = 1700;

export function AuthView() {
  const { login, registerAndLogin, sendCode } = useApp();
  const guard = useGuard();
  const [mode, setMode] = useState<AuthMode>("landing");
  const [activeStep, setActiveStep] = useState(0);
  const [paused, setPaused] = useState(false);
  const [reduced, setReduced] = useState(false);
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

  // pause animations while the tab is hidden
  useEffect(() => {
    const onVis = () => setPaused(document.hidden);
    document.addEventListener("visibilitychange", onVis);
    return () => document.removeEventListener("visibilitychange", onVis);
  }, []);

  // honor reduced-motion: settle on the finished state, no loops
  useEffect(() => {
    const mq = window.matchMedia("(prefers-reduced-motion: reduce)");
    const apply = () => setReduced(mq.matches);
    apply();
    mq.addEventListener("change", apply);
    return () => mq.removeEventListener("change", apply);
  }, []);

  useEffect(() => {
    if (reduced) setActiveStep(PIPELINE_STEPS.length - 1);
  }, [reduced]);

  // advance the pipeline one step at a time, then loop
  useEffect(() => {
    if (mode !== "landing" || paused || reduced) return;
    const last = PIPELINE_STEPS.length - 1;
    const delay = activeStep >= last ? STEP_HOLD : STEP_INTERVAL;
    const timer = window.setTimeout(() => {
      setActiveStep((step) => (step >= last ? 0 : step + 1));
    }, delay);
    return () => window.clearTimeout(timer);
  }, [mode, activeStep, paused, reduced]);

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

  const authForm =
    mode === "login" ? (
      <form className="form-stack auth-form" onSubmit={onLogin}>
        <label>
          <span>学号</span>
          <input
            value={loginForm.student_id}
            autoComplete="username"
            required
            onChange={(e) =>
              setLoginForm({ ...loginForm, student_id: e.target.value })
            }
          />
        </label>
        <label>
          <span>密码</span>
          <input
            type="password"
            value={loginForm.password}
            autoComplete="current-password"
            required
            onChange={(e) =>
              setLoginForm({ ...loginForm, password: e.target.value })
            }
          />
        </label>
        <button className="primary-button" type="submit" disabled={busy}>
          {busy ? "Signing in..." : "Sign in"}
        </button>
      </form>
    ) : (
      <form className="form-stack auth-form" onSubmit={onRegister}>
        <label>
          <span>学号</span>
          <input
            value={reg.student_id}
            autoComplete="username"
            required
            onChange={(e) => setReg({ ...reg, student_id: e.target.value })}
          />
        </label>
        <div className="field-row">
          <label>
            <span>姓名</span>
            <input
              value={reg.name}
              autoComplete="name"
              onChange={(e) => setReg({ ...reg, name: e.target.value })}
            />
          </label>
          <label>
            <span>班级</span>
            <input
              value={reg.class_id}
              onChange={(e) => setReg({ ...reg, class_id: e.target.value })}
            />
          </label>
        </div>
        <label>
          <span>邮箱</span>
          <div className="inline-field">
            <input
              type="email"
              value={reg.email}
              autoComplete="email"
              required
              onChange={(e) => setReg({ ...reg, email: e.target.value })}
            />
            <button
              type="button"
              className="secondary-button"
              onClick={onSendCode}
              disabled={codeBusy}
            >
              {codeBusy ? "已发送" : "发送验证码"}
            </button>
          </div>
        </label>
        <div className="field-row">
          <label>
            <span>密码</span>
            <input
              type="password"
              value={reg.password}
              autoComplete="new-password"
              minLength={6}
              required
              onChange={(e) => setReg({ ...reg, password: e.target.value })}
            />
          </label>
          <label>
            <span>验证码</span>
            <input
              value={reg.code}
              inputMode="numeric"
              maxLength={6}
              required
              onChange={(e) => setReg({ ...reg, code: e.target.value })}
            />
          </label>
        </div>
        <button className="primary-button" type="submit" disabled={busy}>
          {busy ? "Creating..." : "Sign up"}
        </button>
      </form>
    );

  return (
    <section className={`auth-view auth-view-${mode}`}>
      <div className="auth-halo" aria-hidden />
      <div className="auth-dot-field left" aria-hidden />
      <div className="auth-dot-field right" aria-hidden />

      <header className="auth-nav">
        <button
          type="button"
          className="auth-brand"
          onClick={() => setMode("landing")}
        >
          <span className="auth-logo" aria-hidden>
            G
          </span>
          <span>GopherPaper</span>
        </button>
        {mode === "landing" ? (
          <div className="auth-actions">
            <button
              type="button"
              className="auth-link"
              onClick={() => setMode("login")}
            >
              Sign in
            </button>
            <button
              type="button"
              className="auth-signup"
              onClick={() => setMode("register")}
            >
              Sign up
            </button>
          </div>
        ) : (
          <button
            type="button"
            className="auth-link"
            onClick={() => setMode("landing")}
          >
            Back
          </button>
        )}
      </header>

      {mode === "landing" ? (
        <div className="auth-landing">
          <main className="auth-hero">
            <h1>AI for Scientific Research</h1>
            <p>GopherPaper helps students be 10x more evidence-based.</p>
            <button
              type="button"
              className="hero-cta"
              onClick={() => setMode("login")}
            >
              <span>Try now</span>
              <span aria-hidden>→</span>
            </button>
          </main>

          <p className="sr-only">
            GopherPaper runs an automated research pipeline: it gathers papers,
            screens them, extracts insights, and generates a concise summary
            report from your question.
          </p>
          <div className="hero-mock" aria-hidden>
            <div className="mock-charts">
              <svg viewBox="0 0 1080 400" preserveAspectRatio="xMidYMax slice">
                <g stroke="rgba(20,41,34,0.07)" strokeWidth="1">
                  <line x1="70" y1="60" x2="1020" y2="60" />
                  <line x1="70" y1="135" x2="1020" y2="135" />
                  <line x1="70" y1="210" x2="1020" y2="210" />
                  <line x1="70" y1="285" x2="1020" y2="285" />
                  <line x1="70" y1="360" x2="1020" y2="360" />
                  <line x1="190" y1="40" x2="190" y2="380" />
                  <line x1="430" y1="40" x2="430" y2="380" />
                  <line x1="670" y1="40" x2="670" y2="380" />
                  <line x1="910" y1="40" x2="910" y2="380" />
                </g>
                <g
                  fill="rgba(20,41,34,0.34)"
                  fontSize="13"
                  fontWeight="600"
                  textAnchor="end"
                >
                  <text x="56" y="64">1000</text>
                  <text x="56" y="139">750</text>
                  <text x="56" y="214">500</text>
                  <text x="56" y="289">250</text>
                  <text x="56" y="364">0</text>
                </g>
                <polyline
                  fill="none"
                  stroke="rgba(30,109,120,0.5)"
                  strokeWidth="2.5"
                  strokeLinejoin="round"
                  strokeLinecap="round"
                  points="70,330 190,250 310,275 430,195 670,150 790,95 910,135 1020,90"
                />
                <polyline
                  fill="none"
                  stroke="rgba(30,109,120,0.32)"
                  strokeWidth="2.5"
                  strokeLinejoin="round"
                  strokeLinecap="round"
                  strokeDasharray="6 6"
                  points="70,150 190,185 310,230 430,255 670,300 790,320 910,345 1020,360"
                />
                <g fill="#fbfcf6" stroke="rgba(30,109,120,0.7)" strokeWidth="2">
                  <circle cx="70" cy="330" r="4.5" />
                  <circle cx="190" cy="250" r="4.5" />
                  <circle cx="910" cy="135" r="4.5" />
                  <circle cx="1020" cy="90" r="4.5" />
                  <circle cx="790" cy="95" r="4.5" />
                </g>
              </svg>
            </div>
            <div className="mock-paper">
              <div className="mock-progress">
                {PIPELINE_STEPS.map((label, i) => {
                  const state =
                    i < activeStep
                      ? "done"
                      : i === activeStep
                        ? "active"
                        : "pending";
                  return (
                    <div key={label} className={`mock-pstep ${state}`}>
                      <span className="mock-node" />
                      <span className="mock-plabel">{label}</span>
                    </div>
                  );
                })}
              </div>
              <h3 className="mock-title">
                The cognitive effects of L-theanine: a systematic review of human
                clinical trials
              </h3>
              <p className="mock-abstract">
                L-theanine, alone or with caffeine, modestly improves attention and
                stress-related cognition, though the evidence remains limited and
                heterogeneous.
              </p>
              <div className="mock-skeleton">
                <span />
                <span />
                <span />
              </div>
              <div className="mock-chips">
                <span className="mock-chip">n = 150</span>
                <span className="mock-chip">n = 249</span>
              </div>
            </div>
          </div>
        </div>
      ) : (
        <main className="auth-account">
          <div className="auth-panel">
            <div className="auth-panel-head">
              <h1>{mode === "login" ? "Sign in" : "Sign up"}</h1>
              <p>
                {mode === "login"
                  ? "Continue to your research workspace."
                  : "Create your research workspace."}
              </p>
            </div>
            <div className="tab-bar" role="tablist">
              <button
                type="button"
                role="tab"
                aria-selected={mode === "login"}
                className={`tab-button${mode === "login" ? " active" : ""}`}
                onClick={() => setMode("login")}
              >
                Sign in
              </button>
              <button
                type="button"
                role="tab"
                aria-selected={mode === "register"}
                className={`tab-button${mode === "register" ? " active" : ""}`}
                onClick={() => setMode("register")}
              >
                Sign up
              </button>
            </div>
            {authForm}
          </div>
        </main>
      )}
    </section>
  );
}
