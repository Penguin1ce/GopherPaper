"use client";

import {
  ArrowLeft,
  BookMarked,
  Check,
  ChevronLeft,
  ChevronRight,
  Languages,
  ListTree,
  Loader2,
  Maximize2,
  MessageSquarePlus,
  Minus,
  MoreHorizontal,
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
  type FormEvent,
  type ReactNode,
} from "react";
import {
  MonitoredHighlightContainer,
  PdfHighlighter,
  TextHighlight,
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
import type {
  AnnotationRect,
  Paper,
  PaperAnnotation,
  PaperSection,
} from "@/lib/gopherpaper/types";
import { cn } from "@/lib/utils";

const AUTH_KEY = "gopherpaper.auth";
const PDF_WORKER = "/pdfjs/pdf.worker.min.mjs";
const PDF_VIEWER_PATCH_FLAG = "__gopherpaperSkipSameDocumentSet";
const READER_PREFS_KEY = "gopherpaper.reader.preferences";
const RIGHT_PANEL_WIDTH = "24rem";

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
  color: AnnotationColor;
}

const DEFAULT_PREFS: ReaderPreferences = {
  outlineOpen: false,
  translateOpen: true,
  annotationsOpen: true,
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

function loadReaderPreferences(): ReaderPreferences {
  if (typeof window === "undefined") return DEFAULT_PREFS;
  try {
    const raw = localStorage.getItem(READER_PREFS_KEY);
    if (!raw) return DEFAULT_PREFS;
    const saved = JSON.parse(raw) as Partial<ReaderPreferences>;
    const color = COLOR_KEYS.includes(saved.color as AnnotationColor)
      ? (saved.color as AnnotationColor)
      : DEFAULT_PREFS.color;
    return {
      outlineOpen: saved.outlineOpen ?? DEFAULT_PREFS.outlineOpen,
      translateOpen: saved.translateOpen ?? DEFAULT_PREFS.translateOpen,
      annotationsOpen: saved.annotationsOpen ?? DEFAULT_PREFS.annotationsOpen,
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

function isStandaloneTopLevelTitle(title: string) {
  const normalized = title.trim().toLowerCase().replace(/[.:\uFF1A]+$/, "");
  return /^(abstract|acknowledg(?:e)?ments?|references|bibliography|appendix|appendices|supplementary materials?|limitations?|ethics statement|broader impacts?|impact statement|data availability|funding|conflicts? of interest)$/.test(normalized);
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
      target.scrollIntoView({ block: "center", inline: "nearest", behavior: "smooth" });
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
  readPercent,
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
  onColorChange,
}: {
  title: string;
  currentPage: number;
  numPages: number;
  readPercent: number;
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
        <span className="min-w-16 rounded-md border bg-background px-2 py-1 text-center text-xs font-medium text-muted-foreground">
          已读 {readPercent}%
        </span>
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
          <span className="line-clamp-2 min-w-0">{node.title}</span>
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
  onClose,
  onGoToEntry,
}: {
  sections: PaperSection[];
  paperTitle?: string;
  currentPage: number;
  onClose: () => void;
  onGoToEntry: (entry: OutlineEntry) => void;
}) {
  const sorted = useMemo(() => {
    const seen = new Set<string>();
    return sections
      .filter((section) => section.title && section.page_no > 0)
      .filter((section) => !isPaperTitleSection(section, paperTitle))
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
  const active = useMemo(
    () => [...entries].reverse().find((entry) => entry.section.page_no <= currentPage) ?? null,
    [entries, currentPage],
  );
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
  onGoToPage,
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
  onGoToPage: (page: number) => void;
}) {
  const color = (annotation.color as AnnotationColor) || "yellow";
  return (
    <article className="rounded-md border bg-background p-3 shadow-sm">
      <div className="flex items-center justify-between gap-2">
        <button
          type="button"
          onClick={() => onGoToPage(annotation.page_no)}
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
            <DropdownMenuItem onClick={() => onGoToPage(annotation.page_no)}>跳转到原文</DropdownMenuItem>
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
  onGoToPage,
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
  onGoToPage: (page: number) => void;
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
                onGoToPage={onGoToPage}
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
  onGoToPage,
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
  onGoToPage: (page: number) => void;
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
            onGoToPage={onGoToPage}
          />
        )}
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
}: {
  onTranslate: (selection: PdfSelection) => void;
  onHighlight: (selection: PdfSelection) => void;
  onAnnotate: (selection: PdfSelection) => void;
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
        <MessageSquarePlus className="size-3.5" />
        批注
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
}: {
  onDelete: (annotation: PaperAnnotation) => void;
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
        isScrolledTo={isScrolledTo}
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
  onDeleteAnnotation,
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
  onDeleteAnnotation: (annotation: PaperAnnotation) => void;
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
          onDeleteAnnotation={onDeleteAnnotation}
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
  onDeleteAnnotation,
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
  onDeleteAnnotation: (annotation: PaperAnnotation) => void;
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
      <HighlightContainer onDelete={onDeleteAnnotation} />
    </PdfHighlighter>
  );
}

export function ReaderClient() {
  const id = useMemo(() => {
    if (typeof location === "undefined") return "";
    return new URLSearchParams(location.search).get("id") || "";
  }, []);
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
  const [translationErrors, setTranslationErrors] = useState<Record<number, string>>({});
  const [translatingIDs, setTranslatingIDs] = useState<Set<number>>(() => new Set());
  const [expandedAnnotationTextIDs, setExpandedAnnotationTextIDs] = useState<Set<number>>(() => new Set());
  const [expandedAnnotationTransIDs, setExpandedAnnotationTransIDs] = useState<Set<number>>(() => new Set());
  const [expandedAnnotationNoteIDs, setExpandedAnnotationNoteIDs] = useState<Set<number>>(() => new Set());
  const [noteOpen, setNoteOpen] = useState(false);
  const [noteDraft, setNoteDraft] = useState("");
  const [noteColor, setNoteColor] = useState<AnnotationColor>("yellow");
  const [pendingSelection, setPendingSelection] = useState<PdfSelection | null>(null);
  const [busy, setBusy] = useState("");
  const [prefs, setPrefs] = useState<ReaderPreferences>(() => loadReaderPreferences());
  const [pdfUtils, setPdfUtils] = useState<PdfHighlighterUtils | null>(null);
  const translateSeq = useRef(0);
  const progressLoadedRef = useRef(false);
  const outlineFallbackTriedRef = useRef(false);

  const highlights = useMemo(() => annotations.map(annotationToHighlight), [annotations]);
  const initialPage = Math.max(1, paper?.last_read_page || 1);
  const pdfUrl = useMemo(() => (ready && id ? api.paperFileUrl(id) : ""), [ready, id]);
  const rightPanelOpen = prefs.translateOpen || prefs.annotationsOpen;
  const gridClass = rightPanelOpen
    ? "lg:grid-cols-[minmax(0,1fr)_24rem]"
    : "lg:grid-cols-[minmax(0,1fr)]";
  const effectivePageCount = numPages || paper?.page_count || 0;
  const readPercent =
    effectivePageCount > 0
      ? clamp(Math.round((currentPage / effectivePageCount) * 100), 0, 100)
      : 0;

  const updatePrefs = useCallback((patch: Partial<ReaderPreferences>) => {
    setPrefs((cur) => ({ ...cur, ...patch }));
  }, []);

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
        setCurrentPage(Math.max(1, detail.paper.last_read_page || 1));
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
  }, [id]);

  useEffect(() => {
    if (!ready || !id || !progressLoadedRef.current || currentPage <= 0) return;
    const total = numPages || paper?.page_count || 0;
    const timer = window.setTimeout(() => {
      api
        .updatePaperProgress(id, { last_page: currentPage, total_pages: total })
        .then((progress) => {
          setPaper((cur) =>
            cur
              ? {
                  ...cur,
                  progress: progress.progress,
                  last_read_page: progress.last_read_page,
                }
              : cur,
          );
        })
        .catch(() => {});
    }, 900);
    return () => window.clearTimeout(timer);
  }, [currentPage, id, numPages, paper?.page_count, ready]);

  const setPageCount = useCallback((pages: number) => {
    if (pages > 0) setNumPages((cur) => (cur === pages ? cur : pages));
  }, []);

  const goToPage = useCallback(
    (page: number) => {
      const max = numPages || paper?.page_count || page;
      const next = clamp(Math.round(page), 1, Math.max(1, max));
      setCurrentPage(next);
      setPageDraft(String(next));
      pdfUtils?.goToPage(next);
    },
    [numPages, paper?.page_count, pdfUtils],
  );

  const goToOutlineEntry = useCallback(
    (entry: OutlineEntry) => {
      goToPage(entry.section.page_no);
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
        updatePrefs({ annotationsOpen: true });
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
      updatePrefs({ translateOpen: true });
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

  const deleteAnnotation = useCallback(
    async (annotation: PaperAnnotation) => {
      setBusy(`delete-${annotation.id}`);
      try {
        await api.deleteAnnotation(id, annotation.id);
        setAnnotations((prev) => prev.filter((item) => item.id !== annotation.id));
      } catch (err) {
        setError((err as Error)?.message || "删除批注失败");
      } finally {
        setBusy("");
      }
    },
    [id],
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
      return Number((clamp(base + delta, 0.6, 2.4)).toFixed(2));
    });
  };

  return (
    <main className="flex h-dvh flex-col overflow-hidden bg-muted/50">
      <ReaderToolbar
        title={paperName(paper)}
        currentPage={currentPage}
        numPages={numPages}
        readPercent={readPercent}
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
        onToggleTranslate={() => updatePrefs({ translateOpen: !prefs.translateOpen })}
        onToggleAnnotations={() => updatePrefs({ annotationsOpen: !prefs.annotationsOpen })}
        onColorChange={(color) => updatePrefs({ color })}
      />

      <div className={cn("grid min-h-0 flex-1 grid-cols-1", gridClass)}>
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
              onClose={() => updatePrefs({ outlineOpen: false })}
              onGoToEntry={goToOutlineEntry}
            />
          )}
          <div className="relative h-full min-h-0 overflow-hidden">
            {ready && pdfUrl ? (
              <ReaderPdf
                pdfUrl={pdfUrl}
                highlights={highlights}
                initialPage={initialPage}
                scaleValue={scaleValue}
                onPageChange={setCurrentPage}
                onPageCount={setPageCount}
                onTranslateSelection={onTranslateSelection}
                onSaveHighlight={(selection) => void saveAnnotation(selection, "", prefs.color)}
                onAnnotateSelection={onAnnotateSelection}
                onDeleteAnnotation={deleteAnnotation}
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
            onGoToPage={goToPage}
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
