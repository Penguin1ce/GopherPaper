"use client";

import { type ReactNode, useEffect, useRef, useState } from "react";

import * as api from "@/lib/gopherpaper/api";
import { cn } from "@/lib/utils";

// 论文封面:懒渲染 PDF 第一页为缩略图。
// - IntersectionObserver 进入视口才开始加载;
// - dataURL 进模块级缓存,翻页/重渲染不重复取;
// - 并发限流,避免一屏卡片同时拉几十个 PDF;
// - disableAutoFetch + 渲染完即 destroy,靠 Range 请求只取第一页所需字节。

// 卡片 CSS 宽度上限约 420px,乘设备像素比(封顶 2x)保证 Retina 清晰。
const COVER_CSS_WIDTH = 420;
// 封面区是 16:10 裁切,只渲染顶部约 72% 宽高比的区域,底下反正会被裁掉。
const COVER_CROP_RATIO = 0.72;
const MAX_CONCURRENT_RENDERS = 2;

const coverCache = new Map<string, string>();
const failedCovers = new Set<string>();

let activeRenders = 0;
const renderQueue: (() => void)[] = [];

function acquireRenderSlot(): Promise<void> {
  return new Promise((resolve) => {
    const run = () => {
      activeRenders += 1;
      resolve();
    };
    if (activeRenders < MAX_CONCURRENT_RENDERS) run();
    else renderQueue.push(run);
  });
}

function releaseRenderSlot() {
  activeRenders -= 1;
  renderQueue.shift()?.();
}

async function renderCover(paperID: string): Promise<string> {
  const { getDocument, GlobalWorkerOptions } = await import("pdfjs-dist");
  if (!GlobalWorkerOptions.workerSrc) {
    GlobalWorkerOptions.workerSrc = "/pdfjs/pdf.worker.min.mjs";
  }
  const task = getDocument({
    url: api.paperFileUrl(paperID),
    disableAutoFetch: true,
  });
  try {
    const doc = await task.promise;
    const page = await doc.getPage(1);
    const dpr = Math.min(2, globalThis.devicePixelRatio || 1);
    const renderWidth = COVER_CSS_WIDTH * dpr;
    const base = page.getViewport({ scale: 1 });
    const viewport = page.getViewport({ scale: renderWidth / base.width });
    const canvas = document.createElement("canvas");
    canvas.width = Math.ceil(viewport.width);
    canvas.height = Math.min(
      Math.ceil(viewport.height),
      Math.ceil(renderWidth * COVER_CROP_RATIO),
    );
    const ctx = canvas.getContext("2d");
    if (!ctx) throw new Error("canvas 2d 不可用");
    await page.render({ canvasContext: ctx, viewport }).promise;
    return canvas.toDataURL("image/jpeg", 0.92);
  } finally {
    void task.destroy();
  }
}

export function PaperCover({
  paperID,
  fallback,
  className,
}: {
  paperID: string;
  fallback?: ReactNode;
  className?: string;
}) {
  const ref = useRef<HTMLDivElement>(null);
  const [src, setSrc] = useState(() => coverCache.get(paperID) ?? "");
  const [failed, setFailed] = useState(() => failedCovers.has(paperID));

  useEffect(() => {
    setSrc(coverCache.get(paperID) ?? "");
    setFailed(failedCovers.has(paperID));
  }, [paperID]);

  useEffect(() => {
    if (src || failed) return;
    const el = ref.current;
    if (!el || typeof IntersectionObserver === "undefined") return;
    let cancelled = false;
    const io = new IntersectionObserver(
      (entries) => {
        if (!entries.some((entry) => entry.isIntersecting)) return;
        io.disconnect();
        void (async () => {
          await acquireRenderSlot();
          try {
            if (cancelled) return;
            const cached = coverCache.get(paperID);
            if (cached) {
              setSrc(cached);
              return;
            }
            const dataURL = await renderCover(paperID);
            coverCache.set(paperID, dataURL);
            if (!cancelled) setSrc(dataURL);
          } catch {
            failedCovers.add(paperID);
            if (!cancelled) setFailed(true);
          } finally {
            releaseRenderSlot();
          }
        })();
      },
      { rootMargin: "160px" },
    );
    io.observe(el);
    return () => {
      cancelled = true;
      io.disconnect();
    };
  }, [paperID, src, failed]);

  return (
    <div ref={ref} className={cn("relative h-full w-full", className)}>
      {src ? (
        <img
          src={src}
          alt=""
          aria-hidden
          draggable={false}
          className="h-full w-full object-cover object-top transition-transform duration-300 ease-out group-hover:scale-[1.02] motion-reduce:transition-none motion-reduce:group-hover:scale-100 dark:brightness-90"
        />
      ) : (
        <div className="flex h-full w-full items-center justify-center">
          {fallback}
        </div>
      )}
    </div>
  );
}
