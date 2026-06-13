// 小云雀页:独立入口(/pioneer),多面手 agent 会话(查论文 + 瑞幸点单等工具)。
// 不挂主应用 store,登录态从同源 localStorage 共享(与精读页同模式)。
// 瑞幸 token 仅存浏览器 localStorage,发消息时随 X-Luckin-Token 头透传,服务端不落库。
import { StrictMode, memo, useCallback, useEffect, useRef, useState } from "react";
import { createRoot } from "react-dom/client";

import * as api from "./api";
import { Markdown } from "./components/Markdown";
import type { Message, Session } from "./types";
import { formatTime, sessionTitle } from "./utils";
import "./pioneer.css";

const AUTH_KEY = "gopherpaper.auth";
const LUCKIN_KEY = "gopherpaper.luckin";
const LUCKIN_TTL_DAYS = 30; // 瑞幸 token 有效期一个月,本地按绑定时间估算剩余天数
const LUCKIN_HEADER = "X-Luckin-Token";
const AGENT_TYPE = "pioneer";

// loadToken 从主应用共享的 localStorage 取登录 token,缺失返回空。
function loadToken(): string {
  try {
    const raw = localStorage.getItem(AUTH_KEY);
    if (!raw) return "";
    return (JSON.parse(raw) as { token?: string }).token || "";
  } catch {
    return "";
  }
}

// 瑞幸凭据:token 与绑定时刻,只活在本浏览器。
interface LuckinCred {
  token: string;
  savedAt: number;
}

function loadLuckin(): LuckinCred | null {
  try {
    const raw = localStorage.getItem(LUCKIN_KEY);
    if (!raw) return null;
    const cred = JSON.parse(raw) as LuckinCred;
    return cred.token ? cred : null;
  } catch {
    return null;
  }
}

function luckinDaysLeft(cred: LuckinCred): number {
  const elapsed = (Date.now() - cred.savedAt) / 86400000;
  return Math.max(0, Math.ceil(LUCKIN_TTL_DAYS - elapsed));
}

// 瑞幸绑定卡:展示绑定状态与剩余天数,绑定/换绑/解绑都在本地完成。
function LuckinCard({
  cred,
  onChange,
}: {
  cred: LuckinCred | null;
  onChange: (c: LuckinCred | null) => void;
}) {
  const [editing, setEditing] = useState(false);
  const [input, setInput] = useState("");

  const save = () => {
    const token = input.trim();
    if (!token) return;
    const next: LuckinCred = { token, savedAt: Date.now() };
    localStorage.setItem(LUCKIN_KEY, JSON.stringify(next));
    onChange(next);
    setInput("");
    setEditing(false);
  };
  const unbind = () => {
    localStorage.removeItem(LUCKIN_KEY);
    onChange(null);
    setEditing(false);
  };

  const daysLeft = cred ? luckinDaysLeft(cred) : 0;
  const expired = cred !== null && daysLeft <= 0;

  return (
    <section className="luckin-card">
      <header className="luckin-head">
        <div>
          <span className="luckin-title">瑞幸点单</span>
          <p className="luckin-copy">token 只存在当前浏览器</p>
        </div>
        {cred ? (
          <span className={`luckin-badge ${expired ? "expired" : "bound"}`}>
            {expired ? "已过期" : `已绑定 · 剩 ${daysLeft} 天`}
          </span>
        ) : (
          <span className="luckin-badge unbound">未绑定</span>
        )}
      </header>
      {editing || (!cred && !editing) ? (
        <div className="luckin-form">
          <input
            type="password"
            value={input}
            placeholder="粘贴瑞幸 MCP token"
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && save()}
          />
          <div className="luckin-actions">
            <button type="button" className="pioneer-btn primary" onClick={save} disabled={!input.trim()}>
              保存
            </button>
            {cred && (
              <button type="button" className="pioneer-btn" onClick={() => setEditing(false)}>
                取消
              </button>
            )}
          </div>
          <p className="luckin-hint">
            在
            <a href="https://open.lkcoffee.com/mcp" target="_blank" rel="noreferrer">
              瑞幸开放平台
            </a>
            登录创建 token。token 只存当前浏览器,发消息时透传给点单工具,服务端不保存。
          </p>
        </div>
      ) : (
        <div className="luckin-actions">
          <button type="button" className="pioneer-btn" onClick={() => setEditing(true)}>
            换绑
          </button>
          <button type="button" className="pioneer-btn danger" onClick={unbind}>
            解绑
          </button>
        </div>
      )}
    </section>
  );
}

// 流式期间 messages 每帧重建,未变动的历史消息仍是同一对象引用,
// memo 默认浅比较即可让历史气泡跳过重渲染,只剩流式那条随增量刷新。
const Bubble = memo(function Bubble({ message }: { message: Message }) {
  const isAssistant = message.role === "assistant";
  return (
    <article className={`p-message ${isAssistant ? "assistant" : "user"}`}>
      <div className="p-bubble">
        {isAssistant ? (
          <Markdown richLinks>{message.content}</Markdown>
        ) : (
          <p className="p-text">{message.content}</p>
        )}
      </div>
      <div className="p-meta">
        <span>{isAssistant ? "小云雀" : "我"}</span>
        {message.created_at && <span>{formatTime(message.created_at)}</span>}
      </div>
    </article>
  );
});

// 工具调用状态气泡,占 composer 整行排在输入框上方,有状态文案时浮现。
function ToolStatus({ note }: { note: string }) {
  if (!note) return null;
  return (
    <div className="tool-status" role="status">
      <span className="tool-spinner" aria-hidden />
      <span className="tool-status-text">{note}</span>
    </div>
  );
}

function Thinking() {
  return (
    <article className="p-message assistant">
      <div className="p-bubble thinking">
        <span className="dot-pulse" aria-hidden>
          <i />
          <i />
          <i />
        </span>
        <span>小云雀正在处理工具与上下文…</span>
      </div>
    </article>
  );
}

const PROMPT_HINTS = [
  "帮我找几篇关于注意力机制的经典论文",
  "我在国贸,帮我点一杯瑞幸生椰拿铁",
  "查一下我刚才那单咖啡好了没",
];

function PromptHints({ onPick }: { onPick: (q: string) => void }) {
  return (
    <div className="p-hints">
      {PROMPT_HINTS.map((h) => (
        <button key={h} type="button" className="hint" onClick={() => onPick(h)}>
          {h}
        </button>
      ))}
    </div>
  );
}

function PioneerApp() {
  const [token] = useState(loadToken);
  const [sessions, setSessions] = useState<Session[]>([]);
  const [activeID, setActiveID] = useState("");
  const [messages, setMessages] = useState<Message[]>([]);
  const [sending, setSending] = useState(false);
  const [toolNote, setToolNote] = useState("");
  const [input, setInput] = useState("");
  const [error, setError] = useState("");
  const [luckin, setLuckin] = useState<LuckinCred | null>(loadLuckin);
  const listRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    api.setToken(token);
  }, [token]);

  useEffect(() => {
    const el = listRef.current;
    if (el) el.scrollTop = el.scrollHeight;
  }, [messages, sending, toolNote]);

  const fail = (e: unknown) =>
    setError(e instanceof Error ? e.message : "请求失败,请稍后再试");

  const openSession = useCallback(async (id: string) => {
    setActiveID(id);
    setError("");
    try {
      const msgs = await api.listMessages(id);
      setMessages(Array.isArray(msgs) ? msgs : []);
    } catch (e) {
      fail(e);
    }
  }, []);

  // 启动:拉会话列表,只留小云雀会话,默认打开最近一个。
  useEffect(() => {
    if (!token) return;
    api
      .listSessions()
      .then((list) => {
        const mine = (Array.isArray(list) ? list : []).filter(
          (s) => s.agent_type === AGENT_TYPE,
        );
        setSessions(mine);
        if (mine.length > 0) void openSession(mine[0].id);
      })
      .catch(fail);
  }, [token, openSession]);

  const newSession = async (): Promise<Session | null> => {
    try {
      const s = await api.createSession("云雀会话", undefined, AGENT_TYPE);
      setSessions((list) => [s, ...list]);
      setActiveID(s.id);
      setMessages([]);
      return s;
    } catch (e) {
      fail(e);
      return null;
    }
  };

  const removeSession = async (id: string) => {
    try {
      await api.deleteSession(id);
      setSessions((list) => list.filter((s) => s.id !== id));
      if (activeID === id) {
        setActiveID("");
        setMessages([]);
      }
    } catch (e) {
      fail(e);
    }
  };

  const send = async () => {
    const q = input.trim();
    if (!q || sending) return;
    setError("");
    let sid = activeID;
    if (!sid) {
      const s = await newSession();
      if (!s) return;
      sid = s.id;
    }
    setInput("");
    setSending(true);
    // 即时上屏用户消息,真实事件 ID 由下次 listMessages 还原。
    setMessages((list) => [
      ...list,
      {
        id: `local-${Date.now()}`,
        session_id: sid,
        role: "user",
        content: q,
        created_at: new Date().toISOString(),
      },
    ]);
    // SSE 流式占位:首个文本增量到达时上屏一条 streaming 助手消息,
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
            session_id: sid,
            role: "assistant",
            content: "",
            created_at: new Date().toISOString(),
            streaming: true,
          }),
        ]);
        return;
      }
      setMessages((list) => list.map((m) => (m.id === placeholderID ? fn(m) : m)));
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
    const cancelFlush = () => {
      if (rafID !== null) {
        cancelAnimationFrame(rafID);
        rafID = null;
      }
    };
    try {
      const headers: Record<string, string> = {};
      if (luckin?.token) headers[LUCKIN_HEADER] = luckin.token;
      const data = await api.sendMessage(sid, q, headers, {
        onDelta: (text) => {
          pending += text;
          if (rafID === null) rafID = requestAnimationFrame(flush);
        },
        // 工具状态不进消息气泡,显示在输入框上方的独立状态气泡。
        onTool: (tool, done) =>
          setToolNote(done ? `${tool} 已返回,正在继续…` : `正在调用 ${tool} …`),
      });
      // 收尾:取消待处理的帧回调,最终消息直接整体替换占位。
      cancelFlush();
      setMessages((list) => [
        ...list.filter((m) => m.id !== placeholderID),
        { ...data.message, id: data.message.id || `local-a-${Date.now()}` },
      ]);
    } catch (e) {
      cancelFlush();
      setMessages((list) => list.filter((m) => m.id !== placeholderID));
      fail(e);
    } finally {
      setSending(false);
      setToolNote("");
    }
  };

  if (!token) {
    return (
      <div className="pioneer-gate">
        <span className="pioneer-logo gate-logo" aria-hidden>
          雀
        </span>
        <h1>小云雀</h1>
        <p>
          还没有登录态。请先回 <a href="/">GopherPaper 主页</a> 登录,再回到本页。
        </p>
      </div>
    );
  }

  const activeSession = sessions.find((s) => s.id === activeID);

  return (
    <div className="pioneer-shell">
      <aside className="pioneer-side">
        <header className="pioneer-brand">
          <a className="brand-back" href="/" title="返回主应用" aria-label="返回主应用">
            ←
          </a>
          <div className="pioneer-brand-mark">
            <span className="pioneer-logo" aria-hidden>
              雀
            </span>
            <div>
              <p className="eyebrow">GopherPaper</p>
              <h1>小云雀</h1>
              <p>查论文 · 点咖啡的多面手助手</p>
            </div>
          </div>
        </header>
        <button type="button" className="pioneer-btn primary wide" onClick={() => void newSession()}>
          + 新会话
        </button>
        <nav className="pioneer-sessions" aria-label="云雀会话">
          {sessions.length === 0 && <p className="side-empty">还没有会话</p>}
          {sessions.map((s) => (
            <div
              key={s.id}
              className={`session-item ${s.id === activeID ? "active" : ""}`}
            >
              <button
                type="button"
                className="session-main"
                onClick={() => void openSession(s.id)}
              >
                <span className="session-name">{sessionTitle(s)}</span>
                <span className="session-time">
                  云雀会话 · {formatTime(s.updated_at || s.created_at)}
                </span>
              </button>
              <button
                type="button"
                className="session-del"
                title="删除会话"
                onClick={(e) => {
                  e.stopPropagation();
                  void removeSession(s.id);
                }}
              >
                ×
              </button>
            </div>
          ))}
        </nav>
        <LuckinCard cred={luckin} onChange={setLuckin} />
      </aside>

      <main className="pioneer-main">
        <div className="pioneer-toolbar">
          <div className="ctx-info">
            {activeSession ? (
              <>
                <span className="ctx-title">{sessionTitle(activeSession)}</span>
                <span className="ctx-sub">工具增强会话</span>
              </>
            ) : (
              <span className="ctx-sub">输入消息会自动创建云雀会话</span>
            )}
          </div>
          <button
            type="button"
            className="pioneer-btn secondary compact"
            onClick={() => void newSession()}
          >
            + 新建会话
          </button>
        </div>

        <div className="p-list" ref={listRef}>
          {messages.length === 0 && !sending ? (
            <div className="p-welcome">
              <h2>嗨,我是小云雀</h2>
              <p>学术问题、找论文、点杯瑞幸,都可以直接说。</p>
              <PromptHints onPick={setInput} />
            </div>
          ) : (
            <>
              {messages.map((m) => (
                <Bubble key={String(m.id)} message={m} />
              ))}
              {sending && !messages.some((m) => m.streaming) && <Thinking />}
            </>
          )}
        </div>

        {error && <div className="p-error">{error}</div>}

        <form
          className="p-composer"
          onSubmit={(e) => {
            e.preventDefault();
            void send();
          }}
        >
          <ToolStatus note={toolNote} />
          <textarea
            rows={3}
            value={input}
            placeholder="我在重庆大学虎溪校区图书馆，帮我点杯冰美式吧☕"
            disabled={sending}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter" && !e.shiftKey) {
                e.preventDefault();
                void send();
              }
            }}
          />
          <button type="submit" className="pioneer-btn primary" disabled={sending || !input.trim()}>
            {sending ? "思考中…" : "发送"}
          </button>
        </form>
      </main>
    </div>
  );
}

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <PioneerApp />
  </StrictMode>,
);
