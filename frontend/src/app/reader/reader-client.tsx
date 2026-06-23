"use client";

import { ArrowLeft, Loader2, Trash2, X } from "lucide-react";
import { useEffect, useMemo, useRef, useState } from "react";
import { Document, Page, pdfjs } from "react-pdf";
import "react-pdf/dist/Page/TextLayer.css";
import "react-pdf/dist/Page/AnnotationLayer.css";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { ScrollArea } from "@/components/ui/scroll-area";
import * as api from "@/lib/gopherpaper/api";

pdfjs.GlobalWorkerOptions.workerSrc = "/pdfjs/pdf.worker.min.mjs";

const AUTH_KEY = "gopherpaper.auth";

function loadToken(): string {
  if (typeof window === "undefined") return "";
  try {
    const raw = localStorage.getItem(AUTH_KEY);
    if (!raw) return "";
    return (JSON.parse(raw) as { token?: string }).token || "";
  } catch {
    return "";
  }
}

interface Selection {
  text: string;
  x: number;
  y: number;
}

interface CardItem {
  id: number;
  original: string;
  translation: string;
  loading: boolean;
  error: string;
}

export function ReaderClient() {
  const id = useMemo(() => {
    if (typeof location === "undefined") return "";
    return new URLSearchParams(location.search).get("id") || "";
  }, []);
  const [ready, setReady] = useState(false);
  const [title, setTitle] = useState("");
  const [numPages, setNumPages] = useState(0);
  const [error, setError] = useState("");
  const docRef = useRef<HTMLDivElement>(null);
  const [pageWidth, setPageWidth] = useState(820);
  const [sel, setSel] = useState<Selection | null>(null);
  const [cards, setCards] = useState<CardItem[]>([]);
  const nextCardId = useRef(1);

  function closeReader() {
    window.close();
    window.setTimeout(() => {
      if (!window.closed) location.href = "/";
    }, 120);
  }

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

  const pdfUrl = useMemo(() => (ready && id ? api.paperFileUrl(id) : ""), [ready, id]);

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
            c.id === cid ? { ...c, translation: r.translation, loading: false } : c,
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

  return (
    <main className="flex h-dvh min-h-[640px] flex-col bg-muted/40">
      <header className="flex h-14 shrink-0 items-center justify-between border-b bg-background px-4">
        <div className="flex min-w-0 items-center gap-3">
          <Button type="button" variant="ghost" size="icon" onClick={closeReader}>
            <ArrowLeft className="size-4" />
          </Button>
          <div className="min-w-0">
            <div className="truncate text-sm font-medium">{title || "论文精读"}</div>
            <div className="text-xs text-muted-foreground">
              {numPages > 0 ? `${numPages} 页` : "PDF Reader"}
            </div>
          </div>
        </div>
        <Button type="button" variant="secondary" onClick={closeReader}>
          关闭
        </Button>
      </header>
      <div className="grid min-h-0 flex-1 grid-cols-1 lg:grid-cols-[minmax(0,1fr)_24rem]">
        <section className="min-h-0 border-r">
          <div className="flex h-full flex-col">
            <div className="flex shrink-0 items-center justify-between border-b bg-background px-4 py-3">
              <div>
                <div className="text-sm font-medium">PDF 原文</div>
                <div className="text-xs text-muted-foreground">选中英文段落后点击翻译</div>
              </div>
              <span className="text-xs text-muted-foreground">
                {error ? "加载异常" : ready ? "文本层已启用" : "准备中"}
              </span>
            </div>
            <ScrollArea className="min-h-0 flex-1">
              <div ref={docRef} className="mx-auto max-w-5xl p-4" onMouseUp={onMouseUp}>
                {error ? (
                  <div className="rounded-lg border bg-background p-8 text-center text-sm text-destructive">
                    {error}
                  </div>
                ) : ready ? (
                  <Document
                    file={pdfUrl}
                    loading={
                      <div className="flex items-center justify-center gap-2 rounded-lg border bg-background p-8 text-sm text-muted-foreground">
                        <Loader2 className="size-4 animate-spin" />
                        加载 PDF…
                      </div>
                    }
                    error={<div className="rounded-lg border bg-background p-8 text-center text-sm text-destructive">PDF 加载失败</div>}
                    onLoadSuccess={(d) => setNumPages(d.numPages)}
                    onLoadError={(e) => setError(e.message)}
                  >
                    {Array.from({ length: numPages }, (_, i) => (
                      <Page
                        key={i}
                        pageNumber={i + 1}
                        width={pageWidth}
                        renderTextLayer
                        renderAnnotationLayer
                      />
                    ))}
                  </Document>
                ) : null}
              </div>
            </ScrollArea>
          </div>
        </section>
        <aside className="min-h-0 bg-background">
          <div className="flex h-full flex-col">
            <div className="flex shrink-0 items-center justify-between border-b px-4 py-3">
              <div>
                <div className="text-sm font-medium">摘录翻译</div>
                <div className="text-xs text-muted-foreground">
                  {cards.length > 0 ? `${cards.length} 条摘录` : "等待选区"}
                </div>
              </div>
              {cards.length > 0 && (
                <Button type="button" variant="ghost" size="icon" onClick={() => setCards([])}>
                  <Trash2 className="size-4" />
                </Button>
              )}
            </div>
            <ScrollArea className="min-h-0 flex-1">
              <div className="space-y-3 p-4">
                {cards.length === 0 ? (
                  <div className="rounded-lg border border-dashed bg-muted/30 p-8 text-center">
                    <h2 className="text-sm font-medium">还没有翻译摘录</h2>
                    <p className="mt-1 text-sm text-muted-foreground">
                      选中左侧英文原文，点击浮出的“翻译”。
                    </p>
                  </div>
                ) : (
                  cards.map((c, i) => (
                    <Card key={c.id}>
                      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                        <CardTitle className="text-sm">摘录 {cards.length - i}</CardTitle>
                        <Button
                          type="button"
                          variant="ghost"
                          size="icon-sm"
                          onClick={() => setCards((prev) => prev.filter((x) => x.id !== c.id))}
                        >
                          <X className="size-3.5" />
                        </Button>
                      </CardHeader>
                      <CardContent className="space-y-3 text-sm">
                        <div className="rounded-md bg-muted p-3 text-muted-foreground">{c.original}</div>
                        <div className="leading-6">
                          {c.loading ? (
                            <span className="inline-flex items-center gap-2 text-muted-foreground">
                              <Loader2 className="size-4 animate-spin" />
                              翻译中…
                            </span>
                          ) : c.error ? (
                            <span className="text-destructive">{c.error}</span>
                          ) : (
                            c.translation
                          )}
                        </div>
                      </CardContent>
                    </Card>
                  ))
                )}
              </div>
            </ScrollArea>
          </div>
        </aside>
      </div>
      {sel && (
        <Button
          type="button"
          className="fixed z-50 -translate-x-1/2 -translate-y-full shadow-lg"
          style={{ left: sel.x, top: Math.max(54, sel.y - 10) }}
          onClick={() => addCard(sel.text)}
        >
          翻译
        </Button>
      )}
    </main>
  );
}
