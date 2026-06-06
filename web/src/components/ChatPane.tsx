import { useEffect, useRef, useState } from "react";

import { useApp } from "../store";
import { formatTime, intentLabel, paperTitle, sessionTitle } from "../utils";
import type { Message, Reference } from "../types";
import { Markdown } from "./Markdown";
import { Empty, useGuard } from "./ui";

// 从助教消息 meta.sources 解析出引用出处。
function extractSources(meta?: Record<string, unknown>): Reference[] {
  if (!meta) return [];
  const raw = meta.sources;
  if (!Array.isArray(raw)) return [];
  return raw as Reference[];
}

function Sources({ refs }: { refs: Reference[] }) {
  if (refs.length === 0) return null;
  return (
    <div className="sources">
      <p className="sources-label">引用出处</p>
      <ol className="sources-list">
        {refs.map((r, i) => {
          const name = r.source_file || r.source_uri || `片段 ${i + 1}`;
          const scope = r.knowledge_scope === "public" ? "基础库" : "我的论文";
          return (
            <li key={i} className="source-item">
              <span className="source-index">{i + 1}</span>
              <span className="source-name" title={name}>
                {name}
              </span>
              {typeof r.page_no === "number" && r.page_no > 0 && (
                <span className="source-page">p.{r.page_no}</span>
              )}
              <span className={`source-scope ${r.knowledge_scope || ""}`}>
                {scope}
              </span>
            </li>
          );
        })}
      </ol>
    </div>
  );
}

function MessageBubble({ message }: { message: Message }) {
  const isAssistant = message.role === "assistant";
  const refs = isAssistant ? extractSources(message.meta) : [];
  return (
    <article className={`message ${isAssistant ? "assistant" : "user"}`}>
      <div className="message-bubble">
        {isAssistant ? (
          <Markdown>{message.content}</Markdown>
        ) : (
          <p className="message-text">{message.content}</p>
        )}
        {isAssistant && refs.length > 0 && <Sources refs={refs} />}
      </div>
      <div className="message-meta">
        <span>{isAssistant ? "助教" : "我"}</span>
        {isAssistant && message.intent && (
          <span className="intent-chip">{intentLabel(message.intent)}</span>
        )}
        {message.created_at && <span>{formatTime(message.created_at)}</span>}
      </div>
    </article>
  );
}

// 三点等待动画,助教思考中。
function Thinking() {
  return (
    <article className="message assistant">
      <div className="message-bubble thinking">
        <span className="dot-pulse">
          <i />
          <i />
          <i />
        </span>
        <span className="thinking-text">助教正在检索并作答…</span>
      </div>
    </article>
  );
}

// 常用问题,点一下填进输入框。空会话与欢迎页都展示。
const PROMPT_HINTS = [
  "这篇论文的核心贡献是什么?",
  "用了哪些数据集和评测指标?",
  "方法部分的整体流程是怎样的?",
];

function PromptHints({ onPick }: { onPick: (q: string) => void }) {
  return (
    <div className="prompt-hints">
      {PROMPT_HINTS.map((h) => (
        <button
          key={h}
          type="button"
          className="hint-chip"
          onClick={() => onPick(h)}
        >
          {h}
        </button>
      ))}
    </div>
  );
}

export function ChatPane() {
  const {
    messages,
    papers,
    activeSession,
    activePaper,
    sending,
    sendMessage,
    createSession,
  } = useApp();
  const guard = useGuard();
  const [input, setInput] = useState("");
  const listRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const el = listRef.current;
    if (el) el.scrollTop = el.scrollHeight;
  }, [messages, sending]);

  const submit = () => {
    const q = input.trim();
    if (!q || sending) return;
    setInput("");
    guard(() => sendMessage(q));
  };

  // 为当前选中论文新建会话(无论文则建普通会话)。
  const newSession = () =>
    guard(async () => {
      const title = activePaper ? `${paperTitle(activePaper)} 问答` : "新会话";
      await createSession(title, activePaper?.id);
    });

  // 当前会话绑定的论文(会话列表已按论文过滤,通常即 activePaper)。
  const sessionPaper = activeSession?.paper_id
    ? papers.find((p) => p.id === activeSession.paper_id) || null
    : null;

  const hasSession = Boolean(activeSession);
  const hasContent = hasSession || messages.length > 0;

  return (
    <div className="chat-body">
      <div className="chat-toolbar">
        <div className="chat-context-bar">
          <div className="ctx-info">
            {activeSession ? (
              <>
                <span className="ctx-title">{sessionTitle(activeSession)}</span>
                <span className="ctx-sub">
                  {sessionPaper
                    ? `围绕《${paperTitle(sessionPaper)}》`
                    : "跨库问答"}
                </span>
              </>
            ) : (
              <span className="ctx-sub">
                {activePaper
                  ? `准备围绕《${paperTitle(activePaper)}》提问`
                  : "未选择会话"}
              </span>
            )}
          </div>
          <button
            type="button"
            className="secondary-button compact"
            onClick={newSession}
          >
            ＋ 新建会话
          </button>
        </div>
      </div>

      <div className="message-list" ref={listRef}>
        {!hasContent ? (
          <div className="chat-welcome">
            <Empty
              title="准备开始论文问答"
              text={
                activePaper
                  ? `围绕「${paperTitle(activePaper)}」直接提问,或点上方「新建会话」。`
                  : "先在中栏选择一篇论文,再开始提问。下方输入也会自动建会话。"
              }
            />
            <PromptHints onPick={setInput} />
          </div>
        ) : (
          <>
            {messages.length === 0 && (
              <div className="empty-session">
                <Empty
                  title="这个会话还没有消息"
                  text="在下方输入问题,开始第一轮论文问答。"
                  inline
                />
                <PromptHints onPick={setInput} />
              </div>
            )}
            {messages.map((m) => (
              <MessageBubble key={String(m.id)} message={m} />
            ))}
            {sending && <Thinking />}
          </>
        )}
      </div>

      <form
        className="composer"
        onSubmit={(e) => {
          e.preventDefault();
          submit();
        }}
      >
        <textarea
          rows={3}
          value={input}
          placeholder="例如:这篇论文的核心贡献是什么?"
          disabled={sending}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter" && !e.shiftKey) {
              e.preventDefault();
              submit();
            }
          }}
        />
        <button
          type="submit"
          className="primary-button"
          disabled={sending || !input.trim()}
        >
          {sending ? "思考中…" : "提问"}
        </button>
      </form>
    </div>
  );
}
