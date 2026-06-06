import { useState } from "react";

import { useApp } from "../store";
import { useGuard } from "./ui";

export function AuthView() {
  const { login, registerAndLogin, sendCode } = useApp();
  const guard = useGuard();
  const [tab, setTab] = useState<"login" | "register">("login");
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
    <section className="auth-view">
      <div className="auth-copy">
        <p className="eyebrow">GopherPaper</p>
        <h1>
          把每一篇 PDF
          <br />
          读成<span className="ink">可对话的知识</span>
        </h1>
        <p className="lede">
          上传论文,系统自动解析、结构化抽取、构建知识库,围绕论文做带页码出处的多轮问答,并一键生成六类研读报告。
        </p>
        <ul className="auth-points">
          <li>
            <span className="dot" />
            MinerU 在线解析 + 章节感知切分
          </li>
          <li>
            <span className="dot" />
            多智能体编排:意图路由 → RAG 问答 / 抽取 / 报告
          </li>
          <li>
            <span className="dot" />
            答案附引用段落与页码,可溯源
          </li>
        </ul>
      </div>

      <div className="auth-panel">
        <div className="tab-bar" role="tablist">
          <button
            type="button"
            className={`tab-button${tab === "login" ? " active" : ""}`}
            onClick={() => setTab("login")}
          >
            登录
          </button>
          <button
            type="button"
            className={`tab-button${tab === "register" ? " active" : ""}`}
            onClick={() => setTab("register")}
          >
            注册
          </button>
        </div>

        {tab === "login" ? (
          <form className="form-stack" onSubmit={onLogin}>
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
              {busy ? "登录中…" : "登录"}
            </button>
          </form>
        ) : (
          <form className="form-stack" onSubmit={onRegister}>
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
              {busy ? "注册中…" : "注册并登录"}
            </button>
          </form>
        )}
      </div>
    </section>
  );
}
