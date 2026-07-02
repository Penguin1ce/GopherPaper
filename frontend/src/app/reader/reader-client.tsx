"use client";

import {
  ArrowLeft,
  ArrowUp,
  BookMarked,
  Check,
  ChevronLeft,
  ChevronRight,
  Languages,
  ListTree,
  Loader2,
  Maximize2,
  MessageCircleQuestionMark,
  MessageSquarePlus,
  Minus,
  MoreHorizontal,
  Network,
  NotebookPen,
  Palette,
  Plus,
  Trash2,
  X,
} from "lucide-react";
import {
  GlobalWorkerOptions,
  getDocument,
  type OnProgressParameters,
  type PDFDocumentProxy,
} from "pdfjs-dist";
import type { DocumentInitParameters } from "pdfjs-dist/types/src/display/api";
import { PDFViewer } from "pdfjs-dist/web/pdf_viewer.mjs";
import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type CSSProperties,
  type FormEvent,
  type PointerEvent as ReactPointerEvent,
  type ReactNode,
} from "react";
import {
  MonitoredHighlightContainer,
  PdfHighlighter,
  TextHighlight,
  scaledPositionToViewport,
  useHighlightContainerContext,
  usePdfHighlighterContext,
  type Highlight,
  type PdfHighlighterUtils,
  type PdfScaleValue,
  type PdfSelection,
  type Scaled,
} from "react-pdf-highlighter-plus";
import "pdfjs-dist/web/pdf_viewer.css";
import "react-pdf-highlighter-plus/style/style.css";

import { Button } from "@/components/ui/button";
import { Markdown } from "@/components/gopherpaper/markdown";
import { ReadingMindMap } from "@/components/gopherpaper/reading-mind-map";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Textarea } from "@/components/ui/textarea";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import * as api from "@/lib/gopherpaper/api";
import { useApp } from "@/lib/gopherpaper/store";
import type {
  AnnotationRect,
  Message,
  Paper,
  PaperAnnotation,
  PaperSection,
  ReaderContext,
  Reference,
} from "@/lib/gopherpaper/types";
import { cn } from "@/lib/utils";

const AUTH_KEY = "gopherpaper.auth";
const PDF_WORKER = "/pdfjs/pdf.worker.min.mjs";
const PDF_VIEWER_PATCH_FLAG = "__gopherpaperSkipSameDocumentSet";
const READER_PREFS_KEY = "gopherpaper.reader.preferences";
const RIGHT_PANEL_WIDTH = "24rem";
const MIND_MAP_PANEL_DEFAULT_WIDTH = 560;
const MIND_MAP_PANEL_MIN_WIDTH = 380;
const MIND_MAP_PANEL_MAX_WIDTH = 860;
const PDF_MIN_SCALE = 0.6;
const PDF_MAX_SCALE = 2.4;
const LOCATE_TOP_GAP = 32;
const LOCATED_ANNOTATION_SCROLL_RESUME_MS = 600;
const QA_SELECTION_PREVIEW_RUNES = 48;

type PatchablePDFViewer = {
  pdfDocument?: PDFDocumentProxy | null;
  setDocument: (pdfDocument: PDFDocumentProxy | null) => void;
  [PDF_VIEWER_PATCH_FLAG]?: boolean;
};

type AnnotationColor = "yellow" | "blue" | "green" | "pink" | "purple" | "orange";

const COLOR_META: Record<
  AnnotationColor,
  { label: string; className: string; value: string }
> = {
  yellow: {
    label: "黄色",
    className: "bg-amber-300",
    value: "rgba(255, 226, 143, 0.62)",
  },
  blue: {
    label: "蓝色",
    className: "bg-sky-300",
    value: "rgba(147, 197, 253, 0.5)",
  },
  green: {
    label: "绿色",
    className: "bg-emerald-300",
    value: "rgba(134, 239, 172, 0.5)",
  },
  pink: {
    label: "粉色",
    className: "bg-rose-300",
    value: "rgba(253, 164, 175, 0.5)",
  },
  purple: {
    label: "紫色",
    className: "bg-violet-300",
    value: "rgba(196, 181, 253, 0.54)",
  },
  orange: {
    label: "橙色",
    className: "bg-orange-300",
    value: "rgba(253, 186, 116, 0.56)",
  },
};

const COLOR_KEYS = Object.keys(COLOR_META) as AnnotationColor[];

interface ReaderPreferences {
  outlineOpen: boolean;
  translateOpen: boolean;
  annotationsOpen: boolean;
  mindMapOpen: boolean;
  qaOpen: boolean;
  color: AnnotationColor;
}

const DEFAULT_PREFS: ReaderPreferences = {
  outlineOpen: false,
  translateOpen: true,
  annotationsOpen: true,
  mindMapOpen: false,
  qaOpen: false,
  color: "yellow",
};

function patchPdfViewerSetDocument() {
  const prototype = PDFViewer.prototype as unknown as PatchablePDFViewer;
  if (prototype[PDF_VIEWER_PATCH_FLAG]) return;

  const originalSetDocument = prototype.setDocument;
  prototype.setDocument = function setDocumentOnce(
    this: PatchablePDFViewer,
    pdfDocument: PDFDocumentProxy | null,
  ) {
    if (this.pdfDocument === pdfDocument) return;
    originalSetDocument.call(this, pdfDocument);
  };
  prototype[PDF_VIEWER_PATCH_FLAG] = true;
}

patchPdfViewerSetDocument();

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

function loadRequestedPage(): number {
  if (typeof location === "undefined") return 0;
  const params = new URLSearchParams(location.search);
  const raw = params.get("page") || params.get("page_no") || params.get("p") || "";
  const page = Number.parseInt(raw, 10);
  return Number.isFinite(page) && page > 0 ? page : 0;
}

function loadReaderPreferences(): ReaderPreferences {
  if (typeof window === "undefined") return DEFAULT_PREFS;
  try {
    const raw = localStorage.getItem(READER_PREFS_KEY);
    if (!raw) return DEFAULT_PREFS;
    const saved = JSON.parse(raw) as Partial<ReaderPreferences>;
    const color = COLOR_KEYS.includes(saved.color as AnnotationColor)
      ? (saved.color as AnnotationColor)
      : DEFAULT_PREFS.color;
    const mindMapOpen = saved.mindMapOpen ?? DEFAULT_PREFS.mindMapOpen;
    const qaOpen = mindMapOpen ? false : (saved.qaOpen ?? DEFAULT_PREFS.qaOpen);
    return {
      outlineOpen: saved.outlineOpen ?? DEFAULT_PREFS.outlineOpen,
      translateOpen: mindMapOpen || qaOpen ? false : (saved.translateOpen ?? DEFAULT_PREFS.translateOpen),
      annotationsOpen: mindMapOpen || qaOpen ? false : (saved.annotationsOpen ?? DEFAULT_PREFS.annotationsOpen),
      mindMapOpen,
      qaOpen,
      color,
    };
  } catch {
    localStorage.removeItem(READER_PREFS_KEY);
    return DEFAULT_PREFS;
  }
}

function paperName(paper: Paper | null, fallback = "") {
  return paper?.title || paper?.file_name || fallback || "论文精读";
}

function colorValue(color?: string) {
  return COLOR_META[(color as AnnotationColor) || "yellow"]?.value || COLOR_META.yellow.value;
}

function clamp(n: number, min: number, max: number) {
  return Math.min(max, Math.max(min, n));
}

function rectToScaled(rect: AnnotationRect): Scaled {
  return {
    x1: rect.x1,
    y1: rect.y1,
    x2: rect.x2,
    y2: rect.y2,
    width: rect.width,
    height: rect.height,
    pageNumber: rect.pageNumber,
  };
}

function scaledToRect(rect: Scaled): AnnotationRect {
  return {
    x1: rect.x1,
    y1: rect.y1,
    x2: rect.x2,
    y2: rect.y2,
    width: rect.width,
    height: rect.height,
    pageNumber: rect.pageNumber,
  };
}

interface ReaderHighlight extends Highlight {
  type: "text";
  annotation: PaperAnnotation;
  content: { text: string };
}

function annotationToHighlight(annotation: PaperAnnotation): ReaderHighlight {
  return {
    id: String(annotation.id),
    type: "text",
    annotation,
    content: { text: annotation.text },
    position: {
      boundingRect: rectToScaled(annotation.bounding_rect),
      rects: annotation.rects.map(rectToScaled),
    },
  };
}

function scrollHighlightToTop(
  utils: PdfHighlighterUtils,
  highlight: ReaderHighlight,
): HTMLElement | null {
  const viewer = utils.getViewer();
  if (!viewer) return null;
  const pageNumber = highlight.position.boundingRect.pageNumber;
  const pageView = viewer.getPageView(pageNumber - 1);
  const viewport = pageView?.viewport;
  if (!viewport) return null;
  const viewportPosition = scaledPositionToViewport(highlight.position, viewer);

  viewer.scrollPageIntoView({
    pageNumber,
    destArray: [
      null,
      { name: "XYZ" },
        ...viewport.convertToPdfPoint(
          0,
          Math.max(0, viewportPosition.boundingRect.top - LOCATE_TOP_GAP),
        ),
      0,
    ],
  });
  return viewer.container ?? null;
}

interface TranslationResult {
  original: string;
  translation: string;
  pageNo: number;
  loading: boolean;
  error: string;
}

type EventBusCallback = (evt: { pageNumber?: number } | unknown) => void;

interface EventBusLike {
  on: (event: string, callback: EventBusCallback) => void;
  off: (event: string, callback: EventBusCallback) => void;
}

interface PdfViewerScaleLike {
  currentScale?: number;
  container?: HTMLElement | null;
}

interface PdfOutlineItem {
  title?: string;
  dest?: string | unknown[] | null;
  items?: PdfOutlineItem[] | null;
}

interface OutlineEntry {
  section: PaperSection;
  level: number;
  number: string;
  title: string;
  rawTitle: string;
}

interface OutlineNode extends OutlineEntry {
  children: OutlineNode[];
}

function isEventBus(value: unknown): value is EventBusLike {
  if (!value || typeof value !== "object") return false;
  const candidate = value as { on?: unknown; off?: unknown };
  return typeof candidate.on === "function" && typeof candidate.off === "function";
}

function pdfViewerWithScale(utils: PdfHighlighterUtils | null): PdfViewerScaleLike | null {
  const viewer = utils?.getViewer();
  if (!viewer || typeof viewer !== "object") return null;
  return viewer as PdfViewerScaleLike;
}

function toPdfError(error: unknown): Error {
  if (error instanceof Error) return error;
  return new Error(String(error || "PDF 加载失败"));
}

function splitSectionNumber(title: string): { number: string; title: string } | null {
  const match = title.trim().match(/^((?:\d+(?:\.\d+)*|[IVXLC]+|[A-Z])[.)]?)\s+(.+)$/i);
  if (!match) return null;
  return {
    number: match[1].replace(/[.)]$/, ""),
    title: match[2].trim(),
  };
}

function normalizedOutlineTitle(title: string) {
  return normalizeSearchText(title);
}

function isPaperTitleSection(section: PaperSection, paperTitle?: string) {
  if (!paperTitle) return false;
  return normalizedOutlineTitle(section.title) === normalizedOutlineTitle(paperTitle);
}

function normalizedOutlinePhrase(title: string) {
  return title.replace(/\s+/g, " ").trim();
}

function isStandaloneTopLevelTitle(title: string) {
  const normalized = normalizedOutlinePhrase(title).toLowerCase().replace(/[.:\uFF1A]+$/, "");
  return /^(abstract|acknowledg(?:e)?ments?|references|bibliography|appendix|appendices|supplementary materials?|limitations?|ethics statement|broader impacts?|impact statement|data availability|funding|conflicts? of interest)$/.test(normalized);
}

function isOutlineNoiseSection(section: PaperSection) {
  const title = normalizedOutlinePhrase(section.title);
  if (!title || splitSectionNumber(title) || isStandaloneTopLevelTitle(title)) return false;
  const lower = title.toLowerCase().replace(/[。]+$/, ".");
  if (/^(fig(?:ure)?|table|algorithm|equation|eq\.?)\s*[\divxlc]+[.:：)\s]/i.test(title)) {
    return true;
  }
  if (/\b(our|main|key)?\s*contributions?\s+(are|is|can be summarized)\s+as\s+follows\b/i.test(title)) {
    return true;
  }
  if (/\b(the|this)\s+(paper|article|work|section)\s+(is\s+organized|proceeds|is\s+structured)\s+as\s+follows\b/i.test(title)) {
    return true;
  }
  if (/^(in\s+)?(this|our)\s+(paper|work|section|study)\b.+\.$/i.test(lower)) {
    return true;
  }
  if (/^we\s+(make|propose|present|introduce|summarize|highlight|show|demonstrate|provide)\b.+\.$/i.test(lower)) {
    return true;
  }
  return false;
}

function outlineLevel(section: PaperSection, parsed: ReturnType<typeof splitSectionNumber>, prevLevel: number) {
  let rawLevel = Math.max(1, Math.round(section.level || 1));
  if (parsed && /^\d+(\.\d+)*$/.test(parsed.number)) {
    rawLevel = parsed.number.split(".").length;
  } else if (parsed && /^[IVXLC]+$/i.test(parsed.number)) {
    rawLevel = 1;
  } else if (parsed && /^[A-Z]$/.test(parsed.number)) {
    rawLevel = 1;
  } else if (isStandaloneTopLevelTitle(section.title)) {
    rawLevel = 1;
  }
  return Math.min(rawLevel, prevLevel + 1);
}

function normalizeSearchText(text: string) {
  return text
    .toLowerCase()
    .replace(/[\s\-_–—:;,.()[\]{}]+/g, "")
    .trim();
}

function numberedOutline(sections: PaperSection[]): OutlineEntry[] {
  // 先看全篇是否带显式编号:带编号的论文用作者标注的真实编号(未编号的摘要/参考文献留空更准确);
  // 完全没编号的论文再按层级生成兜底序号,保证目录前始终能看到可读的序号。
  const hasExplicit = sections.some((section) => {
    const parsed = splitSectionNumber(section.title);
    return parsed != null && /^\d+(\.\d+)*$/.test(parsed.number);
  });

  const counters: number[] = [];
  let prevLevel = 0;
  return sections.map((section) => {
    const parsed = splitSectionNumber(section.title);
    // 标题自带点分编号(如 3 / 3.1 / 3.1.2)时,层级以编号深度为准:
    // MinerU 的 text_level 常把同级标题判成不同层,而编号本身是作者标注的可靠层级。
    // 无编号(Abstract、References 等)再回退到后端给出的 level。
    const level = outlineLevel(section, parsed, prevLevel);
    prevLevel = level;

    // 维护层级计数器以生成兜底序号。
    counters.length = level;
    counters[level - 1] = (counters[level - 1] || 0) + 1;
    const generated = counters.join(".");

    return {
      section,
      level,
      number: hasExplicit ? (parsed?.number ?? "") : generated,
      title: parsed?.title ?? section.title,
      rawTitle: section.title,
    };
  });
}

// buildOutlineTree 按 level 把扁平条目还原成层级树,供折叠展示。
function buildOutlineTree(entries: OutlineEntry[]): OutlineNode[] {
  const roots: OutlineNode[] = [];
  const stack: OutlineNode[] = [];
  for (const entry of entries) {
    const node: OutlineNode = { ...entry, children: [] };
    while (stack.length > 0 && stack[stack.length - 1].level >= node.level) {
      stack.pop();
    }
    if (stack.length === 0) roots.push(node);
    else stack[stack.length - 1].children.push(node);
    stack.push(node);
  }
  return roots;
}

function outlineSearchTerms(entry: OutlineEntry) {
  return Array.from(
    new Set(
      [
        entry.rawTitle,
        entry.title,
        `${entry.number} ${entry.title}`,
        `${entry.number}. ${entry.title}`,
      ]
        .map((term) => term.trim())
        .filter(Boolean),
    ),
  );
}

function findPageElement(page: number) {
  return document.querySelector<HTMLElement>(
    `.page[data-page-number="${page}"], [data-page-number="${page}"]`,
  );
}

function findTitleElement(page: number, terms: string[]) {
  const pageElement = findPageElement(page);
  if (!pageElement) return null;
  const spans = Array.from(
    pageElement.querySelectorAll<HTMLElement>(".textLayer span, .textLayer [role='presentation']"),
  ).filter((span) => span.textContent?.trim());
  const normalizedTerms = terms.map(normalizeSearchText).filter((term) => term.length >= 4);
  if (normalizedTerms.length === 0 || spans.length === 0) return null;

  const longest = Math.max(...normalizedTerms.map((term) => term.length));
  let best: HTMLElement | null = null;
  let bestScore = 0;

  // 以每个 span 作为标题起点向后拼接:标题往往被拆成多个 span,且正文里也会出现同样的词,
  // 因此要求“起点 span 是某候选词的前缀”而非任意子串命中,避免命中页面靠前的零碎片段。
  for (let start = 0; start < spans.length; start += 1) {
    let combined = "";
    for (let end = start; end < Math.min(spans.length, start + 16); end += 1) {
      combined += spans[end].textContent || "";
      const normalized = normalizeSearchText(combined);
      if (normalized.length < 4) continue;
      const hit = normalizedTerms.find(
        (term) => term.startsWith(normalized) || normalized.startsWith(term),
      );
      if (hit) {
        const matchLen = Math.min(normalized.length, hit.length);
        const fontSize = Number.parseFloat(getComputedStyle(spans[start]).fontSize) || 0;
        // 命中越长越可信;字号大的多半是真正的章节标题,用它在并列时做区分。
        const score = matchLen * 4 + fontSize;
        if (score > bestScore) {
          bestScore = score;
          best = spans[start];
        }
      }
      if (normalized.length > longest + 4) break;
    }
  }
  return best;
}

function flashTitleElement(element: HTMLElement) {
  const previousOutline = element.style.outline;
  const previousOutlineOffset = element.style.outlineOffset;
  const previousBackground = element.style.backgroundColor;
  const previousTransition = element.style.transition;
  element.style.transition = "background-color 160ms ease, outline-color 160ms ease";
  element.style.backgroundColor = "rgba(250, 204, 21, 0.28)";
  element.style.outline = "2px solid rgba(245, 158, 11, 0.72)";
  element.style.outlineOffset = "2px";
  window.setTimeout(() => {
    element.style.backgroundColor = previousBackground;
    element.style.outline = previousOutline;
    element.style.outlineOffset = previousOutlineOffset;
    element.style.transition = previousTransition;
  }, 1600);
}

function scrollTitleIntoView(page: number, terms: string[]) {
  let attempts = 0;
  const tick = () => {
    const target = findTitleElement(page, terms);
    if (target) {
      const previousScrollMarginTop = target.style.scrollMarginTop;
      target.style.scrollMarginTop = `${LOCATE_TOP_GAP}px`;
      target.scrollIntoView({ block: "start", inline: "nearest", behavior: "smooth" });
      window.setTimeout(() => {
        target.style.scrollMarginTop = previousScrollMarginTop;
      }, 1600);
      flashTitleElement(target);
      return;
    }
    attempts += 1;
    if (attempts < 18) {
      window.setTimeout(tick, 120);
    }
  };
  window.setTimeout(tick, 180);
}

async function outlinePage(pdfDocument: PDFDocumentProxy, dest: string | unknown[] | null | undefined) {
  let explicitDest = dest;
  if (typeof explicitDest === "string") {
    explicitDest = await pdfDocument.getDestination(explicitDest);
  }
  if (!Array.isArray(explicitDest) || explicitDest.length === 0) return 0;
  const ref = explicitDest[0];
  if (typeof ref === "object" && ref !== null) {
    try {
      return (await pdfDocument.getPageIndex(ref as Parameters<PDFDocumentProxy["getPageIndex"]>[0])) + 1;
    } catch {
      return 0;
    }
  }
  if (typeof ref === "number" && Number.isFinite(ref)) {
    return Math.max(1, Math.round(ref));
  }
  return 0;
}

async function extractPdfOutline(pdfDocument: PDFDocumentProxy, paperID: string): Promise<PaperSection[]> {
  const outline = (await pdfDocument.getOutline()) as PdfOutlineItem[] | null;
  if (!outline?.length) return [];
  const sections: PaperSection[] = [];
  let order = 0;

  const walk = async (items: PdfOutlineItem[], level: number) => {
    for (const item of items) {
      const title = item.title?.trim();
      const pageNo = await outlinePage(pdfDocument, item.dest);
      if (title && pageNo > 0) {
        sections.push({
          id: -(order + 1),
          paper_id: paperID,
          level,
          title,
          page_no: pageNo,
          order_idx: order,
        });
        order += 1;
      }
      if (item.items?.length) {
        await walk(item.items, level + 1);
      }
    }
  };

  await walk(outline, 1);
  return sections;
}

function ToolbarButton({
  label,
  active,
  disabled,
  onClick,
  children,
}: {
  label: string;
  active?: boolean;
  disabled?: boolean;
  onClick?: () => void;
  children: ReactNode;
}) {
  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <Button
            type="button"
            variant={active ? "secondary" : "ghost"}
            size="icon-sm"
            disabled={disabled}
            aria-label={label}
            aria-pressed={active}
            className={cn(
              "size-8 rounded-md text-muted-foreground hover:text-foreground",
              active && "text-foreground",
            )}
          >
            {children}
          </Button>
        }
        onClick={onClick}
      />
      <TooltipContent>{label}</TooltipContent>
    </Tooltip>
  );
}

function ToolbarDivider() {
  return <span className="mx-1 h-6 w-px bg-border" aria-hidden="true" />;
}

function ColorSwatches({
  value,
  onChange,
  disabled,
  compact = false,
}: {
  value: string;
  onChange: (color: AnnotationColor) => void;
  disabled?: boolean;
  compact?: boolean;
}) {
  return (
    <div className="flex items-center gap-1">
      {COLOR_KEYS.map((key) => {
        const meta = COLOR_META[key];
        const selected = value === key;
        return (
          <button
            key={key}
            type="button"
            title={meta.label}
            aria-label={meta.label}
            aria-pressed={selected}
            disabled={disabled}
            onClick={() => onChange(key)}
            className={cn(
              "grid place-items-center rounded-full border transition hover:scale-105 disabled:opacity-50",
              compact ? "size-5" : "size-6",
              meta.className,
              selected ? "border-foreground shadow-sm" : "border-transparent",
            )}
          >
            {selected && <Check className={compact ? "size-2.5" : "size-3"} />}
          </button>
        );
      })}
    </div>
  );
}

function ReaderToolbar({
  title,
  currentPage,
  numPages,
  pageDraft,
  scaleValue,
  prefs,
  onClose,
  onPrevPage,
  onNextPage,
  onPageDraftChange,
  onPageSubmit,
  onZoomIn,
  onZoomOut,
  onResetZoom,
  onFitWidth,
  onToggleTranslate,
  onToggleAnnotations,
  onToggleMindMap,
  onToggleQA,
  onColorChange,
}: {
  title: string;
  currentPage: number;
  numPages: number;
  pageDraft: string;
  scaleValue: PdfScaleValue;
  prefs: ReaderPreferences;
  onClose: () => void;
  onPrevPage: () => void;
  onNextPage: () => void;
  onPageDraftChange: (value: string) => void;
  onPageSubmit: () => void;
  onZoomIn: () => void;
  onZoomOut: () => void;
  onResetZoom: () => void;
  onFitWidth: () => void;
  onToggleTranslate: () => void;
  onToggleAnnotations: () => void;
  onToggleMindMap: () => void;
  onToggleQA: () => void;
  onColorChange: (color: AnnotationColor) => void;
}) {
  const zoomText = typeof scaleValue === "number" ? `${Math.round(scaleValue * 100)}%` : "适宽";

  const submitPage = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    onPageSubmit();
  };

  return (
    <header className="grid h-12 shrink-0 grid-cols-[minmax(0,1fr)_auto_minmax(0,1fr)] items-center gap-3 border-b bg-background/95 px-3 shadow-sm backdrop-blur">
      <div className="flex min-w-0 items-center gap-2">
        <ToolbarButton label="返回工作台" onClick={onClose}>
          <ArrowLeft className="size-4" />
        </ToolbarButton>
        <div className="min-w-0">
          <div className="truncate text-sm font-semibold">{title}</div>
        </div>
      </div>
      <div className="flex min-w-0 items-center justify-center gap-2">
        <ToolbarButton label="上一页" disabled={currentPage <= 1} onClick={onPrevPage}>
          <ChevronLeft className="size-4" />
        </ToolbarButton>
        <form onSubmit={submitPage} className="flex items-center gap-1 text-xs text-muted-foreground">
          <input
            value={pageDraft}
            onChange={(event) => onPageDraftChange(event.target.value)}
            inputMode="numeric"
            aria-label="页码"
            className="h-8 w-14 rounded-md border bg-background px-2 text-center text-sm font-medium text-foreground outline-none focus:border-ring focus:ring-2 focus:ring-ring/20"
          />
          <span className="min-w-10">/ {numPages || "-"}</span>
        </form>
        <ToolbarButton
          label="下一页"
          disabled={numPages > 0 && currentPage >= numPages}
          onClick={onNextPage}
        >
          <ChevronRight className="size-4" />
        </ToolbarButton>
        <ToolbarDivider />
        <ToolbarButton label="缩小" onClick={onZoomOut}>
          <Minus className="size-4" />
        </ToolbarButton>
        <button
          type="button"
          onClick={onResetZoom}
          className="h-8 min-w-14 rounded-md px-2 text-xs font-medium text-muted-foreground transition hover:bg-muted hover:text-foreground"
        >
          {zoomText}
        </button>
        <ToolbarButton label="放大" onClick={onZoomIn}>
          <Plus className="size-4" />
        </ToolbarButton>
        <ToolbarButton label="适宽展示" active={scaleValue === "page-width"} onClick={onFitWidth}>
          <Maximize2 className="size-4" />
        </ToolbarButton>
        <ToolbarDivider />
        <Palette className="ml-1 size-4 text-muted-foreground" />
        <ColorSwatches value={prefs.color} onChange={onColorChange} compact />
      </div>
      <div className="flex min-w-0 items-center justify-end gap-1">
        <ToolbarButton label="翻译面板" active={prefs.translateOpen} onClick={onToggleTranslate}>
          <Languages className="size-4" />
        </ToolbarButton>
        <ToolbarButton label="批注面板" active={prefs.annotationsOpen} onClick={onToggleAnnotations}>
          <BookMarked className="size-4" />
        </ToolbarButton>
        <ToolbarButton label="精读脑图" active={prefs.mindMapOpen} onClick={onToggleMindMap}>
          <Network className="size-4" />
        </ToolbarButton>
        <ToolbarButton label="小耄耋问答" active={prefs.qaOpen} onClick={onToggleQA}>
          <MessageSquarePlus className="size-4" />
        </ToolbarButton>
      </div>
    </header>
  );
}

function OutlineTreeNode({
  node,
  depth,
  activeId,
  activeTrail,
  expanded,
  onToggle,
  onGoToEntry,
}: {
  node: OutlineNode;
  depth: number;
  activeId: number | null;
  activeTrail: Set<number>;
  expanded: Set<number>;
  onToggle: (id: number) => void;
  onGoToEntry: (entry: OutlineEntry) => void;
}) {
  const hasChildren = node.children.length > 0;
  const isOpen = expanded.has(node.section.id);
  const isActive = activeId === node.section.id;
  const inTrail = !isActive && activeTrail.has(node.section.id);
  const rowRef = useRef<HTMLDivElement | null>(null);

  // 阅读位置滚到本节时,把目录里的高亮项带进可视区,实现“目录跟随”。
  useEffect(() => {
    if (isActive) {
      rowRef.current?.scrollIntoView({ block: "nearest" });
    }
  }, [isActive]);

  return (
    <div>
      <div
        ref={rowRef}
        className={cn(
          "flex items-center gap-1 rounded-md pr-2 text-sm transition hover:bg-muted",
          isActive && "bg-accent text-accent-foreground",
          inTrail && "font-medium text-foreground",
        )}
        style={{ paddingLeft: `${4 + depth * 14}px` }}
      >
        {hasChildren ? (
          <button
            type="button"
            aria-label={isOpen ? "收起子目录" : "展开子目录"}
            aria-expanded={isOpen}
            onClick={() => onToggle(node.section.id)}
            className="grid size-5 shrink-0 place-items-center rounded text-muted-foreground transition hover:text-foreground"
          >
            <ChevronRight className={cn("size-3.5 transition-transform", isOpen && "rotate-90")} />
          </button>
        ) : (
          <span className="size-5 shrink-0" aria-hidden="true" />
        )}
        <button
          type="button"
          onClick={() => onGoToEntry(node)}
          className="flex min-w-0 flex-1 items-start gap-2 py-1.5 text-left leading-5"
        >
          {node.number && (
            <span className="shrink-0 tabular-nums text-muted-foreground">{node.number}</span>
          )}
          <span className="gp-outline-title line-clamp-2 min-w-0">
            <Markdown compact>{node.title}</Markdown>
          </span>
        </button>
      </div>
      {hasChildren && isOpen && (
        <div>
          {node.children.map((child) => (
            <OutlineTreeNode
              key={child.section.id}
              node={child}
              depth={depth + 1}
              activeId={activeId}
              activeTrail={activeTrail}
              expanded={expanded}
              onToggle={onToggle}
              onGoToEntry={onGoToEntry}
            />
          ))}
        </div>
      )}
    </div>
  );
}

function OutlineDrawer({
  sections,
  paperTitle,
  currentPage,
  activeSectionId,
  onClose,
  onGoToEntry,
}: {
  sections: PaperSection[];
  paperTitle?: string;
  currentPage: number;
  activeSectionId?: number | null;
  onClose: () => void;
  onGoToEntry: (entry: OutlineEntry) => void;
}) {
  const sorted = useMemo(() => {
    const seen = new Set<string>();
    return sections
      .filter((section) => section.title && section.page_no > 0)
      .filter((section) => !isPaperTitleSection(section, paperTitle))
      .filter((section) => !isOutlineNoiseSection(section))
      .slice()
      .sort((a, b) => a.order_idx - b.order_idx || a.page_no - b.page_no)
      .filter((section) => {
        // 同页同名标题(常见于页眉/页脚被识别成标题)只保留首个,降低目录噪声。
        const key = `${normalizeSearchText(section.title)}@${section.page_no}`;
        if (seen.has(key)) return false;
        seen.add(key);
        return true;
      });
  }, [paperTitle, sections]);
  const entries = useMemo(() => numberedOutline(sorted), [sorted]);
  const tree = useMemo(() => buildOutlineTree(entries), [entries]);
  const pageActive = useMemo(
    () => [...entries].reverse().find((entry) => entry.section.page_no <= currentPage) ?? null,
    [entries, currentPage],
  );
  const clickedActive = useMemo(
    () => entries.find((entry) => entry.section.id === activeSectionId) ?? null,
    [activeSectionId, entries],
  );
  const active = clickedActive ?? pageActive;
  const activeId = active?.section.id ?? null;

  // 当前阅读位置所在节点的祖先链:折叠状态下也能在父级标题上标出“你在这里”。
  const activeTrail = useMemo(() => {
    const trail = new Set<number>();
    if (!active) return trail;
    const path: number[] = [];
    const dfs = (nodes: OutlineNode[]): boolean => {
      for (const node of nodes) {
        path.push(node.section.id);
        if (node.section.id === active.section.id || dfs(node.children)) return true;
        path.pop();
      }
      return false;
    };
    dfs(tree);
    path.forEach((id) => trail.add(id));
    return trail;
  }, [active, tree]);

  // 二级及以下默认收起。
  const [expanded, setExpanded] = useState<Set<number>>(() => new Set());
  const toggle = useCallback((id: number) => {
    setExpanded((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }, []);

  // 翻到新章节时自动展开其所在分支,让高亮项随阅读位置跟随显示;
  // 其余分支保持收起,手动收起当前分支后只要不换章节就不会被强行展开。
  useEffect(() => {
    if (activeId == null) return;
    const path: number[] = [];
    const dfs = (nodes: OutlineNode[]): boolean => {
      for (const node of nodes) {
        path.push(node.section.id);
        if (node.section.id === activeId || dfs(node.children)) return true;
        path.pop();
      }
      return false;
    };
    dfs(tree);
    if (path.length <= 1) return; // 顶级条目本就可见,无需展开
    setExpanded((prev) => {
      let changed = false;
      const next = new Set(prev);
      // 只展开祖先(不含 active 自身),保证其可见而不强行铺开它的子目录。
      for (let i = 0; i < path.length - 1; i += 1) {
        if (!next.has(path[i])) {
          next.add(path[i]);
          changed = true;
        }
      }
      return changed ? next : prev;
    });
  }, [activeId, tree]);

  const hasNested = useMemo(() => tree.some((node) => node.children.length > 0), [tree]);
  const allExpanded = useMemo(() => {
    const ids: number[] = [];
    const collect = (nodes: OutlineNode[]) => {
      for (const node of nodes) {
        if (node.children.length > 0) {
          ids.push(node.section.id);
          collect(node.children);
        }
      }
    };
    collect(tree);
    return ids.length > 0 && ids.every((id) => expanded.has(id));
  }, [tree, expanded]);

  const toggleAll = useCallback(() => {
    setExpanded(() => {
      if (allExpanded) return new Set();
      const ids = new Set<number>();
      const collect = (nodes: OutlineNode[]) => {
        for (const node of nodes) {
          if (node.children.length > 0) {
            ids.add(node.section.id);
            collect(node.children);
          }
        }
      };
      collect(tree);
      return ids;
    });
  }, [allExpanded, tree]);

  return (
    <aside className="absolute inset-y-0 left-0 z-40 w-80 max-w-[calc(100%-2rem)] border-r bg-background shadow-xl">
      <div className="flex h-full flex-col">
        <div className="flex items-center justify-between gap-2 border-b px-3 py-2">
          <div className="min-w-0">
            <div className="flex items-center gap-2 text-sm font-semibold">
              <ListTree className="size-4 text-muted-foreground" />
              目录
            </div>
          </div>
          <div className="flex items-center gap-1">
            {hasNested && (
              <Button
                type="button"
                variant="ghost"
                size="xs"
                className="text-xs text-muted-foreground"
                onClick={toggleAll}
              >
                {allExpanded ? "全部收起" : "全部展开"}
              </Button>
            )}
            <Button type="button" variant="ghost" size="icon-xs" aria-label="关闭目录" onClick={onClose}>
              <X className="size-3.5" />
            </Button>
          </div>
        </div>
        <ScrollArea className="min-h-0 flex-1">
          <div className="space-y-0.5 p-2">
            {tree.length === 0 ? (
              <div className="rounded-md border border-dashed bg-muted/30 p-3 text-sm text-muted-foreground">
                暂无目录信息
              </div>
            ) : (
              tree.map((node) => (
                <OutlineTreeNode
                  key={node.section.id}
                  node={node}
                  depth={0}
                  activeId={activeId}
                  activeTrail={activeTrail}
                  expanded={expanded}
                  onToggle={toggle}
                  onGoToEntry={onGoToEntry}
                />
              ))
            )}
          </div>
        </ScrollArea>
      </div>
    </aside>
  );
}

function ReaderLeftRail({
  outlineOpen,
  onToggleOutline,
}: {
  outlineOpen: boolean;
  onToggleOutline: () => void;
}) {
  if (outlineOpen) return null;
  return (
    <div className="absolute left-3 top-5 z-30 flex flex-col gap-2">
      <ToolbarButton label="打开目录" active={outlineOpen} onClick={onToggleOutline}>
        <ListTree className="size-4" />
      </ToolbarButton>
    </div>
  );
}

function TranslationPanel({
  translation,
  onClear,
}: {
  translation: TranslationResult | null;
  onClear: () => void;
}) {
  return (
    <section className="w-full shrink-0 border-b">
      <div className="flex items-center justify-between gap-2 border-b px-3 py-2">
        <div className="flex items-center gap-2 text-xs font-medium uppercase text-muted-foreground">
          <Languages className="size-3.5" />
          选段翻译
        </div>
        {translation && (
          <Button type="button" variant="ghost" size="icon-xs" aria-label="清空翻译" onClick={onClear}>
            <X className="size-3.5" />
          </Button>
        )}
      </div>
      <div className="p-3 text-sm">
        {!translation ? (
          <div className="rounded-md border border-dashed bg-muted/30 p-3 text-muted-foreground">
            选中 PDF 原文后点击翻译。
          </div>
        ) : (
          <div className="space-y-3">
            <div>
              <div className="mb-1 text-xs text-muted-foreground">原文 · p.{translation.pageNo}</div>
              <div className="line-clamp-3 rounded-md bg-muted p-2 text-muted-foreground">
                {translation.original}
              </div>
            </div>
            <div className="rounded-md border bg-background p-3 leading-6">
              {translation.loading ? (
                <span className="inline-flex items-center gap-2 text-muted-foreground">
                  <Loader2 className="size-4 animate-spin" />
                  翻译中...
                </span>
              ) : translation.error ? (
                <span className="text-destructive">{translation.error}</span>
              ) : (
                translation.translation
              )}
            </div>
          </div>
        )}
      </div>
    </section>
  );
}

function AnnotationCard({
  annotation,
  busy,
  translating,
  translationError,
  textExpanded,
  transExpanded,
  noteExpanded,
  onToggleTextExpanded,
  onToggleTransExpanded,
  onToggleNoteExpanded,
  onDelete,
  onColorChange,
  onRetryTranslate,
  onLocate,
}: {
  annotation: PaperAnnotation;
  busy: string;
  translating: boolean;
  translationError?: string;
  textExpanded: boolean;
  transExpanded: boolean;
  noteExpanded: boolean;
  onToggleTextExpanded: (annotation: PaperAnnotation) => void;
  onToggleTransExpanded: (annotation: PaperAnnotation) => void;
  onToggleNoteExpanded: (annotation: PaperAnnotation) => void;
  onDelete: (annotation: PaperAnnotation) => void;
  onColorChange: (annotation: PaperAnnotation, color: AnnotationColor) => void;
  onRetryTranslate: (annotation: PaperAnnotation) => void;
  onLocate: (annotation: PaperAnnotation) => void;
}) {
  const color = (annotation.color as AnnotationColor) || "yellow";
  return (
    <article className="rounded-md border bg-background p-3 shadow-sm">
      <div className="flex items-center justify-between gap-2">
        <button
          type="button"
          onClick={() => onLocate(annotation)}
          className="flex min-w-0 items-center gap-2 text-left text-sm font-semibold"
        >
          <span
            className={cn(
              "size-3 shrink-0 rounded-sm",
              COLOR_META[color]?.className || COLOR_META.yellow.className,
            )}
          />
          <span className="truncate">页 {annotation.page_no}</span>
        </button>
        <DropdownMenu>
          <DropdownMenuTrigger
            render={
              <Button type="button" variant="ghost" size="icon-xs" aria-label="批注操作">
                {busy === `delete-${annotation.id}` ? (
                  <Loader2 className="size-3.5 animate-spin" />
                ) : (
                  <MoreHorizontal className="size-3.5" />
                )}
              </Button>
            }
          />
          <DropdownMenuContent align="end" className="w-44">
            <DropdownMenuItem onClick={() => onLocate(annotation)}>跳转到原文</DropdownMenuItem>
            <DropdownMenuItem onClick={() => onRetryTranslate(annotation)}>
              {annotation.translation ? "重新翻译" : "翻译"}
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem
              disabled={busy === `delete-${annotation.id}`}
              onClick={() => onDelete(annotation)}
              variant="destructive"
            >
              删除
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
      <div className="mt-3 space-y-3 text-sm">
        <button
          type="button"
          aria-expanded={textExpanded}
          onClick={() => onToggleTextExpanded(annotation)}
          className={cn(
            "w-full rounded-md bg-muted px-3 py-2 text-left leading-6 text-muted-foreground transition hover:bg-muted/80",
            !textExpanded && "line-clamp-3",
          )}
        >
          {annotation.text}
        </button>
        <div className="rounded-md border bg-muted/20 p-2 leading-6">
          {translating ? (
            <span className="inline-flex items-center gap-2 text-muted-foreground">
              <Loader2 className="size-3.5 animate-spin" />
              正在自动翻译
            </span>
          ) : translationError ? (
            <div className="flex items-center justify-between gap-2 text-destructive">
              <span className="min-w-0">{translationError}</span>
              <Button
                type="button"
                size="xs"
                variant="ghost"
                onClick={() => onRetryTranslate(annotation)}
              >
                重试
              </Button>
            </div>
          ) : annotation.translation ? (
            <button
              type="button"
              aria-expanded={transExpanded}
              onClick={() => onToggleTransExpanded(annotation)}
              className={cn(
                "w-full text-left leading-6 transition",
                !transExpanded && "line-clamp-3",
              )}
            >
              {annotation.translation}
            </button>
          ) : (
            <div className="flex items-center justify-between gap-2 text-muted-foreground">
              <span>暂无译文</span>
              <Button type="button" size="xs" variant="ghost" onClick={() => onRetryTranslate(annotation)}>
                翻译
              </Button>
            </div>
          )}
        </div>
        {annotation.note && (
          <button
            type="button"
            aria-expanded={noteExpanded}
            onClick={() => onToggleNoteExpanded(annotation)}
            className={cn(
              "w-full rounded-md bg-muted/40 px-3 py-2 text-left leading-6 transition hover:bg-muted/60",
              !noteExpanded && "line-clamp-3",
            )}
          >
            {annotation.note}
          </button>
        )}
        <div className="flex items-center justify-between gap-3">
          <span className="text-xs text-muted-foreground">颜色</span>
          <ColorSwatches
            value={annotation.color}
            disabled={busy === `color-${annotation.id}`}
            onChange={(nextColor) => onColorChange(annotation, nextColor)}
            compact
          />
        </div>
      </div>
    </article>
  );
}

function AnnotationPanel({
  annotations,
  busy,
  translatingIDs,
  translationErrors,
  textExpandedIDs,
  transExpandedIDs,
  noteExpandedIDs,
  onToggleTextExpanded,
  onToggleTransExpanded,
  onToggleNoteExpanded,
  onDelete,
  onColorChange,
  onRetryTranslate,
  onLocateAnnotation,
}: {
  annotations: PaperAnnotation[];
  busy: string;
  translatingIDs: Set<number>;
  translationErrors: Record<number, string>;
  textExpandedIDs: Set<number>;
  transExpandedIDs: Set<number>;
  noteExpandedIDs: Set<number>;
  onToggleTextExpanded: (annotation: PaperAnnotation) => void;
  onToggleTransExpanded: (annotation: PaperAnnotation) => void;
  onToggleNoteExpanded: (annotation: PaperAnnotation) => void;
  onDelete: (annotation: PaperAnnotation) => void;
  onColorChange: (annotation: PaperAnnotation, color: AnnotationColor) => void;
  onRetryTranslate: (annotation: PaperAnnotation) => void;
  onLocateAnnotation: (annotation: PaperAnnotation) => void;
}) {
  return (
    <section className="flex min-h-0 w-full flex-1 flex-col">
      <div className="flex items-center justify-between gap-2 border-b px-3 py-2">
        <div className="flex items-center gap-2 text-xs font-medium uppercase text-muted-foreground">
          <BookMarked className="size-3.5" />
          高亮与批注
        </div>
        <span className="rounded-full bg-muted px-2 py-0.5 text-xs text-muted-foreground">
          {annotations.length}
        </span>
      </div>
      <ScrollArea className="min-h-0 w-full flex-1">
        <div className="w-full space-y-3 p-3">
          {annotations.length === 0 ? (
            <div className="rounded-md border border-dashed bg-muted/30 p-4 text-sm text-muted-foreground">
              选中 PDF 原文后点击高亮或批注。
            </div>
          ) : (
            annotations.map((annotation) => (
              <AnnotationCard
                key={annotation.id}
                annotation={annotation}
                busy={busy}
                translating={translatingIDs.has(annotation.id)}
                translationError={translationErrors[annotation.id]}
                textExpanded={textExpandedIDs.has(annotation.id)}
                transExpanded={transExpandedIDs.has(annotation.id)}
                noteExpanded={noteExpandedIDs.has(annotation.id)}
                onToggleTextExpanded={onToggleTextExpanded}
                onToggleTransExpanded={onToggleTransExpanded}
                onToggleNoteExpanded={onToggleNoteExpanded}
                onDelete={onDelete}
                onColorChange={onColorChange}
                onRetryTranslate={onRetryTranslate}
                onLocate={onLocateAnnotation}
              />
            ))
          )}
        </div>
      </ScrollArea>
    </section>
  );
}

function ReaderSidePanel({
  showTranslation,
  showAnnotations,
  translation,
  annotations,
  busy,
  translatingIDs,
  translationErrors,
  expandedAnnotationTextIDs,
  expandedAnnotationTransIDs,
  expandedAnnotationNoteIDs,
  onClearTranslation,
  onToggleAnnotationTextExpanded,
  onToggleAnnotationTransExpanded,
  onToggleAnnotationNoteExpanded,
  onDelete,
  onColorChange,
  onRetryTranslate,
  onLocateAnnotation,
}: {
  showTranslation: boolean;
  showAnnotations: boolean;
  translation: TranslationResult | null;
  annotations: PaperAnnotation[];
  busy: string;
  translatingIDs: Set<number>;
  translationErrors: Record<number, string>;
  expandedAnnotationTextIDs: Set<number>;
  expandedAnnotationTransIDs: Set<number>;
  expandedAnnotationNoteIDs: Set<number>;
  onClearTranslation: () => void;
  onToggleAnnotationTextExpanded: (annotation: PaperAnnotation) => void;
  onToggleAnnotationTransExpanded: (annotation: PaperAnnotation) => void;
  onToggleAnnotationNoteExpanded: (annotation: PaperAnnotation) => void;
  onDelete: (annotation: PaperAnnotation) => void;
  onColorChange: (annotation: PaperAnnotation, color: AnnotationColor) => void;
  onRetryTranslate: (annotation: PaperAnnotation) => void;
  onLocateAnnotation: (annotation: PaperAnnotation) => void;
}) {
  return (
    <aside
      className="h-full min-h-0 w-full self-stretch overflow-hidden border-l bg-background"
      style={{ width: RIGHT_PANEL_WIDTH, minWidth: RIGHT_PANEL_WIDTH, maxWidth: RIGHT_PANEL_WIDTH }}
    >
      <div className="flex h-full min-h-0 flex-col">
        <div className="border-b px-3 py-2">
          <div className="text-sm font-semibold">研读面板</div>
          <div className="text-xs text-muted-foreground">翻译 · 批注</div>
        </div>
        {showTranslation && (
          <TranslationPanel translation={translation} onClear={onClearTranslation} />
        )}
        {showAnnotations && (
          <AnnotationPanel
            annotations={annotations}
            busy={busy}
            translatingIDs={translatingIDs}
            translationErrors={translationErrors}
            textExpandedIDs={expandedAnnotationTextIDs}
            transExpandedIDs={expandedAnnotationTransIDs}
            noteExpandedIDs={expandedAnnotationNoteIDs}
            onToggleTextExpanded={onToggleAnnotationTextExpanded}
            onToggleTransExpanded={onToggleAnnotationTransExpanded}
            onToggleNoteExpanded={onToggleAnnotationNoteExpanded}
            onDelete={onDelete}
            onColorChange={onColorChange}
            onRetryTranslate={onRetryTranslate}
            onLocateAnnotation={onLocateAnnotation}
          />
        )}
      </div>
    </aside>
  );
}

function refsFromMeta(meta?: Record<string, unknown>): Reference[] {
  const raw = meta?.sources;
  return Array.isArray(raw) ? (raw as Reference[]) : [];
}

function figuresFromRefs(refs: Reference[]): Record<string, string> {
  const map: Record<string, string> = {};
  for (const ref of refs) {
    if (ref.block_type === "image" && ref.img_name && ref.doc_id) {
      map[ref.img_name] = ref.doc_id;
    }
  }
  return map;
}

function readerSourceLabel(ref: Reference, index: number) {
  if (ref.block_type === "selection") {
    return ref.page_no ? `选段 p.${ref.page_no}` : "选段";
  }
  if (ref.fallback_scope === "paper" && ref.page_no) {
    return `全文补充 p.${ref.page_no}`;
  }
  if (ref.page_no) return `p.${ref.page_no}`;
  return `来源 ${index + 1}`;
}

function ReaderQASources({ refs }: { refs: Reference[] }) {
  if (refs.length === 0) return null;
  return (
    <div className="mt-3 flex flex-wrap gap-1.5">
      {refs.slice(0, 6).map((ref, index) => {
        const selectionRef = ref.block_type === "selection";
        return (
          <span
            key={`${ref.id ?? index}`}
            className={cn(
              "rounded-full border px-2 py-0.5 text-[11px]",
              selectionRef
                ? "border-primary/25 bg-primary/5 text-primary"
                : "bg-muted/40 text-muted-foreground",
            )}
          >
            {readerSourceLabel(ref, index)}
          </span>
        );
      })}
    </div>
  );
}

function ReaderQAMessage({ message }: { message: Message }) {
  const assistant = message.role === "assistant";
  const refs = assistant ? refsFromMeta(message.meta) : [];
  return (
    <article className={cn("flex flex-col gap-1.5", assistant ? "items-start" : "items-end")}>
      {assistant ? (
        <div className="w-full text-sm leading-6">
          <Markdown figures={figuresFromRefs(refs)}>{message.content}</Markdown>
          <ReaderQASources refs={refs} />
        </div>
      ) : (
        <div className="max-w-[84%] rounded-2xl bg-primary px-3 py-2 text-sm leading-6 text-primary-foreground">
          {message.content}
        </div>
      )}
      <span className="px-0.5 text-[11px] text-muted-foreground">
        {assistant ? "小耄耋" : "我"}
      </span>
    </article>
  );
}

function readerSelectionPreview(text: string) {
  const normalized = text.replace(/\s+/g, " ").trim();
  const runes = Array.from(normalized);
  if (runes.length <= QA_SELECTION_PREVIEW_RUNES) return normalized;
  return `${runes.slice(0, QA_SELECTION_PREVIEW_RUNES).join("")}...`;
}

const MAODIE_PROMPT_HINTS = [
  "解释一下当前页的这段内容",
  "这里的方法步骤是什么?",
  "这个结论有什么依据?",
];

interface ReaderQASelection {
  text: string;
  pageNo: number;
}

type ReaderQAScope = "selection" | "page" | "paper";

function isAbortError(err: unknown) {
  return err instanceof Error && err.name === "AbortError";
}

function ReaderQAPanel({
  paper,
  currentPage,
  selection,
  onClearSelection,
  onClose,
}: {
  paper: Paper | null;
  currentPage: number;
  selection: ReaderQASelection | null;
  onClearSelection: () => void;
  onClose: () => void;
}) {
  const [sessionID, setSessionID] = useState("");
  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState("");
  const [sending, setSending] = useState(false);
  const [error, setError] = useState("");
  const [scope, setScope] = useState<ReaderQAScope>(selection ? "selection" : "page");
  const bottomRef = useRef<HTMLDivElement | null>(null);
  const abortRef = useRef<AbortController | null>(null);
  const mountedRef = useRef(true);
  const localIDSeq = useRef(0);
  const paperID = paper?.id || "";
  const hasDraft = input.trim().length > 0;
  const effectiveScope: ReaderQAScope = scope === "selection" && !selection ? "page" : scope;
  const contextPage = effectiveScope === "selection" ? selection?.pageNo || currentPage : currentPage;
  const selectedText = selection?.text || "";
  const selectionPreview = selection ? readerSelectionPreview(selection.text) : "";
  const contextLabel = effectiveScope === "paper" ? "全文" : `p.${contextPage || "-"}`;

  useEffect(() => {
    // StrictMode 会先跑一轮 setup→cleanup 再真正挂载,setup 必须把标记写回 true,
    // 否则 cleanup 置 false 后所有 mountedRef 守卫的状态更新永久失效(答案不落地、spinner 不停)。
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
      abortRef.current?.abort();
    };
  }, []);

  useEffect(() => {
    if (selection) {
      setScope("selection");
    } else {
      setScope((cur) => (cur === "selection" ? "page" : cur));
    }
  }, [selection]);

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: "instant" });
  }, [messages, sending]);

  useEffect(() => {
    let cancelled = false;
    abortRef.current?.abort();
    abortRef.current = null;
    setSessionID("");
    setMessages([]);
    setError("");
    setSending(false);
    if (!paperID) return;
    api
      .listSessions()
      .then((sessions) => {
        if (cancelled) return null;
        const session = sessions
          .filter((item) => item.paper_id === paperID && item.agent_type === "maodie")
          .sort((a, b) => new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime())[0];
        if (!session) return null;
        setSessionID(session.id);
        return api.listMessages(session.id);
      })
      .then((list) => {
        if (!cancelled && Array.isArray(list)) setMessages(list);
      })
      .catch((err) => {
        if (!cancelled) setError((err as Error)?.message || "加载小耄耋会话失败");
      });
    return () => {
      cancelled = true;
    };
  }, [paperID]);

  const ensureSession = async () => {
    if (sessionID) return sessionID;
    if (!paperID) throw new Error("缺少论文 id");
    const session = await api.createSession(`${paperName(paper)} 小耄耋`, paperID, "maodie");
    setSessionID(session.id);
    return session.id;
  };

  const startNewSession = async () => {
    if (!paperID) return;
    abortRef.current?.abort();
    abortRef.current = null;
    setSending(false);
    setError("");
    setInput("");
    setMessages([]);
    localIDSeq.current = 0;
    try {
      const session = await api.createSession(`${paperName(paper)} 小耄耋`, paperID, "maodie");
      if (!mountedRef.current) return;
      setSessionID(session.id);
    } catch (err) {
      if (!mountedRef.current) return;
      setError((err as Error)?.message || "新建小耄耋会话失败");
    }
  };

  const readerContextForSubmit = (): ReaderContext => {
    if (effectiveScope === "paper") {
      return { scope: "paper" };
    }
    if (effectiveScope === "selection" && selection) {
      return {
        scope: "selection",
        page_no: selection.pageNo,
        selected_text: selectedText,
      };
    }
    return {
      scope: "page",
      page_no: currentPage,
    };
  };

  const closePanel = () => {
    abortRef.current?.abort();
    abortRef.current = null;
    onClose();
  };

  const submit = async (draft?: string) => {
    const query = (draft ?? input).trim();
    if (!query || sending) return;
    setInput("");
    setError("");
    setSending(true);
    let placeholderID = "";
    const controller = new AbortController();
    abortRef.current = controller;
    try {
      const sid = await ensureSession();
      localIDSeq.current += 1;
      const localUserID = `local-${localIDSeq.current}`;
      placeholderID = `stream-${localIDSeq.current}`;
      const createdAt = new Date().toISOString();
      const localUser: Message = {
        id: localUserID,
        session_id: sid,
        role: "user",
        content: query,
        created_at: createdAt,
      };
      const localAssistant: Message = {
        id: placeholderID,
        session_id: sid,
        role: "assistant",
        content: "",
        intent: "maodie",
        created_at: createdAt,
        streaming: true,
      };
      setMessages((list) => [...list, localUser, localAssistant]);
      const readerContext = readerContextForSubmit();
      const data = await api.sendMessage(sid, query, undefined, {
        onDelta: (text, reset) => {
          if (!mountedRef.current || controller.signal.aborted) return;
          setMessages((list) =>
            list.map((message) =>
              message.id === placeholderID
                ? { ...message, content: reset ? text : message.content + text }
                : message,
            ),
          );
        },
      }, readerContext, controller.signal);
      if (!mountedRef.current || controller.signal.aborted) return;
      setMessages((list) =>
        list.map((message) =>
          message.id === placeholderID
            ? { ...data.message, id: data.message.id || placeholderID, meta: data.meta ?? data.message.meta }
            : message,
        ),
      );
    } catch (err) {
      if (isAbortError(err)) return;
      if (!mountedRef.current) return;
      if (placeholderID) {
        setMessages((list) => list.filter((message) => message.id !== placeholderID));
      }
      setError((err as Error)?.message || "小耄耋应答失败");
    } finally {
      if (abortRef.current === controller) {
        abortRef.current = null;
      }
      if (mountedRef.current) {
        setSending(false);
      }
    }
  };

  return (
    <aside
      className="h-full min-h-0 w-full self-stretch overflow-hidden border-l bg-background"
      style={{ width: RIGHT_PANEL_WIDTH, minWidth: RIGHT_PANEL_WIDTH, maxWidth: RIGHT_PANEL_WIDTH }}
    >
      <div className="flex h-full min-h-0 flex-col">
        <div className="flex items-center justify-between gap-2 border-b px-3 py-2">
          <div className="min-w-0">
            <div className="flex items-center gap-2 text-sm font-semibold">
              <MessageSquarePlus className="size-4 text-muted-foreground" />
              小耄耋
            </div>
            <div className="truncate text-xs text-muted-foreground">
              {contextLabel} · {paperName(paper)}
            </div>
          </div>
          <div className="flex shrink-0 items-center gap-1">
            <Button
              type="button"
              variant="ghost"
              size="icon-xs"
              aria-label="新开对话"
              title="新开对话"
              onClick={() => void startNewSession()}
            >
              <Plus className="size-3.5" />
            </Button>
            <Button type="button" variant="ghost" size="icon-xs" aria-label="关闭问答" onClick={closePanel}>
              <X className="size-3.5" />
            </Button>
          </div>
        </div>
        <ScrollArea className="min-h-0 flex-1">
          <div className="space-y-5 px-4 py-4">
            {messages.length === 0 ? (
              <div className="space-y-3">
	                <div className="rounded-md border border-dashed bg-muted/30 p-3 text-sm text-muted-foreground">
	                  {effectiveScope === "paper"
	                    ? "已切到全文。"
	                    : effectiveScope === "selection"
	                      ? "已绑定当前选段。"
	                      : "默认绑定当前页。"}
	                </div>
                <div className="flex flex-wrap gap-2">
                  {MAODIE_PROMPT_HINTS.map((hint) => (
                    <Button key={hint} type="button" size="sm" variant="outline" onClick={() => void submit(hint)}>
                      {hint}
                    </Button>
                  ))}
                </div>
              </div>
            ) : (
              messages.map((message) => <ReaderQAMessage key={String(message.id)} message={message} />)
            )}
            {sending && !messages.some((message) => message.streaming) && (
              <div className="inline-flex items-center gap-2 rounded-xl border bg-card px-3 py-2 text-sm text-muted-foreground">
                <Loader2 className="size-4 animate-spin" />
                小耄耋正在阅读当前上下文…
              </div>
            )}
            {error && <div className="rounded-md border border-destructive/30 bg-background p-3 text-sm text-destructive">{error}</div>}
            <div ref={bottomRef} />
          </div>
        </ScrollArea>
        <form
          className="shrink-0 border-t px-3 py-3"
          onSubmit={(event) => {
            event.preventDefault();
            void submit();
          }}
        >
          <div className="mb-2 grid grid-cols-3 rounded-lg border bg-muted/30 p-0.5 text-xs">
            {([
              ["selection", "选段"],
              ["page", "本页"],
              ["paper", "全文"],
            ] as const).map(([value, label]) => {
              const disabled = value === "selection" && !selection;
              const active = effectiveScope === value;
              return (
                <button
                  key={value}
                  type="button"
                  disabled={disabled}
                  className={cn(
                    "h-7 rounded-md px-2 font-medium text-muted-foreground transition-colors disabled:cursor-not-allowed disabled:opacity-40",
                    active && "bg-background text-foreground shadow-sm",
                  )}
                  onClick={() => setScope(value)}
                >
                  {label}
                </button>
              );
            })}
          </div>
          {selection && (
            <div className="mb-2 flex max-w-full items-center gap-2 rounded-lg border border-primary/20 bg-primary/5 px-2 py-1.5 text-xs">
              <span className="shrink-0 rounded-md bg-background/80 px-1.5 py-0.5 font-medium text-primary">
                p.{selection.pageNo}
              </span>
              <span className="min-w-0 flex-1 truncate text-muted-foreground">{selectionPreview}</span>
              <Button
                type="button"
                variant="ghost"
                size="icon-xs"
                className="-mr-1 size-6 text-muted-foreground hover:text-foreground"
                aria-label="取消选段绑定"
                onClick={() => {
                  onClearSelection();
                  setScope("page");
                }}
              >
                <X className="size-3.5" />
              </Button>
            </div>
          )}
          <div className="flex items-end gap-2 rounded-[1.4rem] border bg-card py-1.5 pl-2 pr-1.5 shadow-sm focus-within:border-ring/50">
            <Textarea
              rows={1}
              value={input}
              disabled={sending}
              className="max-h-32 min-h-9 resize-none border-0 bg-transparent px-3 py-1.5 text-sm leading-6 shadow-none focus-visible:ring-0"
              onChange={(event) => setInput(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === "Enter" && !event.shiftKey) {
                  event.preventDefault();
                  void submit();
                }
              }}
            />
            {(hasDraft || sending) && (
              <Button
                type="submit"
                size="icon"
                className="size-9 shrink-0 rounded-full"
                disabled={sending || !hasDraft}
                aria-label={sending ? "正在发送" : "发送"}
              >
                {sending ? <Loader2 className="size-4 animate-spin" /> : <ArrowUp className="size-4 stroke-[2.6]" />}
              </Button>
            )}
          </div>
        </form>
      </div>
    </aside>
  );
}

function ReaderPdfLoader({
  document,
  beforeLoad,
  errorMessage,
  children,
}: {
  document: DocumentInitParameters;
  beforeLoad: (progress: OnProgressParameters | null) => ReactNode;
  errorMessage: (error: Error) => ReactNode;
  children: (pdfDocument: PDFDocumentProxy) => ReactNode;
}) {
  const [pdfDocument, setPdfDocument] = useState<PDFDocumentProxy | null>(null);
  const [loadingProgress, setLoadingProgress] = useState<OnProgressParameters | null>(null);
  const [error, setError] = useState<Error | null>(null);

  useEffect(() => {
    let cancelled = false;

    setError(null);
    setPdfDocument(null);
    setLoadingProgress(null);
    GlobalWorkerOptions.workerSrc = PDF_WORKER;

    const loadingTask = getDocument(document);
    loadingTask.onProgress = (progress: OnProgressParameters) => {
      if (cancelled) return;
      setLoadingProgress(progress.loaded > progress.total ? null : progress);
    };
    loadingTask.promise
      .then((loaded) => {
        if (cancelled) return;
        setPdfDocument(loaded);
      })
      .catch((err) => {
        if (cancelled) return;
        setError(toPdfError(err));
      })
      .finally(() => {
        if (!cancelled) setLoadingProgress(null);
      });

    return () => {
      cancelled = true;
      void loadingTask.destroy().catch(() => {});
    };
  }, [document]);

  if (error) return errorMessage(error);
  if (!pdfDocument) return beforeLoad(loadingProgress);
  return children(pdfDocument);
}

function SelectionToolbar({
  onTranslate,
  onHighlight,
  onAnnotate,
  onAsk,
}: {
  onTranslate: (selection: PdfSelection) => void;
  onHighlight: (selection: PdfSelection) => void;
  onAnnotate: (selection: PdfSelection) => void;
  onAsk: (selection: PdfSelection) => void;
}) {
  const utils = usePdfHighlighterContext();

  const applySelection = (fn: (selection: PdfSelection) => void) => {
    const selection = utils.getCurrentSelection();
    if (!selection?.content.text?.trim()) return;
    fn(selection);
    utils.setTip(null);
  };

  return (
    <div className="flex items-center gap-1 rounded-lg border bg-popover p-1 text-popover-foreground shadow-lg">
      <Button type="button" size="sm" variant="ghost" onClick={() => applySelection(onTranslate)}>
        <Languages className="size-3.5" />
        翻译
      </Button>
      <Button type="button" size="sm" variant="ghost" onClick={() => applySelection(onHighlight)}>
        <BookMarked className="size-3.5" />
        高亮
      </Button>
      <Button type="button" size="sm" variant="ghost" onClick={() => applySelection(onAnnotate)}>
        <NotebookPen className="size-3.5" />
        批注
      </Button>
      <Button type="button" size="sm" variant="ghost" onClick={() => applySelection(onAsk)}>
        <MessageCircleQuestionMark className="size-3.5" />
        问答
      </Button>
    </div>
  );
}

function HighlightTip({
  annotation,
  onDelete,
}: {
  annotation: PaperAnnotation;
  onDelete: (annotation: PaperAnnotation) => void;
}) {
  return (
    <div className="max-w-xs rounded-lg border bg-popover p-3 text-xs text-popover-foreground shadow-lg">
      <div className="line-clamp-3 leading-5">{annotation.text}</div>
      {annotation.translation && (
        <div className="mt-2 line-clamp-3 rounded-md bg-muted px-2 py-1.5 text-muted-foreground">
          {annotation.translation}
        </div>
      )}
      {annotation.note && (
        <div className="mt-2 rounded-md bg-muted px-2 py-1.5 text-muted-foreground">
          {annotation.note}
        </div>
      )}
      <div className="mt-2 flex items-center justify-between gap-2">
        <span className="text-muted-foreground">p.{annotation.page_no}</span>
        <Button type="button" size="xs" variant="ghost" onClick={() => onDelete(annotation)}>
          <Trash2 className="size-3" />
          删除
        </Button>
      </div>
    </div>
  );
}

function HighlightContainer({
  onDelete,
  locatedAnnotationId,
}: {
  onDelete: (annotation: PaperAnnotation) => void;
  locatedAnnotationId: number | null;
}) {
  const { highlight, isScrolledTo } = useHighlightContainerContext<ReaderHighlight>();
  const annotation = highlight.annotation;
  return (
    <MonitoredHighlightContainer
      highlightTip={{
        position: highlight.position,
        content: <HighlightTip annotation={annotation} onDelete={onDelete} />,
      }}
    >
      <TextHighlight
        highlight={highlight}
        isScrolledTo={isScrolledTo || locatedAnnotationId === annotation.id}
        highlightColor={colorValue(annotation.color)}
        copyText={annotation.text}
        onDelete={() => onDelete(annotation)}
      />
    </MonitoredHighlightContainer>
  );
}

function ReaderPdf({
  pdfUrl,
  highlights,
  initialPage,
  scaleValue,
  onPageChange,
  onPageCount,
  onTranslateSelection,
  onSaveHighlight,
  onAnnotateSelection,
  onAskSelection,
  onDeleteAnnotation,
  locatedAnnotationId,
  onDocumentReady,
  onUtilsReady,
}: {
  pdfUrl: string;
  highlights: ReaderHighlight[];
  initialPage: number;
  scaleValue: PdfScaleValue;
  onPageChange: (page: number) => void;
  onPageCount: (pages: number) => void;
  onTranslateSelection: (selection: PdfSelection) => void;
  onSaveHighlight: (selection: PdfSelection) => void;
  onAnnotateSelection: (selection: PdfSelection) => void;
  onAskSelection: (selection: PdfSelection) => void;
  onDeleteAnnotation: (annotation: PaperAnnotation) => void;
  locatedAnnotationId: number | null;
  onDocumentReady: (pdfDocument: PDFDocumentProxy) => void;
  onUtilsReady: (utils: PdfHighlighterUtils | null) => void;
}) {
  const utilsRef = useRef<PdfHighlighterUtils | null>(null);
  const restoredRef = useRef(false);
  const [utilsVersion, setUtilsVersion] = useState(0);
  const [pagesReady, setPagesReady] = useState(false);
  const pdfDocument = useMemo(() => ({ url: pdfUrl }), [pdfUrl]);

  const setUtils = useCallback(
    (utils: PdfHighlighterUtils) => {
      if (utilsRef.current === utils) return;
      utilsRef.current = utils;
      onUtilsReady(utils);
      setUtilsVersion((v) => v + 1);
    },
    [onUtilsReady],
  );

  useEffect(() => {
    restoredRef.current = false;
    setPagesReady(false);
    return () => onUtilsReady(null);
  }, [onUtilsReady, pdfUrl]);

  useEffect(() => {
    const utils = utilsRef.current;
    if (!utils) return;
    const eventBus = utils.getEventBus();
    if (!isEventBus(eventBus)) return;
    const handler: EventBusCallback = (evt) => {
      const pageNumber =
        evt && typeof evt === "object" && "pageNumber" in evt
          ? (evt.pageNumber as unknown)
          : undefined;
      if (typeof pageNumber === "number" && pageNumber > 0) {
        onPageChange(pageNumber);
      }
    };
    eventBus.on("pagechanging", handler);
    eventBus.on("pagechange", handler);
    return () => {
      eventBus.off("pagechanging", handler);
      eventBus.off("pagechange", handler);
    };
  }, [onPageChange, utilsVersion]);

  useEffect(() => {
    const utils = utilsRef.current;
    if (!utils) return;
    const eventBus = utils.getEventBus();
    if (!isEventBus(eventBus)) return;
    const markPagesReady = () => setPagesReady(true);
    eventBus.on("pagesinit", markPagesReady);
    eventBus.on("pagesloaded", markPagesReady);
    if (utils.getViewer()?.pagesCount) {
      markPagesReady();
    }
    return () => {
      eventBus.off("pagesinit", markPagesReady);
      eventBus.off("pagesloaded", markPagesReady);
    };
  }, [utilsVersion]);

  useEffect(() => {
    if (restoredRef.current || initialPage <= 1 || !pagesReady) return;
    const utils = utilsRef.current;
    if (!utils) return;
    restoredRef.current = true;
    window.setTimeout(() => utils.goToPage(initialPage), 250);
  }, [initialPage, pagesReady, utilsVersion]);

  return (
    <ReaderPdfLoader
      document={pdfDocument}
      beforeLoad={() => (
        <div className="flex h-full items-center justify-center gap-2 text-sm text-muted-foreground">
          <Loader2 className="size-4 animate-spin" />
          加载 PDF...
        </div>
      )}
      errorMessage={(error) => (
        <div className="flex h-full items-center justify-center p-8 text-sm text-destructive">
          PDF 加载失败：{error.message}
        </div>
      )}
    >
      {(pdfDocument) => (
        <LoadedPdfHighlighter
          pdfDocument={pdfDocument}
          highlights={highlights}
          scaleValue={pagesReady ? scaleValue : "auto"}
          onPageCount={onPageCount}
          onTranslateSelection={onTranslateSelection}
          onSaveHighlight={onSaveHighlight}
          onAnnotateSelection={onAnnotateSelection}
          onAskSelection={onAskSelection}
          onDeleteAnnotation={onDeleteAnnotation}
          locatedAnnotationId={locatedAnnotationId}
          onDocumentReady={onDocumentReady}
          setUtils={setUtils}
        />
      )}
    </ReaderPdfLoader>
  );
}

function LoadedPdfHighlighter({
  pdfDocument,
  highlights,
  scaleValue,
  onPageCount,
  onTranslateSelection,
  onSaveHighlight,
  onAnnotateSelection,
  onAskSelection,
  onDeleteAnnotation,
  locatedAnnotationId,
  onDocumentReady,
  setUtils,
}: {
  pdfDocument: PDFDocumentProxy;
  highlights: ReaderHighlight[];
  scaleValue: PdfScaleValue;
  onPageCount: (pages: number) => void;
  onTranslateSelection: (selection: PdfSelection) => void;
  onSaveHighlight: (selection: PdfSelection) => void;
  onAnnotateSelection: (selection: PdfSelection) => void;
  onAskSelection: (selection: PdfSelection) => void;
  onDeleteAnnotation: (annotation: PaperAnnotation) => void;
  locatedAnnotationId: number | null;
  onDocumentReady: (pdfDocument: PDFDocumentProxy) => void;
  setUtils: (utils: PdfHighlighterUtils) => void;
}) {
  useEffect(() => {
    onPageCount(pdfDocument.numPages);
    onDocumentReady(pdfDocument);
  }, [onDocumentReady, onPageCount, pdfDocument]);

  return (
    <PdfHighlighter
      pdfDocument={pdfDocument}
      highlights={highlights}
      pdfScaleValue={scaleValue}
      enableAreaSelection={() => false}
      textSelectionColor="rgba(14, 165, 233, 0.22)"
      selectionTip={
        <SelectionToolbar
          onTranslate={onTranslateSelection}
          onHighlight={onSaveHighlight}
          onAnnotate={onAnnotateSelection}
          onAsk={onAskSelection}
        />
      }
      utilsRef={setUtils}
      theme={{
        mode: "light",
        containerBackgroundColor: "oklch(0.96 0.003 230)",
        scrollbarThumbColor: "oklch(0.72 0.01 230)",
        scrollbarTrackColor: "oklch(0.92 0.004 230)",
      }}
      style={{ height: "100%" }}
    >
      <HighlightContainer onDelete={onDeleteAnnotation} locatedAnnotationId={locatedAnnotationId} />
    </PdfHighlighter>
  );
}

export function ReaderClient() {
  const { activePaperID, selectPaper } = useApp();
  const id = useMemo(() => {
    if (typeof location === "undefined") return "";
    return new URLSearchParams(location.search).get("id") || "";
  }, []);
  const requestedPage = useMemo(() => loadRequestedPage(), []);
  const [ready, setReady] = useState(false);
  const [paper, setPaper] = useState<Paper | null>(null);
  const [sections, setSections] = useState<PaperSection[]>([]);
  const [numPages, setNumPages] = useState(0);
  const [currentPage, setCurrentPage] = useState(1);
  const [pageDraft, setPageDraft] = useState("1");
  const [scaleValue, setScaleValue] = useState<PdfScaleValue>(1);
  const [error, setError] = useState("");
  const [annotations, setAnnotations] = useState<PaperAnnotation[]>([]);
  const [translation, setTranslation] = useState<TranslationResult | null>(null);
  const [qaSelection, setQASelection] = useState<ReaderQASelection | null>(null);
  const [translationErrors, setTranslationErrors] = useState<Record<number, string>>({});
  const [translatingIDs, setTranslatingIDs] = useState<Set<number>>(() => new Set());
  const [expandedAnnotationTextIDs, setExpandedAnnotationTextIDs] = useState<Set<number>>(() => new Set());
  const [expandedAnnotationTransIDs, setExpandedAnnotationTransIDs] = useState<Set<number>>(() => new Set());
  const [expandedAnnotationNoteIDs, setExpandedAnnotationNoteIDs] = useState<Set<number>>(() => new Set());
  const [mindMapPanelWidth, setMindMapPanelWidth] = useState(MIND_MAP_PANEL_DEFAULT_WIDTH);
  const [noteOpen, setNoteOpen] = useState(false);
  const [noteDraft, setNoteDraft] = useState("");
  const [noteColor, setNoteColor] = useState<AnnotationColor>("yellow");
  const [pendingSelection, setPendingSelection] = useState<PdfSelection | null>(null);
  const [busy, setBusy] = useState("");
  const [prefs, setPrefs] = useState<ReaderPreferences>(() => loadReaderPreferences());
  const [pdfUtils, setPdfUtils] = useState<PdfHighlighterUtils | null>(null);
  const [outlineActiveSectionId, setOutlineActiveSectionId] = useState<number | null>(null);
  const [locatedAnnotationId, setLocatedAnnotationId] = useState<number | null>(null);
  const translateSeq = useRef(0);
  const progressLoadedRef = useRef(false);
  const outlineFallbackTriedRef = useRef(false);
  const outlineJumpRef = useRef<{ sectionId: number; pageNo: number; ignoreUntil: number } | null>(null);
  const locatedScrollTimerRef = useRef<number | null>(null);
  const locatedScrollCleanupRef = useRef<(() => void) | null>(null);
  const mindMapGridRef = useRef<HTMLDivElement | null>(null);
  const pdfWheelRef = useRef<HTMLDivElement | null>(null);

  const highlights = useMemo(() => annotations.map(annotationToHighlight), [annotations]);
  const initialPage = Math.max(1, requestedPage || paper?.last_read_page || 1);
  const pdfUrl = useMemo(() => (ready && id ? api.paperFileUrl(id) : ""), [ready, id]);
  const qaPanelOpen = !prefs.mindMapOpen && prefs.qaOpen;
  const rightPanelOpen = !prefs.mindMapOpen && !prefs.qaOpen && (prefs.translateOpen || prefs.annotationsOpen);
  const sidePanelOpen = qaPanelOpen || rightPanelOpen;
  const gridClass =
    prefs.mindMapOpen
      ? "lg:grid-cols-[minmax(0,1fr)_var(--mind-map-panel-width)]"
      : sidePanelOpen
        ? "lg:grid-cols-[minmax(0,1fr)_24rem]"
        : "lg:grid-cols-[minmax(0,1fr)]";
  const gridStyle = prefs.mindMapOpen
    ? ({ "--mind-map-panel-width": `${mindMapPanelWidth}px` } as CSSProperties)
    : undefined;

  const updatePrefs = useCallback((patch: Partial<ReaderPreferences>) => {
    setPrefs((cur) => ({ ...cur, ...patch }));
  }, []);

  const toggleTranslatePanel = useCallback(() => {
    setPrefs((cur) => {
      const translateOpen = !cur.translateOpen;
      return {
        ...cur,
        translateOpen,
        mindMapOpen: translateOpen ? false : cur.mindMapOpen,
        qaOpen: translateOpen ? false : cur.qaOpen,
      };
    });
  }, []);

  const toggleAnnotationsPanel = useCallback(() => {
    setPrefs((cur) => {
      const annotationsOpen = !cur.annotationsOpen;
      return {
        ...cur,
        annotationsOpen,
        mindMapOpen: annotationsOpen ? false : cur.mindMapOpen,
        qaOpen: annotationsOpen ? false : cur.qaOpen,
      };
    });
  }, []);

  const toggleMindMapPanel = useCallback(() => {
    setPrefs((cur) => {
      const mindMapOpen = !cur.mindMapOpen;
      return {
        ...cur,
        mindMapOpen,
        translateOpen: mindMapOpen ? false : cur.translateOpen,
        annotationsOpen: mindMapOpen ? false : cur.annotationsOpen,
        qaOpen: mindMapOpen ? false : cur.qaOpen,
      };
    });
  }, []);

  const toggleQAPanel = useCallback(() => {
    setQASelection(null);
    setPrefs((cur) => {
      const qaOpen = !cur.qaOpen;
      return {
        ...cur,
        qaOpen,
        mindMapOpen: qaOpen ? false : cur.mindMapOpen,
        translateOpen: qaOpen ? false : cur.translateOpen,
        annotationsOpen: qaOpen ? false : cur.annotationsOpen,
      };
    });
  }, []);

  const startMindMapResize = useCallback(
    (event: ReactPointerEvent<HTMLDivElement>) => {
      event.preventDefault();
      const startX = event.clientX;
      const startWidth = mindMapPanelWidth;
      const previousCursor = document.body.style.cursor;
      const previousSelect = document.body.style.userSelect;
      let frame = 0;
      let nextWidth = startWidth;

      const onMove = (moveEvent: PointerEvent) => {
        nextWidth = clamp(
          startWidth + startX - moveEvent.clientX,
          MIND_MAP_PANEL_MIN_WIDTH,
          MIND_MAP_PANEL_MAX_WIDTH,
        );
        if (frame) return;
        frame = window.requestAnimationFrame(() => {
          mindMapGridRef.current?.style.setProperty("--mind-map-panel-width", `${nextWidth}px`);
          frame = 0;
        });
      };
      const onUp = () => {
        if (frame) {
          window.cancelAnimationFrame(frame);
          frame = 0;
        }
        mindMapGridRef.current?.style.setProperty("--mind-map-panel-width", `${nextWidth}px`);
        setMindMapPanelWidth(nextWidth);
        document.body.style.cursor = previousCursor;
        document.body.style.userSelect = previousSelect;
        window.removeEventListener("pointermove", onMove);
        window.removeEventListener("pointerup", onUp);
      };

      document.body.style.cursor = "col-resize";
      document.body.style.userSelect = "none";
      window.addEventListener("pointermove", onMove);
      window.addEventListener("pointerup", onUp, { once: true });
    },
    [mindMapPanelWidth],
  );

  useEffect(() => {
    localStorage.setItem(READER_PREFS_KEY, JSON.stringify(prefs));
  }, [prefs]);

  useEffect(() => {
    setPageDraft(String(currentPage));
  }, [currentPage]);

  function closeReader() {
    window.close();
    window.setTimeout(() => {
      if (!window.closed) location.href = "/";
    }, 120);
  }

  useEffect(() => {
    if (!id || activePaperID === id) return;
    selectPaper(id);
  }, [activePaperID, id, selectPaper]);

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
    outlineFallbackTriedRef.current = false;
    setSections([]);
    setReady(true);
    Promise.all([api.paperDetail(id), api.listAnnotations(id)])
      .then(([detail, list]) => {
        const initialSections = Array.isArray(detail.sections) ? detail.sections : [];
        setPaper(detail.paper);
        setSections((cur) => (initialSections.length > 0 ? initialSections : cur));
        setCurrentPage(Math.max(1, requestedPage || detail.paper.last_read_page || 1));
        if (detail.paper.page_count > 0) setNumPages(detail.paper.page_count);
        setAnnotations(Array.isArray(list) ? list : []);
        progressLoadedRef.current = true;
        if (initialSections.length === 0) {
          void api
            .rebuildPaperSections(id)
            .then((rebuilt) => {
              if (Array.isArray(rebuilt) && rebuilt.length > 0) {
                setSections(rebuilt);
              }
            })
            .catch(() => {});
        }
      })
      .catch((err) => setError((err as Error)?.message || "加载论文失败"));
  }, [id, requestedPage]);

  useEffect(() => {
    if (!ready || !id || !progressLoadedRef.current || currentPage <= 0) return;
    const total = numPages || paper?.page_count || 0;
    const timer = window.setTimeout(() => {
      api
        .updatePaperProgress(id, { last_page: currentPage, total_pages: total })
        .catch(() => {});
    }, 900);
    return () => window.clearTimeout(timer);
  }, [currentPage, id, numPages, paper?.page_count, ready]);

  const zoomPdfAtWheel = useCallback(
    (event: WheelEvent) => {
      if (!event.ctrlKey && !event.metaKey) return;
      event.preventDefault();
      event.stopPropagation();

      const viewer = pdfViewerWithScale(pdfUtils);
      const scrollElement = viewer?.container || pdfWheelRef.current;
      if (!scrollElement || event.deltaY === 0) return;

      const viewerScale = viewer?.currentScale;
      const baseScale =
        typeof viewerScale === "number" && Number.isFinite(viewerScale) && viewerScale > 0
          ? viewerScale
          : typeof scaleValue === "number"
            ? scaleValue
            : 1;
      const direction = event.deltaY > 0 ? -1 : 1;
      const magnitude = clamp(Math.abs(event.deltaY) / 720, 0.04, 0.14);
      const nextScale = Number(clamp(baseScale * (1 + direction * magnitude), PDF_MIN_SCALE, PDF_MAX_SCALE).toFixed(2));
      if (nextScale === Number(baseScale.toFixed(2))) return;

      const rect = scrollElement.getBoundingClientRect();
      const clientX = event.clientX;
      const clientY = event.clientY;
      const anchorX = scrollElement.scrollLeft + clientX - rect.left;
      const anchorY = scrollElement.scrollTop + clientY - rect.top;
      setScaleValue(nextScale);

      window.requestAnimationFrame(() => {
        window.requestAnimationFrame(() => {
          const nextViewer = pdfViewerWithScale(pdfUtils);
          const nextScrollElement = nextViewer?.container || scrollElement;
          const nextRect = nextScrollElement.getBoundingClientRect();
          const ratio = nextScale / baseScale;
          nextScrollElement.scrollLeft = anchorX * ratio - (clientX - nextRect.left);
          nextScrollElement.scrollTop = anchorY * ratio - (clientY - nextRect.top);
        });
      });
    },
    [pdfUtils, scaleValue],
  );

  useEffect(() => {
    const element = pdfWheelRef.current;
    if (!element) return;
    element.addEventListener("wheel", zoomPdfAtWheel, { passive: false });
    return () => element.removeEventListener("wheel", zoomPdfAtWheel);
  }, [zoomPdfAtWheel]);

  const setPageCount = useCallback((pages: number) => {
    if (pages > 0) setNumPages((cur) => (cur === pages ? cur : pages));
  }, []);

  const clearLocatedAnnotation = useCallback(() => {
    if (locatedScrollTimerRef.current != null) {
      window.clearTimeout(locatedScrollTimerRef.current);
      locatedScrollTimerRef.current = null;
    }
    locatedScrollCleanupRef.current?.();
    locatedScrollCleanupRef.current = null;
    setLocatedAnnotationId(null);
  }, []);

  const scheduleLocatedAnnotationScrollClear = useCallback(
    (container: HTMLElement) => {
      if (locatedScrollTimerRef.current != null) {
        window.clearTimeout(locatedScrollTimerRef.current);
        locatedScrollTimerRef.current = null;
      }
      locatedScrollCleanupRef.current?.();
      locatedScrollCleanupRef.current = null;

      locatedScrollTimerRef.current = window.setTimeout(() => {
        locatedScrollTimerRef.current = null;
        const clearOnScroll = () => clearLocatedAnnotation();
        container.addEventListener("scroll", clearOnScroll, { once: true });
        locatedScrollCleanupRef.current = () => {
          container.removeEventListener("scroll", clearOnScroll);
        };
      }, LOCATED_ANNOTATION_SCROLL_RESUME_MS);
    },
    [clearLocatedAnnotation],
  );

  useEffect(() => clearLocatedAnnotation, [clearLocatedAnnotation]);

  const clearOutlineActive = useCallback(() => {
    outlineJumpRef.current = null;
    setOutlineActiveSectionId(null);
  }, []);

  const handlePdfPageChange = useCallback((page: number) => {
    const jumped = outlineJumpRef.current;
    if (jumped) {
      if (page === jumped.pageNo) {
        setCurrentPage(page);
        return;
      }
      if (Date.now() < jumped.ignoreUntil) return;
      clearOutlineActive();
    }
    setCurrentPage(page);
  }, [clearOutlineActive]);

  const goToPage = useCallback(
    (page: number, outlineEntry?: OutlineEntry) => {
      clearLocatedAnnotation();
      const max = numPages || paper?.page_count || page;
      const next = clamp(Math.round(page), 1, Math.max(1, max));
      if (outlineEntry) {
        outlineJumpRef.current = {
          sectionId: outlineEntry.section.id,
          pageNo: next,
          ignoreUntil: Date.now() + 1200,
        };
        setOutlineActiveSectionId(outlineEntry.section.id);
      } else {
        clearOutlineActive();
      }
      setCurrentPage(next);
      setPageDraft(String(next));
      pdfUtils?.goToPage(next);
    },
    [clearLocatedAnnotation, clearOutlineActive, numPages, paper?.page_count, pdfUtils],
  );

  const goToAnnotation = useCallback(
    (annotationID: number, pageNumber?: number, openAnnotations = false) => {
      const annotation = annotations.find((item) => item.id === annotationID);
      const page = annotation?.page_no || pageNumber || 1;
      if (openAnnotations) {
        updatePrefs({ annotationsOpen: true, mindMapOpen: false, qaOpen: false });
        if (annotation?.note) {
          setExpandedAnnotationNoteIDs((prev) => new Set(prev).add(annotation.id));
        }
      }
      clearOutlineActive();
      setCurrentPage(page);
      setPageDraft(String(page));
      if (annotation && pdfUtils) {
        const container = scrollHighlightToTop(pdfUtils, annotationToHighlight(annotation));
        if (container) {
          setLocatedAnnotationId(annotation.id);
          scheduleLocatedAnnotationScrollClear(container);
          return;
        }
        clearLocatedAnnotation();
        pdfUtils.goToPage(page);
        return;
      }
      goToPage(page);
    },
    [
      annotations,
      clearLocatedAnnotation,
      clearOutlineActive,
      goToPage,
      pdfUtils,
      scheduleLocatedAnnotationScrollClear,
      updatePrefs,
    ],
  );

  const goToOutlineEntry = useCallback(
    (entry: OutlineEntry) => {
      goToPage(entry.section.page_no, entry);
      scrollTitleIntoView(entry.section.page_no, outlineSearchTerms(entry));
    },
    [goToPage],
  );

  const loadPdfOutlineFallback = useCallback(
    async (pdfDocument: PDFDocumentProxy) => {
      if (!id || sections.length > 0 || outlineFallbackTriedRef.current) return;
      outlineFallbackTriedRef.current = true;
      try {
        const fallback = await extractPdfOutline(pdfDocument, id);
        if (fallback.length > 0) {
          setSections((cur) => (cur.length > 0 ? cur : fallback));
        }
      } catch {
        // PDF 内置目录只是兜底,失败不影响阅读。
      }
    },
    [id, sections.length],
  );

  const toggleAnnotationTextExpanded = useCallback((annotation: PaperAnnotation) => {
    setExpandedAnnotationTextIDs((prev) => {
      const next = new Set(prev);
      if (next.has(annotation.id)) next.delete(annotation.id);
      else next.add(annotation.id);
      return next;
    });
  }, []);

  const toggleAnnotationTransExpanded = useCallback((annotation: PaperAnnotation) => {
    setExpandedAnnotationTransIDs((prev) => {
      const next = new Set(prev);
      if (next.has(annotation.id)) next.delete(annotation.id);
      else next.add(annotation.id);
      return next;
    });
  }, []);

  const toggleAnnotationNoteExpanded = useCallback((annotation: PaperAnnotation) => {
    setExpandedAnnotationNoteIDs((prev) => {
      const next = new Set(prev);
      if (next.has(annotation.id)) next.delete(annotation.id);
      else next.add(annotation.id);
      return next;
    });
  }, []);

  const submitPage = useCallback(() => {
    const next = Number.parseInt(pageDraft, 10);
    if (!Number.isFinite(next)) {
      setPageDraft(String(currentPage));
      return;
    }
    goToPage(next);
  }, [currentPage, goToPage, pageDraft]);

  const translateAnnotation = useCallback(
    async (annotation: PaperAnnotation) => {
      if (!id || !annotation.text.trim()) return;
      setTranslatingIDs((prev) => new Set(prev).add(annotation.id));
      setTranslationErrors((prev) => {
        const next = { ...prev };
        delete next[annotation.id];
        return next;
      });
      try {
        const res = await api.translate(id, annotation.text);
        const updated = await api.updateAnnotation(id, annotation.id, {
          translation: res.translation,
        });
        setAnnotations((prev) =>
          prev.map((item) => (item.id === updated.id ? updated : item)),
        );
      } catch (err) {
        setTranslationErrors((prev) => ({
          ...prev,
          [annotation.id]: (err as Error)?.message || "翻译失败",
        }));
      } finally {
        setTranslatingIDs((prev) => {
          const next = new Set(prev);
          next.delete(annotation.id);
          return next;
        });
      }
    },
    [id],
  );

  const saveAnnotation = useCallback(
    async (selection: PdfSelection, note = "", color = prefs.color) => {
      const text = selection.content.text?.trim() || "";
      const pageNo = selection.position.boundingRect.pageNumber;
      if (!text) return false;
      if (selection.position.rects.some((rect) => rect.pageNumber !== pageNo)) {
        setError("跨页选区暂不支持保存批注，请缩短到单页后重试");
        return false;
      }
      setBusy(note ? "annotation" : "highlight");
      try {
        const annotation = await api.createAnnotation(id, {
          page_no: pageNo,
          text,
          note,
          color,
          bounding_rect: scaledToRect(selection.position.boundingRect),
          rects: selection.position.rects.map(scaledToRect),
        });
        setAnnotations((prev) => [annotation, ...prev]);
        updatePrefs({ annotationsOpen: true, mindMapOpen: false, qaOpen: false });
        window.getSelection()?.removeAllRanges();
        if (!note) void translateAnnotation(annotation);
        return true;
      } catch (err) {
        setError((err as Error)?.message || "保存批注失败");
        return false;
      } finally {
        setBusy("");
      }
    },
    [id, prefs.color, translateAnnotation, updatePrefs],
  );

  const onTranslateSelection = useCallback(
    (selection: PdfSelection) => {
      const text = selection.content.text?.trim();
      if (!text) return;
      const seq = translateSeq.current + 1;
      translateSeq.current = seq;
      const pageNo = selection.position.boundingRect.pageNumber;
      updatePrefs({ translateOpen: true, mindMapOpen: false, qaOpen: false });
      setTranslation({ original: text, translation: "", pageNo, loading: true, error: "" });
      window.getSelection()?.removeAllRanges();

      api
        .translate(id, text)
        .then((r) => {
          if (translateSeq.current !== seq) return;
          setTranslation({ original: text, translation: r.translation, pageNo, loading: false, error: "" });
        })
        .catch((err) => {
          if (translateSeq.current !== seq) return;
          setTranslation({
            original: text,
            translation: "",
            pageNo,
            loading: false,
            error: (err as Error)?.message || "翻译失败",
          });
        });
    },
    [id, updatePrefs],
  );

  const onAnnotateSelection = useCallback(
    (selection: PdfSelection) => {
      setPendingSelection(selection);
      setNoteDraft("");
      setNoteColor(prefs.color);
      setNoteOpen(true);
    },
    [prefs.color],
  );

  const onAskSelection = useCallback(
    (selection: PdfSelection) => {
      const text = selection.content.text?.trim() || "";
      if (!text) return;
      setQASelection({
        text,
        pageNo: selection.position.boundingRect.pageNumber,
      });
      updatePrefs({
        qaOpen: true,
        mindMapOpen: false,
        translateOpen: false,
        annotationsOpen: false,
      });
      window.getSelection()?.removeAllRanges();
    },
    [updatePrefs],
  );

  const deleteAnnotation = useCallback(
    async (annotation: PaperAnnotation) => {
      setBusy(`delete-${annotation.id}`);
      try {
        await api.deleteAnnotation(id, annotation.id);
        setAnnotations((prev) => prev.filter((item) => item.id !== annotation.id));
        if (locatedAnnotationId === annotation.id) {
          clearLocatedAnnotation();
        }
      } catch (err) {
        setError((err as Error)?.message || "删除批注失败");
      } finally {
        setBusy("");
      }
    },
    [clearLocatedAnnotation, id, locatedAnnotationId],
  );

  const changeAnnotationColor = useCallback(
    async (annotation: PaperAnnotation, color: AnnotationColor) => {
      setBusy(`color-${annotation.id}`);
      try {
        const updated = await api.updateAnnotation(id, annotation.id, { color });
        setAnnotations((prev) => prev.map((item) => (item.id === updated.id ? updated : item)));
      } catch (err) {
        setError((err as Error)?.message || "更新批注失败");
      } finally {
        setBusy("");
      }
    },
    [id],
  );

  const saveNote = async () => {
    if (!pendingSelection || busy) return;
    const ok = await saveAnnotation(pendingSelection, noteDraft, noteColor);
    if (!ok) return;
    setNoteOpen(false);
    setPendingSelection(null);
    setNoteDraft("");
  };

  const zoomBy = (delta: number) => {
    setScaleValue((cur) => {
      const base = typeof cur === "number" ? cur : 1;
      return Number((clamp(base + delta, PDF_MIN_SCALE, PDF_MAX_SCALE)).toFixed(2));
    });
  };

  return (
    <main className="flex h-dvh flex-col overflow-hidden bg-muted/50">
      <ReaderToolbar
        title={paperName(paper)}
        currentPage={currentPage}
        numPages={numPages}
        pageDraft={pageDraft}
        scaleValue={scaleValue}
        prefs={prefs}
        onClose={closeReader}
        onPrevPage={() => goToPage(currentPage - 1)}
        onNextPage={() => goToPage(currentPage + 1)}
        onPageDraftChange={setPageDraft}
        onPageSubmit={submitPage}
        onZoomIn={() => zoomBy(0.1)}
        onZoomOut={() => zoomBy(-0.1)}
        onResetZoom={() => setScaleValue(1)}
        onFitWidth={() => setScaleValue("page-width")}
        onToggleTranslate={toggleTranslatePanel}
        onToggleAnnotations={toggleAnnotationsPanel}
        onToggleMindMap={toggleMindMapPanel}
        onToggleQA={toggleQAPanel}
        onColorChange={(color) => updatePrefs({ color })}
      />

      <div ref={mindMapGridRef} className={cn("grid min-h-0 flex-1 grid-cols-1", gridClass)} style={gridStyle}>
        <section className="relative min-h-0 overflow-hidden border-r">
          {error && (
            <div className="absolute left-1/2 top-4 z-50 max-w-md -translate-x-1/2 rounded-lg border border-destructive/30 bg-background px-4 py-3 text-sm text-destructive shadow-lg">
              <div className="flex items-start gap-2">
                <span className="min-w-0 flex-1">{error}</span>
                <button type="button" onClick={() => setError("")} aria-label="关闭提示">
                  <X className="size-4" />
                </button>
              </div>
            </div>
          )}
          <ReaderLeftRail
            outlineOpen={prefs.outlineOpen}
            onToggleOutline={() => updatePrefs({ outlineOpen: true })}
          />
          {prefs.outlineOpen && (
            <OutlineDrawer
              sections={sections}
              paperTitle={paperName(paper)}
              currentPage={currentPage}
              activeSectionId={outlineActiveSectionId}
              onClose={() => updatePrefs({ outlineOpen: false })}
              onGoToEntry={goToOutlineEntry}
            />
          )}
          <div ref={pdfWheelRef} className="relative h-full min-h-0 overflow-hidden">
            {ready && pdfUrl ? (
              <ReaderPdf
                pdfUrl={pdfUrl}
                highlights={highlights}
                initialPage={initialPage}
                scaleValue={scaleValue}
                onPageChange={handlePdfPageChange}
                onPageCount={setPageCount}
                onTranslateSelection={onTranslateSelection}
                onSaveHighlight={(selection) => void saveAnnotation(selection, "", prefs.color)}
                onAnnotateSelection={onAnnotateSelection}
                onAskSelection={onAskSelection}
                onDeleteAnnotation={deleteAnnotation}
                locatedAnnotationId={locatedAnnotationId}
                onDocumentReady={(pdfDocument) => void loadPdfOutlineFallback(pdfDocument)}
                onUtilsReady={setPdfUtils}
              />
            ) : (
              <div className="flex h-full items-center justify-center text-sm text-muted-foreground">
                {error || "准备阅读器..."}
              </div>
            )}
          </div>
        </section>
        {prefs.mindMapOpen && (
          <ReadingMindMap
            open={prefs.mindMapOpen}
            paper={paper}
            onResizeStart={startMindMapResize}
            onLocateHighlight={goToAnnotation}
          />
        )}
        {rightPanelOpen && (
          <ReaderSidePanel
            showTranslation={prefs.translateOpen}
            showAnnotations={prefs.annotationsOpen}
            translation={translation}
            annotations={annotations}
            busy={busy}
            translatingIDs={translatingIDs}
            translationErrors={translationErrors}
            expandedAnnotationTextIDs={expandedAnnotationTextIDs}
            expandedAnnotationTransIDs={expandedAnnotationTransIDs}
            expandedAnnotationNoteIDs={expandedAnnotationNoteIDs}
            onClearTranslation={() => setTranslation(null)}
            onToggleAnnotationTextExpanded={toggleAnnotationTextExpanded}
            onToggleAnnotationTransExpanded={toggleAnnotationTransExpanded}
            onToggleAnnotationNoteExpanded={toggleAnnotationNoteExpanded}
            onDelete={deleteAnnotation}
            onColorChange={changeAnnotationColor}
            onRetryTranslate={(annotation) => void translateAnnotation(annotation)}
            onLocateAnnotation={(annotation) => goToAnnotation(annotation.id, annotation.page_no, false)}
          />
        )}
        {qaPanelOpen && (
          <ReaderQAPanel
            paper={paper}
            currentPage={currentPage}
            selection={qaSelection}
            onClearSelection={() => setQASelection(null)}
            onClose={() => {
              setQASelection(null);
              updatePrefs({ qaOpen: false });
            }}
          />
        )}
      </div>

      <Dialog
        open={noteOpen}
        onOpenChange={(open) => {
          setNoteOpen(open);
          if (!open) setPendingSelection(null);
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>添加批注</DialogTitle>
          </DialogHeader>
          <div className="space-y-3">
            <Textarea
              value={noteDraft}
              onChange={(event) => setNoteDraft(event.target.value)}
              placeholder="写下这段原文的理解、疑问或后续要追踪的问题"
              className="min-h-32"
            />
            <div className="flex items-center justify-between gap-3 rounded-md border bg-muted/30 px-3 py-2">
              <span className="text-sm text-muted-foreground">批注颜色</span>
              <ColorSwatches value={noteColor} onChange={setNoteColor} />
            </div>
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setNoteOpen(false)}>
              取消
            </Button>
            <Button type="button" disabled={Boolean(busy)} onClick={() => void saveNote()}>
              {busy === "annotation" && <Loader2 className="size-4 animate-spin" />}
              保存
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </main>
  );
}
