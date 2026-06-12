// 精读页:独立入口(/reader?id=<论文ID>),新标签页打开。
// 左栏 pdf.js 渲染原始 PDF 含可选文本层,选中英文原文浮出"翻译",
// 点击后右栏生成卡片调翻译 agent 回填中文。不挂主应用 store,
// token 从同源 localStorage 取(与主页面共享登录态)。
import { StrictMode, useEffect, useMemo, useRef, useState } from "react";
import { createRoot } from "react-dom/client";
import { Document, Page, pdfjs } from "react-pdf";
import "react-pdf/dist/Page/TextLayer.css";
import "react-pdf/dist/Page/AnnotationLayer.css";
import workerSrc from "pdfjs-dist/build/pdf.worker.min.mjs?url";

import * as api from "./api";
import "./reader.css";

pdfjs.GlobalWorkerOptions.workerSrc = workerSrc;

const AUTH_KEY = "gopherpaper.auth";

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

// 选区浮标:记录待翻译文本与屏幕坐标(视口固定定位)。
interface Selection {
  text: string;
  x: number;
  y: number;
}

// 翻译卡片:原文 + 译文,带加载与错误态。
interface Card {
  id: number;
  original: string;
  translation: string;
  loading: boolean;
  error: string;
}

function Reader() {
  const id = useMemo(
    () => new URLSearchParams(location.search).get("id") || "",
    [],
  );
  const [ready, setReady] = useState(false);
  const [title, setTitle] = useState("");
  const [numPages, setNumPages] = useState(0);
  const [error, setError] = useState("");
  const docRef = useRef<HTMLDivElement>(null);
  const [pageWidth, setPageWidth] = useState(820);
  const [sel, setSel] = useState<Selection | null>(null);
  const [cards, setCards] = useState<Card[]>([]);
  const nextCardId = useRef(1);

  function closeReader() {
    window.close();
    window.setTimeout(() => {
      if (!window.closed) location.href = "/";
    }, 120);
  }

  // 初始化:无 token 跳回登录,无 id 报错,否则拉标题。
  useEffect(() => {
    const token = loadToken();
    if (!token) {
      location.replace("/");
      return;
    }
    api.setToken(token);
    if (!id) {
      setError("缺少论文 id");
      return;
    }
    setReady(true);
    api
      .paperDetail(id)
      .then((d) => setTitle(d.paper.title || d.paper.file_name))
      .catch(() => {});
  }, [id]);

  // 页宽随容器自适应,留出边距。
  useEffect(() => {
    const el = docRef.current;
    if (!el) return;
    const ro = new ResizeObserver((entries) => {
      const w = entries[0]?.contentRect.width ?? 0;
      if (w > 0) setPageWidth(Math.min(960, Math.max(280, w - 48)));
    });
    ro.observe(el);
    return () => ro.disconnect();
  }, [ready]);

  // 依赖 ready:token 在 effect 里 setToken 后才置 ready,
  // 此时重算才拿得到带 token 的地址(否则首渲染 token 还是空)。
  const pdfUrl = useMemo(
    () => (ready && id ? api.paperFileUrl(id) : ""),
    [ready, id],
  );

  // 鼠标松开:读当前选区,非空则在选区上方浮出翻译按钮。
  function onMouseUp() {
    const s = window.getSelection();
    const text = s?.toString().trim() ?? "";
    if (!text || !s || s.rangeCount === 0) {
      setSel(null);
      return;
    }
    const rect = s.getRangeAt(0).getBoundingClientRect();
    setSel({ text, x: rect.left + rect.width / 2, y: rect.top });
  }

  // 把选中原文加成卡片并调翻译接口回填。
  function addCard(text: string) {
    const cid = nextCardId.current++;
    setCards((prev) => [
      { id: cid, original: text, translation: "", loading: true, error: "" },
      ...prev,
    ]);
    setSel(null);
    window.getSelection()?.removeAllRanges();
    api
      .translate(id, text)
      .then((r) =>
        setCards((prev) =>
          prev.map((c) =>
            c.id === cid
              ? { ...c, translation: r.translation, loading: false }
              : c,
          ),
        ),
      )
      .catch((e) =>
        setCards((prev) =>
          prev.map((c) =>
            c.id === cid
              ? { ...c, error: (e as Error)?.message || "翻译失败", loading: false }
              : c,
          ),
        ),
      );
  }

  function removeCard(id: number) {
    setCards((prev) => prev.filter((c) => c.id !== id));
  }

  return (
    <div className="reader-shell">
      <header className="reader-bar">
        <div className="reader-brand">
          <span className="reader-logo" aria-hidden>
            R
          </span>
          <div className="reader-title-wrap">
            <p className="eyebrow">GopherPaper Reader</p>
            <h1 className="reader-title">{title || "论文精读"}</h1>
          </div>
        </div>
        <div className="reader-actions">
          <span className="reader-page-pill">
            {numPages > 0 ? `${numPages} 页` : "PDF 精读"}
          </span>
          <button type="button" className="reader-back" onClick={closeReader}>
            关闭
          </button>
        </div>
      </header>
      <div className="reader-body">
        <section className="reader-doc-pane">
          <div className="reader-panel-head">
            <div>
              <span className="reader-panel-title">PDF 原文</span>
              <p>选中左侧英文段落，浮出按钮后即可翻译。</p>
            </div>
            <span className={`reader-status ${ready && !error ? "ready" : ""}`}>
              {error ? "加载异常" : ready ? "文本层已启用" : "准备中"}
            </span>
          </div>
          <main className="reader-doc" ref={docRef} onMouseUp={onMouseUp}>
            {error ? (
              <div className="reader-error">{error}</div>
            ) : ready ? (
              <Document
                file={pdfUrl}
                loading={<div className="reader-hint">加载 PDF…</div>}
                error={<div className="reader-error">PDF 加载失败</div>}
                onLoadSuccess={(d) => setNumPages(d.numPages)}
                onLoadError={(e) => setError(e.message)}
              >
                {Array.from({ length: numPages }, (_, i) => (
                  <Page
                    key={i}
                    pageNumber={i + 1}
                    width={pageWidth}
                    className="reader-page"
                    renderTextLayer
                    renderAnnotationLayer
                  />
                ))}
              </Document>
            ) : null}
          </main>
        </section>
        <aside className="reader-trans">
          <div className="reader-trans-head">
            <div>
              <span className="reader-panel-title">摘录翻译</span>
              <p>{cards.length > 0 ? `${cards.length} 条摘录` : "等待选区"}</p>
            </div>
            {cards.length > 0 && (
              <button
                type="button"
                className="reader-clear"
                onClick={() => setCards([])}
              >
                清空
              </button>
            )}
          </div>
          {cards.length === 0 ? (
            <div className="reader-trans-empty">
              <h2>还没有翻译摘录</h2>
              <p>选中左侧英文原文，点击浮出的「翻译」即可生成中文。</p>
            </div>
          ) : (
            <div className="reader-cards">
              {cards.map((c, i) => (
                <article
                  key={c.id}
                  className={`reader-card${c.loading ? " loading" : ""}${c.error ? " error" : ""}`}
                  aria-busy={c.loading}
                >
                  <header className="reader-card-head">
                    <span>摘录 {cards.length - i}</span>
                    <button
                      type="button"
                      className="reader-card-remove"
                      onClick={() => removeCard(c.id)}
                      title="删除摘录"
                    >
                      ×
                    </button>
                  </header>
                  <div className="reader-card-src">{c.original}</div>
                  <div className="reader-card-dst">
                    {c.loading
                      ? "翻译中…"
                      : c.error
                        ? `失败：${c.error}`
                        : c.translation}
                  </div>
                </article>
              ))}
            </div>
          )}
        </aside>
      </div>

      {sel && (
        <button
          className="reader-sel-btn"
          style={{ left: sel.x, top: sel.y }}
          onMouseDown={(e) => {
            e.preventDefault(); // 防止按下时清掉选区
            addCard(sel.text);
          }}
        >
          翻译
        </button>
      )}
    </div>
  );
}

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <Reader />
  </StrictMode>,
);
