"use client";

import "@/lib/pdfjs-global";
import {
  ArrowUp,
  Check,
  ChevronRight,
  ListTree,
  Loader2,
  MessageSquarePlus,
  Plus,
  X,
} from "lucide-react";
import { useRouter, useSearchParams } from "next/navigation";
import type { PDFDocumentProxy } from "pdfjs-dist";
import { PDFViewer } from "pdfjs-dist/web/pdf_viewer.mjs";
import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type CSSProperties,
  type PointerEvent as ReactPointerEvent,
} from "react";
import {
  type DrawingStroke,
  type PdfHighlighterUtils,
  type PdfScaleValue,
  type PdfSelection,
  type Scaled,
  type ScaledPosition,
} from "react-pdf-highlighter-plus";
import "pdfjs-dist/web/pdf_viewer.css";
import "react-pdf-highlighter-plus/style/style.css";
import "./components/reader-overrides.css";

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
import { ScrollArea } from "@/components/ui/scroll-area";
import { Textarea } from "@/components/ui/textarea";
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
import {
  ReaderLeftRail as CompactReaderLeftRail,
  ReaderToolbar as CompactReaderToolbar,
} from "./components/reader-toolbar";
import {
  ReaderPdf as InteractiveReaderPdf,
  annotationToHighlight as splitAnnotationToHighlight,
  scrollHighlightToTop as scrollSplitHighlightToTop,
} from "./components/reader-pdf";
import { ReaderSidePanel as SplitReaderSidePanel } from "./components/reader-side-panel";
import {
  DEFAULT_DRAWING_SIZE,
  DEFAULT_TEXT_SIZE,
  DRAWING_SIZE_MAX,
  DRAWING_SIZE_MIN,
  FREETEXT_CREATE_TEXT,
  FREETEXT_EMPTY_DRAFT,
  COLOR_KEYS as READER_COLOR_KEYS,
  annotationKind,
  defaultFreetextPosition,
  drawingStyle,
  drawingStyleForColor,
  freetextStyle,
  freetextStyleForColor,
  normalizeFreetextText,
  positionToRects,
  type AnnotationColor as ReaderAnnotationColor,
  type ReaderTool,
} from "./lib/annotations";

const AUTH_KEY = "gopherpaper.auth";
const PDF_VIEWER_PATCH_FLAG = "__gopherpaperSkipSameDocumentSet";
const READER_PREFS_KEY = "gopherpaper.reader.preferences";
const RIGHT_PANEL_WIDTH = "24rem";
const MIND_MAP_PANEL_DEFAULT_WIDTH = 560;
const MIND_MAP_PANEL_MIN_WIDTH = 380;
const MIND_MAP_PANEL_MAX_WIDTH = 860;
const OUTLINE_PANEL_WIDTH_CLASS = "lg:grid-cols-[20rem_minmax(0,1fr)]";
const PDF_MIN_SCALE = 0.6;
const PDF_MAX_SCALE = 2.4;
const PDF_WHEEL_ZOOM_STEP = 0.1;
const PDF_WHEEL_ZOOM_DELTA = 96;
const PDF_WHEEL_ZOOM_LINE_HEIGHT = 32;
const PDF_WHEEL_ZOOM_MAX_STEPS = 2;
const PDF_WHEEL_ZOOM_IDLE_MS = 180;
const LOCATE_TOP_GAP = 32;
const LOCATED_ANNOTATION_SCROLL_RESUME_MS = 600;
const QA_SELECTION_PREVIEW_RUNES = 48;
const FREETEXT_DUPLICATE_POSITION_EPSILON = 8;
const FREETEXT_CREATE_COOLDOWN_MS = 800;

type PatchablePDFViewer = {
  pdfDocument?: PDFDocumentProxy | null;
  setDocument: (pdfDocument: PDFDocumentProxy | null) => void;
  [PDF_VIEWER_PATCH_FLAG]?: boolean;
};

type AnnotationColor =
  | "yellow"
  | "red"
  | "green"
  | "blue"
  | "purple"
  | "magenta"
  | "orange"
  | "gray"
  | "black";

const COLOR_META: Record<
  AnnotationColor,
  { label: string; className: string; value: string }
> = {
  yellow: {
    label: "黄色",
    className: "bg-amber-300",
    value: "rgba(255, 226, 143, 0.62)",
  },
  red: {
    label: "红色",
    className: "bg-red-400",
    value: "rgba(248, 113, 113, 0.5)",
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
  purple: {
    label: "紫色",
    className: "bg-violet-300",
    value: "rgba(196, 181, 253, 0.54)",
  },
  magenta: {
    label: "洋红色",
    className: "bg-fuchsia-400",
    value: "rgba(217, 70, 239, 0.38)",
  },
  orange: {
    label: "橙色",
    className: "bg-orange-300",
    value: "rgba(253, 186, 116, 0.56)",
  },
  gray: {
    label: "灰色",
    className: "bg-neutral-400",
    value: "rgba(163, 163, 163, 0.45)",
  },
  black: {
    label: "黑色",
    className: "bg-black",
    value: "rgba(24, 24, 27, 0.3)",
  },
};

const COLOR_KEYS = Object.keys(COLOR_META) as AnnotationColor[];

interface ReaderPreferences {
  outlineOpen: boolean;
  translateOpen: boolean;
  annotationsOpen: boolean;
  mindMapOpen: boolean;
  qaOpen: boolean;
  color: ReaderAnnotationColor;
  activeTool: ReaderTool;
  textSize: number;
  drawingSize: number;
}

const DEFAULT_PREFS: ReaderPreferences = {
  outlineOpen: false,
  translateOpen: true,
  annotationsOpen: true,
  mindMapOpen: false,
  qaOpen: false,
  color: "yellow",
  activeTool: "select",
  textSize: DEFAULT_TEXT_SIZE,
  drawingSize: DEFAULT_DRAWING_SIZE,
};

interface RecentFreetextCreate {
  pageNumber: number;
  x1: number;
  y1: number;
  until: number;
}

interface DrawingCreateMeta {
  width: number;
  height: number;
  snapshot?: string;
}

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

function requestedPageFromParams(params: Pick<URLSearchParams, "get">): number {
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
    const color = READER_COLOR_KEYS.includes(saved.color as ReaderAnnotationColor)
      ? (saved.color as ReaderAnnotationColor)
      : DEFAULT_PREFS.color;
    const mindMapOpen = saved.mindMapOpen ?? DEFAULT_PREFS.mindMapOpen;
    const qaOpen = mindMapOpen ? false : (saved.qaOpen ?? DEFAULT_PREFS.qaOpen);
    const activeTool =
      saved.activeTool === "freetext" || saved.activeTool === "drawing" || saved.activeTool === "select"
        ? saved.activeTool
        : DEFAULT_PREFS.activeTool;
    const textSize =
      typeof saved.textSize === "number" && Number.isFinite(saved.textSize)
        ? clamp(saved.textSize, 10, 28)
        : DEFAULT_PREFS.textSize;
    const drawingSize =
      typeof saved.drawingSize === "number" && Number.isFinite(saved.drawingSize)
        ? clamp(saved.drawingSize, DRAWING_SIZE_MIN, DRAWING_SIZE_MAX)
        : DEFAULT_PREFS.drawingSize;
    return {
      outlineOpen: saved.outlineOpen ?? DEFAULT_PREFS.outlineOpen,
      translateOpen: mindMapOpen || qaOpen ? false : (saved.translateOpen ?? DEFAULT_PREFS.translateOpen),
      annotationsOpen: mindMapOpen || qaOpen ? false : (saved.annotationsOpen ?? DEFAULT_PREFS.annotationsOpen),
      mindMapOpen,
      qaOpen,
      color,
      activeTool,
      textSize,
      drawingSize,
    };
  } catch {
    localStorage.removeItem(READER_PREFS_KEY);
    return DEFAULT_PREFS;
  }
}

function paperName(paper: Paper | null, fallback = "") {
  return paper?.title || paper?.file_name || fallback || "论文精读";
}

function clamp(n: number, min: number, max: number) {
  return Math.min(max, Math.max(min, n));
}

function roundPdfScale(scale: number) {
  return Number(scale.toFixed(3));
}

function wheelDeltaPixels(event: WheelEvent, pageHeight: number) {
  if (event.deltaMode === WheelEvent.DOM_DELTA_LINE) {
    return event.deltaY * PDF_WHEEL_ZOOM_LINE_HEIGHT;
  }
  if (event.deltaMode === WheelEvent.DOM_DELTA_PAGE) {
    return event.deltaY * pageHeight;
  }
  return event.deltaY;
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

const RECT_EPSILON = 0.0001;
const RECT_NUMBER_FIELDS = ["x1", "y1", "x2", "y2", "width", "height"] as const;

function sameAnnotationRect(a: AnnotationRect, b: AnnotationRect) {
  return (
    a.pageNumber === b.pageNumber &&
    RECT_NUMBER_FIELDS.every((field) => Math.abs(a[field] - b[field]) <= RECT_EPSILON)
  );
}

function sameAnnotationRects(a: AnnotationRect[] | undefined, b: AnnotationRect[]) {
  if ((a?.length ?? 0) !== b.length) return false;
  return b.every((rect, index) => {
    const candidate = a?.[index];
    return candidate != null && sameAnnotationRect(candidate, rect);
  });
}

function annotationUpdatedAt(annotation: PaperAnnotation) {
  const updated = Date.parse(annotation.updated_at || "");
  if (Number.isFinite(updated)) return updated;
  const created = Date.parse(annotation.created_at || "");
  return Number.isFinite(created) ? created : 0;
}

function isDraftFreetext(annotation: PaperAnnotation) {
  const text = normalizeFreetextText(annotation.text);
  return !text || text === FREETEXT_CREATE_TEXT;
}

function sameDraftFreetextLayer(a: PaperAnnotation, b: PaperAnnotation) {
  if (annotationKind(a) !== "freetext" || annotationKind(b) !== "freetext") return false;
  if (!isDraftFreetext(a) && !isDraftFreetext(b)) return false;
  const ar = a.bounding_rect;
  const br = b.bounding_rect;
  return (
    ar.pageNumber === br.pageNumber &&
    Math.abs(ar.x1 - br.x1) <= FREETEXT_DUPLICATE_POSITION_EPSILON &&
    Math.abs(ar.y1 - br.y1) <= FREETEXT_DUPLICATE_POSITION_EPSILON
  );
}

function shouldPreferFreetext(candidate: PaperAnnotation, existing: PaperAnnotation) {
  const candidateDraft = isDraftFreetext(candidate);
  const existingDraft = isDraftFreetext(existing);
  if (candidateDraft !== existingDraft) return !candidateDraft;
  const candidateTime = annotationUpdatedAt(candidate);
  const existingTime = annotationUpdatedAt(existing);
  return candidateTime > existingTime || (candidateTime === existingTime && candidate.id > existing.id);
}

function compareReaderAnnotations(a: PaperAnnotation, b: PaperAnnotation) {
  if (a.page_no !== b.page_no) return a.page_no - b.page_no;
  if (a.bounding_rect.y1 !== b.bounding_rect.y1) return a.bounding_rect.y1 - b.bounding_rect.y1;
  if (a.bounding_rect.x1 !== b.bounding_rect.x1) return a.bounding_rect.x1 - b.bounding_rect.x1;
  return a.id - b.id;
}

function visibleReaderAnnotations(items: PaperAnnotation[]) {
  const result: PaperAnnotation[] = [];
  const indexById = new Map<number, number>();
  const freetextIndexes: number[] = [];

  for (const item of items) {
    const duplicatedIndex = indexById.get(item.id);
    if (duplicatedIndex != null) {
      if (annotationUpdatedAt(item) >= annotationUpdatedAt(result[duplicatedIndex])) {
        result[duplicatedIndex] = item;
      }
      continue;
    }

    if (annotationKind(item) !== "freetext") {
      indexById.set(item.id, result.length);
      result.push(item);
      continue;
    }

    const existingIndex = freetextIndexes.find((index) => sameDraftFreetextLayer(result[index], item));
    if (existingIndex == null) {
      indexById.set(item.id, result.length);
      freetextIndexes.push(result.length);
      result.push(item);
      continue;
    }

    const existing = result[existingIndex];
    if (shouldPreferFreetext(item, existing)) {
      result[existingIndex] = item;
    }
  }

  return result.sort(compareReaderAnnotations);
}

function scrollSideAnnotationIntoView(annotationID: number) {
  const selector = `[data-reader-side-annotation-id="${annotationID}"]`;
  const scroll = () => {
    document.querySelector<HTMLElement>(selector)?.scrollIntoView({
      block: "nearest",
      behavior: "smooth",
    });
  };
  window.requestAnimationFrame(() => {
    window.requestAnimationFrame(scroll);
  });
}

function shouldKeepAnnotationSelection(target: EventTarget | null) {
  if (!(target instanceof HTMLElement)) return false;
  return Boolean(
    target.closest(
      [
        "[data-reader-annotation-id]",
        "[data-reader-side-annotation-id]",
        "input",
        "textarea",
        "select",
        "[contenteditable='true']",
      ].join(", "),
    ),
  );
}

function annotationWithPosition(
  annotation: PaperAnnotation,
  boundingRect: AnnotationRect,
  rects: AnnotationRect[],
): PaperAnnotation {
  return {
    ...annotation,
    page_no: boundingRect.pageNumber,
    bounding_rect: boundingRect,
    rects,
  };
}

function isRecentFreetextCreate(recent: RecentFreetextCreate | null, rect: AnnotationRect) {
  return Boolean(
    recent &&
      Date.now() < recent.until &&
      recent.pageNumber === rect.pageNumber &&
      Math.abs(recent.x1 - rect.x1) <= FREETEXT_DUPLICATE_POSITION_EPSILON &&
      Math.abs(recent.y1 - rect.y1) <= FREETEXT_DUPLICATE_POSITION_EPSILON,
  );
}

interface TranslationResult {
  original: string;
  translation: string;
  pageNo: number;
  loading: boolean;
  error: string;
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

function pdfViewerWithScale(utils: PdfHighlighterUtils | null): PdfViewerScaleLike | null {
  const viewer = utils?.getViewer();
  if (!viewer || typeof viewer !== "object") return null;
  return viewer as PdfViewerScaleLike;
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

async function annotationColorPatch(
  annotation: PaperAnnotation,
  color: ReaderAnnotationColor,
): Promise<Parameters<typeof api.updateAnnotation>[2]> {
  if (annotation.kind === "freetext") {
    const current = freetextStyle(annotation);
    return {
      color,
      style_json: {
        ...(annotation.style_json ?? {}),
        ...freetextStyleForColor(color, current.fontSize),
      },
    };
  }

  if (annotation.kind === "drawing") {
    const current = drawingStyle(annotation);
    const nextStyle = drawingStyleForColor(color, current.strokeWidth);
    const contentJson = await recolorDrawingContent(annotation, nextStyle.strokeColor);
    return {
      color,
      style_json: {
        ...(annotation.style_json ?? {}),
        ...nextStyle,
      },
      ...(contentJson ? { content_json: contentJson } : {}),
    };
  }

  return { color };
}

async function recolorDrawingContent(annotation: PaperAnnotation, strokeColor: string) {
  const content = annotation.content_json ?? {};
  const strokes = storedDrawingStrokes(content.strokes);
  if (!strokes || strokes.length === 0) return undefined;

  const nextStrokes = strokes.map((stroke) => ({ ...stroke, color: strokeColor }));
  const size = await drawingImageSize(content.image, nextStrokes);
  const image = renderDrawingStrokesToImage(nextStrokes, size.width, size.height);
  if (!image) return { ...content, strokes: nextStrokes };
  return { ...content, image, strokes: nextStrokes };
}

function storedDrawingStrokes(value: unknown): DrawingStroke[] | null {
  if (!Array.isArray(value)) return null;
  const strokes = value.filter((stroke): stroke is DrawingStroke => {
    if (!stroke || typeof stroke !== "object") return false;
    const candidate = stroke as DrawingStroke;
    return (
      Array.isArray(candidate.points) &&
      candidate.points.every((point) =>
        point &&
        typeof point === "object" &&
        typeof point.x === "number" &&
        typeof point.y === "number",
      ) &&
      typeof candidate.width === "number"
    );
  });
  return strokes.length > 0 ? strokes : null;
}

async function drawingImageSize(value: unknown, strokes: DrawingStroke[]) {
  if (typeof value === "string" && value.startsWith("data:image/") && typeof Image !== "undefined") {
    const size = await new Promise<{ width: number; height: number } | null>((resolve) => {
      const image = new Image();
      image.onload = () => resolve({ width: image.naturalWidth, height: image.naturalHeight });
      image.onerror = () => resolve(null);
      image.src = value;
    });
    if (size && size.width > 0 && size.height > 0) return size;
  }

  const maxStrokeWidth = Math.max(1, ...strokes.map((stroke) => stroke.width || 1));
  const padding = maxStrokeWidth * 2;
  let maxX = 1;
  let maxY = 1;
  for (const stroke of strokes) {
    for (const point of stroke.points) {
      maxX = Math.max(maxX, point.x);
      maxY = Math.max(maxY, point.y);
    }
  }
  return {
    width: Math.ceil(maxX + padding),
    height: Math.ceil(maxY + padding),
  };
}

function renderDrawingStrokesToImage(strokes: DrawingStroke[], width: number, height: number) {
  const canvas = document.createElement("canvas");
  canvas.width = Math.max(1, Math.ceil(width));
  canvas.height = Math.max(1, Math.ceil(height));
  const ctx = canvas.getContext("2d");
  if (!ctx) return "";
  for (const stroke of strokes) {
    if (stroke.points.length < 2) continue;
    ctx.strokeStyle = stroke.color;
    ctx.lineWidth = stroke.width;
    ctx.lineCap = "round";
    ctx.lineJoin = "round";
    ctx.beginPath();
    ctx.moveTo(stroke.points[0].x, stroke.points[0].y);
    for (const point of stroke.points.slice(1)) {
      ctx.lineTo(point.x, point.y);
    }
    ctx.stroke();
  }
  return canvas.toDataURL("image/png");
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
    <aside className="h-72 min-h-0 w-full overflow-hidden border-r bg-background shadow-sm lg:h-full">
      <div className="flex h-full min-h-0 flex-col">
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
  if (ref.block_type === "selection") return ref.page_no ? `选段 p.${ref.page_no}` : "选段";
  if (ref.fallback_scope === "paper" && ref.page_no) return `全文补充 p.${ref.page_no}`;
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
              selectionRef ? "border-primary/25 bg-primary/5 text-primary" : "bg-muted/40 text-muted-foreground",
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
      <span className="px-0.5 text-[11px] text-muted-foreground">{assistant ? "小耄耋" : "我"}</span>
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
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
      abortRef.current?.abort();
    };
  }, []);

  useEffect(() => {
    if (selection) setScope("selection");
    else setScope((cur) => (cur === "selection" ? "page" : cur));
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
    if (effectiveScope === "paper") return { scope: "paper" };
    if (effectiveScope === "selection" && selection) {
      return { scope: "selection", page_no: selection.pageNo, selected_text: selectedText };
    }
    return { scope: "page", page_no: currentPage };
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
      const data = await api.sendMessage(
        sid,
        query,
        undefined,
        {
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
        },
        readerContextForSubmit(),
        controller.signal,
      );
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
      if (placeholderID) setMessages((list) => list.filter((message) => message.id !== placeholderID));
      setError((err as Error)?.message || "小耄耋应答失败");
    } finally {
      if (abortRef.current === controller) abortRef.current = null;
      if (mountedRef.current) setSending(false);
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
                小耄耋正在阅读当前上下文...
              </div>
            )}
            {error && (
              <div className="rounded-md border border-destructive/30 bg-background p-3 text-sm text-destructive">
                {error}
              </div>
            )}
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

export function ReaderClient() {
  const { activePaperID, selectPaper } = useApp();
  const router = useRouter();
  const searchParams = useSearchParams();
  const id = (searchParams.get("id") || "").trim();
  const requestedPage = useMemo(() => requestedPageFromParams(searchParams), [searchParams]);
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
  const [selectedAnnotationId, setSelectedAnnotationId] = useState<number | null>(null);
  const [pendingFreetextFocusId, setPendingFreetextFocusId] = useState<number | null>(null);
  const translateSeq = useRef(0);
  const progressLoadedRef = useRef(false);
  const outlineFallbackTriedRef = useRef(false);
  const outlineJumpRef = useRef<{ sectionId: number; pageNo: number; ignoreUntil: number } | null>(null);
  const locatedScrollTimerRef = useRef<number | null>(null);
  const locatedScrollCleanupRef = useRef<(() => void) | null>(null);
  const annotationFocusScrollIgnoreUntilRef = useRef(0);
  const annotationFocusActiveRef = useRef(false);
  const positionPatchSeqRef = useRef(new Map<number, number>());
  const freetextCreateInFlightRef = useRef(false);
  const recentFreetextCreateRef = useRef<RecentFreetextCreate | null>(null);
  const clearDrawingDraftRef = useRef<(() => void) | null>(null);
  const saveDrawingDraftRef = useRef<(() => void) | null>(null);
  const mindMapGridRef = useRef<HTMLDivElement | null>(null);
  const pdfWheelRef = useRef<HTMLDivElement | null>(null);
  const scaleValueRef = useRef<PdfScaleValue>(scaleValue);
  const pdfUtilsRef = useRef<PdfHighlighterUtils | null>(pdfUtils);
  const wheelZoomRef = useRef({
    frame: 0,
    restoreSeq: 0,
    deltaY: 0,
    clientX: 0,
    clientY: 0,
    idleTimer: 0,
  });

  const visibleAnnotations = useMemo(() => visibleReaderAnnotations(annotations), [annotations]);
  const highlights = useMemo(
    () => visibleAnnotations.map(splitAnnotationToHighlight),
    [visibleAnnotations],
  );
  const initialPage = Math.max(1, requestedPage || paper?.last_read_page || 1);
  const pdfUrl = useMemo(() => (ready && id ? api.paperFileUrl(id) : ""), [ready, id]);
  const qaPanelOpen = !prefs.mindMapOpen && prefs.qaOpen;
  const rightPanelOpen = !prefs.mindMapOpen && !prefs.qaOpen && (prefs.translateOpen || prefs.annotationsOpen);
  const sidePanelOpen = qaPanelOpen || rightPanelOpen;
  const gridClass = prefs.outlineOpen
    ? prefs.mindMapOpen
      ? "lg:grid-cols-[20rem_minmax(0,1fr)_var(--mind-map-panel-width)]"
      : sidePanelOpen
        ? "lg:grid-cols-[20rem_minmax(0,1fr)_24rem]"
        : OUTLINE_PANEL_WIDTH_CLASS
    : prefs.mindMapOpen
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

  const changeActiveTool = useCallback((tool: ReaderTool) => {
    saveDrawingDraftRef.current?.();
    setPrefs((cur) => ({
      ...cur,
      activeTool: cur.activeTool === tool && tool !== "select" ? "select" : tool,
    }));
  }, []);

  useEffect(() => {
    scaleValueRef.current = scaleValue;
  }, [scaleValue]);

  useEffect(() => {
    pdfUtilsRef.current = pdfUtils;
  }, [pdfUtils]);

  useEffect(() => {
    annotationFocusActiveRef.current = selectedAnnotationId != null || locatedAnnotationId != null;
  }, [locatedAnnotationId, selectedAnnotationId]);

  const handleFreetextFocusHandled = useCallback((annotationID: number) => {
    setPendingFreetextFocusId((cur) => (cur === annotationID ? null : cur));
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

  const closeReader = useCallback(() => {
    router.push("/");
  }, [router]);

  useEffect(() => {
    if (!id || activePaperID === id) return;
    selectPaper(id);
  }, [activePaperID, id, selectPaper]);

  useEffect(() => {
    const token = loadToken();
    setError("");
    setReady(false);
    setPaper(null);
    setSections([]);
    setNumPages(0);
    setCurrentPage(1);
    setPageDraft("1");
    setAnnotations([]);
    setTranslation(null);
    setPdfUtils(null);
    progressLoadedRef.current = false;
    if (!token) {
      location.replace("/");
      return;
    }
    api.setToken(token);
    if (!id) {
      setError("缺少论文 id");
      return;
    }
    positionPatchSeqRef.current.clear();
    outlineFallbackTriedRef.current = false;
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

      const viewer = pdfViewerWithScale(pdfUtilsRef.current);
      const scrollElement = viewer?.container || pdfWheelRef.current;
      if (!scrollElement || event.deltaY === 0) return;

      event.preventDefault();
      event.stopPropagation();

      const zoom = wheelZoomRef.current;
      zoom.deltaY += wheelDeltaPixels(event, scrollElement.clientHeight);
      zoom.clientX = event.clientX;
      zoom.clientY = event.clientY;
      if (zoom.idleTimer) window.clearTimeout(zoom.idleTimer);
      zoom.idleTimer = window.setTimeout(() => {
        zoom.deltaY = 0;
        zoom.idleTimer = 0;
      }, PDF_WHEEL_ZOOM_IDLE_MS);

      if (zoom.frame) return;
      zoom.frame = window.requestAnimationFrame(() => {
        zoom.frame = 0;
        const deltaY = zoom.deltaY;
        const stepCount = Math.min(
          PDF_WHEEL_ZOOM_MAX_STEPS,
          Math.trunc(Math.abs(deltaY) / PDF_WHEEL_ZOOM_DELTA),
        );
        if (stepCount <= 0) return;
        zoom.deltaY = deltaY - Math.sign(deltaY) * stepCount * PDF_WHEEL_ZOOM_DELTA;

        const currentViewer = pdfViewerWithScale(pdfUtilsRef.current);
        const currentScrollElement = currentViewer?.container || pdfWheelRef.current;
        if (!currentScrollElement) return;

        const viewerScale = currentViewer?.currentScale;
        const refScale = scaleValueRef.current;
        const baseScale =
          typeof refScale === "number"
            ? refScale
            : typeof viewerScale === "number" && Number.isFinite(viewerScale) && viewerScale > 0
              ? viewerScale
              : 1;
        const direction = deltaY < 0 ? 1 : -1;
        const nextScale = roundPdfScale(
          clamp(baseScale + direction * PDF_WHEEL_ZOOM_STEP * stepCount, PDF_MIN_SCALE, PDF_MAX_SCALE),
        );
        if (nextScale === roundPdfScale(baseScale)) {
          zoom.deltaY = 0;
          return;
        }

        const rect = currentScrollElement.getBoundingClientRect();
        const clientX = zoom.clientX;
        const clientY = zoom.clientY;
        const anchorX = currentScrollElement.scrollLeft + clientX - rect.left;
        const anchorY = currentScrollElement.scrollTop + clientY - rect.top;
        const restoreSeq = zoom.restoreSeq + 1;
        zoom.restoreSeq = restoreSeq;
        scaleValueRef.current = nextScale;
        setScaleValue(nextScale);

        window.requestAnimationFrame(() => {
          window.requestAnimationFrame(() => {
            if (wheelZoomRef.current.restoreSeq !== restoreSeq) return;
            const nextViewer = pdfViewerWithScale(pdfUtilsRef.current);
            const nextScrollElement = nextViewer?.container || currentScrollElement;
            const nextRect = nextScrollElement.getBoundingClientRect();
            const ratio = nextScale / baseScale;
            nextScrollElement.scrollLeft = anchorX * ratio - (clientX - nextRect.left);
            nextScrollElement.scrollTop = anchorY * ratio - (clientY - nextRect.top);
          });
        });
      });
    },
    [],
  );

  useEffect(() => {
    const element = pdfWheelRef.current;
    if (!element) return;
    const zoom = wheelZoomRef.current;
    element.addEventListener("wheel", zoomPdfAtWheel, { passive: false, capture: true });
    return () => {
      element.removeEventListener("wheel", zoomPdfAtWheel, { capture: true });
      if (zoom.frame) {
        window.cancelAnimationFrame(zoom.frame);
        zoom.frame = 0;
      }
      if (zoom.idleTimer) {
        window.clearTimeout(zoom.idleTimer);
        zoom.idleTimer = 0;
      }
    };
  }, [zoomPdfAtWheel]);

  const centerCurrentPdfPage = useCallback(() => {
    if (
      annotationFocusActiveRef.current ||
      Date.now() < annotationFocusScrollIgnoreUntilRef.current
    ) {
      return;
    }
    const viewer = pdfViewerWithScale(pdfUtilsRef.current);
    const scrollElement = viewer?.container || pdfWheelRef.current;
    if (!scrollElement || currentPage <= 0) return;

    const pageElement =
      scrollElement.querySelector<HTMLElement>(`.page[data-page-number="${currentPage}"]`) ??
      findPageElement(currentPage);
    if (!pageElement) return;

    const scrollRect = scrollElement.getBoundingClientRect();
    const pageRect = pageElement.getBoundingClientRect();
    const pageCenter = scrollElement.scrollLeft + pageRect.left - scrollRect.left + pageRect.width / 2;
    const nextLeft = Math.max(0, pageCenter - scrollElement.clientWidth / 2);
    scrollElement.scrollTo({ left: nextLeft, top: scrollElement.scrollTop, behavior: "instant" });
  }, [currentPage]);

  useEffect(() => {
    if (!ready) return;
    let secondFrame = 0;
    const firstFrame = window.requestAnimationFrame(() => {
      secondFrame = window.requestAnimationFrame(centerCurrentPdfPage);
    });
    return () => {
      window.cancelAnimationFrame(firstFrame);
      if (secondFrame) window.cancelAnimationFrame(secondFrame);
    };
  }, [
    centerCurrentPdfPage,
    mindMapPanelWidth,
    prefs.mindMapOpen,
    prefs.outlineOpen,
    ready,
    rightPanelOpen,
  ]);

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

  const clearAnnotationFocus = useCallback(() => {
    setSelectedAnnotationId(null);
    clearLocatedAnnotation();
  }, [clearLocatedAnnotation]);

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

  useEffect(() => {
    const viewer = pdfViewerWithScale(pdfUtils);
    const container = viewer?.container || pdfWheelRef.current;
    if (!container) return;

    const clearOnReaderScroll = () => {
      if (Date.now() < annotationFocusScrollIgnoreUntilRef.current) return;
      clearAnnotationFocus();
    };

    container.addEventListener("scroll", clearOnReaderScroll, { passive: true });
    return () => container.removeEventListener("scroll", clearOnReaderScroll);
  }, [clearAnnotationFocus, pdfUtils]);

  const selectAnnotation = useCallback(
    (annotation: PaperAnnotation) => {
      setSelectedAnnotationId(annotation.id);
      updatePrefs({ annotationsOpen: true, mindMapOpen: false });
      scrollSideAnnotationIntoView(annotation.id);
    },
    [updatePrefs],
  );

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
      clearAnnotationFocus();
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
    [clearAnnotationFocus, clearOutlineActive, numPages, paper?.page_count, pdfUtils],
  );

  const goToAnnotation = useCallback(
    (annotationID: number, pageNumber?: number, openAnnotations = false) => {
      const annotation = annotations.find((item) => item.id === annotationID);
      const page = annotation?.page_no || pageNumber || 1;
      setSelectedAnnotationId(annotationID);
      if (openAnnotations) {
        updatePrefs({ annotationsOpen: true, mindMapOpen: false, qaOpen: false });
        if (annotation?.note) {
          setExpandedAnnotationNoteIDs((prev) => new Set(prev).add(annotation.id));
        }
        scrollSideAnnotationIntoView(annotationID);
      }
      clearOutlineActive();
      setCurrentPage(page);
      setPageDraft(String(page));
      if (annotation && pdfUtils) {
        annotationFocusScrollIgnoreUntilRef.current = Date.now() + LOCATED_ANNOTATION_SCROLL_RESUME_MS;
        const container = scrollSplitHighlightToTop(pdfUtils, splitAnnotationToHighlight(annotation));
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
      if (!id || annotation.kind === "drawing" || !annotation.text.trim()) return;
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
          kind: "selection",
          text,
          note,
          color,
          bounding_rect: scaledToRect(selection.position.boundingRect),
          rects: selection.position.rects.map(scaledToRect),
        });
        setAnnotations((prev) => [annotation, ...prev]);
        setSelectedAnnotationId(annotation.id);
        updatePrefs({ annotationsOpen: true, mindMapOpen: false, qaOpen: false });
        scrollSideAnnotationIntoView(annotation.id);
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
      const text = selection.content.text?.trim();
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
        setPendingFreetextFocusId((cur) => (cur === annotation.id ? null : cur));
        setSelectedAnnotationId((cur) => (cur === annotation.id ? null : cur));
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

  useEffect(() => {
    const handleDeleteSelectedAnnotation = (event: KeyboardEvent) => {
      if (event.key !== "Delete") return;
      if (event.defaultPrevented || event.altKey || event.ctrlKey || event.metaKey) return;
      const target = event.target;
      if (
        target instanceof HTMLElement &&
        target.closest("input, textarea, select, [contenteditable='true']")
      ) {
        return;
      }
      if (busy || selectedAnnotationId == null) return;

      const annotation = visibleAnnotations.find((item) => item.id === selectedAnnotationId);
      if (!annotation) return;

      event.preventDefault();
      void deleteAnnotation(annotation);
    };

    document.addEventListener("keydown", handleDeleteSelectedAnnotation);
    return () => document.removeEventListener("keydown", handleDeleteSelectedAnnotation);
  }, [busy, deleteAnnotation, selectedAnnotationId, visibleAnnotations]);

  const changeAnnotationColor = useCallback(
    async (annotation: PaperAnnotation, color: AnnotationColor) => {
      setBusy(`color-${annotation.id}`);
      try {
        const payload = await annotationColorPatch(annotation, color);
        const updated = await api.updateAnnotation(id, annotation.id, payload);
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

  const createFreetextAnnotation = useCallback(
    async (position: ScaledPosition) => {
      if (!id) return;
      if (freetextCreateInFlightRef.current) return;
      const normalizedPosition = defaultFreetextPosition(position);
      const { boundingRect, rects } = positionToRects(normalizedPosition);
      if (isRecentFreetextCreate(recentFreetextCreateRef.current, boundingRect)) return;
      freetextCreateInFlightRef.current = true;
      recentFreetextCreateRef.current = {
        pageNumber: boundingRect.pageNumber,
        x1: boundingRect.x1,
        y1: boundingRect.y1,
        until: Date.now() + FREETEXT_CREATE_COOLDOWN_MS,
      };
      updatePrefs({ activeTool: "select" });
      setBusy("freetext");
      try {
        const annotation = await api.createAnnotation(id, {
          page_no: boundingRect.pageNumber,
          kind: "freetext",
          text: FREETEXT_CREATE_TEXT,
          color: prefs.color,
          bounding_rect: boundingRect,
          rects,
          style_json: freetextStyleForColor(prefs.color, prefs.textSize, boundingRect.width),
        });
        setAnnotations((prev) => [{ ...annotation, text: FREETEXT_EMPTY_DRAFT }, ...prev]);
        setSelectedAnnotationId(annotation.id);
        setPendingFreetextFocusId(annotation.id);
        updatePrefs({ activeTool: "select", annotationsOpen: true, mindMapOpen: false, qaOpen: false });
        scrollSideAnnotationIntoView(annotation.id);
      } catch (err) {
        setError((err as Error)?.message || "保存文字批注失败");
      } finally {
        freetextCreateInFlightRef.current = false;
        setBusy("");
      }
    },
    [id, prefs.color, prefs.textSize, updatePrefs],
  );

  const createDrawingAnnotation = useCallback(
    async (
      image: string,
      position: ScaledPosition,
      strokes: DrawingStroke[],
      meta: DrawingCreateMeta,
    ) => {
      if (!id || !image || strokes.length === 0) return false;
      const { boundingRect, rects } = positionToRects(position);
      const style = drawingStyleForColor(prefs.color, prefs.drawingSize);
      setBusy("drawing");
      try {
        const annotation = await api.createAnnotation(id, {
          page_no: boundingRect.pageNumber,
          kind: "drawing",
          text: "",
          color: prefs.color,
          bounding_rect: boundingRect,
          rects,
          style_json: style,
          content_json: {
            image,
            strokes,
            canvasWidth: meta.width,
            canvasHeight: meta.height,
            imageWidth: meta.width,
            imageHeight: meta.height,
            strokeCoordinateSpace: "local",
            ...(meta.snapshot ? { snapshot: meta.snapshot } : {}),
          },
        });
        setAnnotations((prev) => [annotation, ...prev]);
        setSelectedAnnotationId(annotation.id);
        updatePrefs({ annotationsOpen: true, mindMapOpen: false, qaOpen: false });
        scrollSideAnnotationIntoView(annotation.id);
        return true;
      } catch (err) {
        setError((err as Error)?.message || "保存绘画批注失败");
        return false;
      } finally {
        setBusy("");
      }
    },
    [id, prefs.color, prefs.drawingSize, updatePrefs],
  );

  const patchAnnotation = useCallback(
    async (annotation: PaperAnnotation, payload: Parameters<typeof api.updateAnnotation>[2], busyKey: string) => {
      setBusy(busyKey);
      try {
        const updated = await api.updateAnnotation(id, annotation.id, payload);
        setAnnotations((prev) => prev.map((item) => (item.id === updated.id ? updated : item)));
      } catch (err) {
        setError((err as Error)?.message || "更新批注失败");
      } finally {
        setBusy("");
      }
    },
    [id],
  );

  const updateAnnotationPosition = useCallback(
    (annotation: PaperAnnotation, position: ScaledPosition, snapshot?: string) => {
      const { boundingRect, rects } = positionToRects(position);
      const nextContentJson =
        annotation.kind === "drawing" && snapshot
          ? {
              ...(annotation.content_json ?? {}),
              snapshot,
            }
          : undefined;
      if (
        sameAnnotationRect(annotation.bounding_rect, boundingRect) &&
        sameAnnotationRects(annotation.rects, rects) &&
        !nextContentJson
      ) {
        return;
      }

      const seq = (positionPatchSeqRef.current.get(annotation.id) ?? 0) + 1;
      positionPatchSeqRef.current.set(annotation.id, seq);
      const previousPageNo = annotation.page_no;
      const previousBoundingRect = annotation.bounding_rect;
      const previousRects = annotation.rects;
      const previousContentJson = annotation.content_json;

      setAnnotations((prev) =>
        prev.map((item) =>
          item.id === annotation.id
            ? {
                ...annotationWithPosition(item, boundingRect, rects),
                ...(nextContentJson
                  ? {
                      content_json: {
                        ...(item.content_json ?? {}),
                        snapshot,
                      },
                    }
                  : {}),
              }
            : item,
        ),
      );

      api
        .updateAnnotation(id, annotation.id, {
          bounding_rect: boundingRect,
          rects,
          ...(nextContentJson ? { content_json: nextContentJson } : {}),
        })
        .then((updated) => {
          if (positionPatchSeqRef.current.get(annotation.id) !== seq) return;
          setAnnotations((prev) =>
            prev.map((item) =>
              item.id === updated.id
                ? {
                    ...item,
                    page_no: updated.page_no,
                    bounding_rect: updated.bounding_rect,
                    rects: updated.rects,
                    ...(nextContentJson ? { content_json: updated.content_json } : {}),
                    updated_at: updated.updated_at,
                  }
                : item,
            ),
          );
        })
        .catch((err) => {
          if (positionPatchSeqRef.current.get(annotation.id) !== seq) return;
          setAnnotations((prev) =>
            prev.map((item) =>
              item.id === annotation.id
                ? {
                    ...item,
                    page_no: previousPageNo,
                    bounding_rect: previousBoundingRect,
                    rects: previousRects,
                    content_json: previousContentJson,
                  }
                : item,
            ),
          );
          setError((err as Error)?.message || "鏇存柊鎵规敞澶辫触");
        })
        .finally(() => {
          if (positionPatchSeqRef.current.get(annotation.id) === seq) {
            positionPatchSeqRef.current.delete(annotation.id);
          }
        });
    },
    [id],
  );

  const updateAnnotationText = useCallback(
    (annotation: PaperAnnotation, text: string) => {
      const nextText = normalizeFreetextText(text);
      if (annotation.kind === "freetext" && (!nextText || nextText === FREETEXT_CREATE_TEXT)) {
        void deleteAnnotation(annotation);
        return;
      }
      void patchAnnotation(annotation, { text: nextText }, `text-${annotation.id}`);
    },
    [deleteAnnotation, patchAnnotation],
  );

  const clearAnnotationSelectionFromOutside = useCallback(
    (event: ReactPointerEvent<HTMLElement>) => {
      if (selectedAnnotationId == null) return;
      if (shouldKeepAnnotationSelection(event.target)) return;
      clearAnnotationFocus();
    },
    [clearAnnotationFocus, selectedAnnotationId],
  );

  const zoomBy = (delta: number) => {
    saveDrawingDraftRef.current?.();
    setScaleValue((cur) => {
      const base = typeof cur === "number" ? cur : 1;
      return Number((clamp(base + delta, PDF_MIN_SCALE, PDF_MAX_SCALE)).toFixed(2));
    });
  };

  const setZoomValue = (value: PdfScaleValue) => {
    saveDrawingDraftRef.current?.();
    setScaleValue(value);
  };

  return (
    <main
      className="flex h-dvh flex-col overflow-hidden bg-muted/50"
      onPointerDownCapture={clearAnnotationSelectionFromOutside}
    >
      <CompactReaderToolbar
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
        onResetZoom={() => setZoomValue(1)}
        onFitWidth={() => setZoomValue("page-width")}
        onToggleTranslate={toggleTranslatePanel}
        onToggleAnnotations={toggleAnnotationsPanel}
        onToggleMindMap={toggleMindMapPanel}
        onToggleQA={toggleQAPanel}
        onToolChange={changeActiveTool}
        onColorChange={(color) => updatePrefs({ color })}
        onTextSizeChange={(textSize) => updatePrefs({ textSize })}
        onDrawingSizeChange={(drawingSize) => updatePrefs({ drawingSize })}
        onClearDrawingDraft={() => clearDrawingDraftRef.current?.()}
      />

      <div ref={mindMapGridRef} className={cn("grid min-h-0 flex-1 grid-cols-1", gridClass)} style={gridStyle}>
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
          <CompactReaderLeftRail
            outlineOpen={prefs.outlineOpen}
            onToggleOutline={() => updatePrefs({ outlineOpen: true })}
          />
          <div ref={pdfWheelRef} className="relative h-full min-h-0 overflow-hidden">
            {ready && pdfUrl ? (
              <InteractiveReaderPdf
                pdfUrl={pdfUrl}
                highlights={highlights}
                initialPage={initialPage}
                scaleValue={scaleValue}
                activeTool={prefs.activeTool}
                onPageChange={handlePdfPageChange}
                onPageCount={setPageCount}
                onTranslateSelection={onTranslateSelection}
                onSaveHighlight={(selection) => void saveAnnotation(selection, "", prefs.color)}
                onAnnotateSelection={onAnnotateSelection}
                onAskSelection={onAskSelection}
                onCreateFreetext={(position) => void createFreetextAnnotation(position)}
                drawingColor={drawingStyleForColor(prefs.color, prefs.drawingSize).strokeColor}
                drawingSize={prefs.drawingSize}
                onCreateDrawing={(image, position, strokes, meta) =>
                  createDrawingAnnotation(image, position, strokes, meta)
                }
                onDrawingDraftClearReady={(clear) => {
                  clearDrawingDraftRef.current = clear;
                }}
                onDrawingDraftSaveReady={(save) => {
                  saveDrawingDraftRef.current = save;
                }}
                onUpdateAnnotationPosition={updateAnnotationPosition}
                onUpdateAnnotationText={updateAnnotationText}
                onDeleteAnnotation={deleteAnnotation}
                locatedAnnotationId={locatedAnnotationId}
                selectedAnnotationId={selectedAnnotationId}
                pendingFreetextFocusId={pendingFreetextFocusId}
                onSelectAnnotation={selectAnnotation}
                onFreetextFocusHandled={handleFreetextFocusHandled}
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
          <SplitReaderSidePanel
            showTranslation={prefs.translateOpen}
            showAnnotations={prefs.annotationsOpen}
            translation={translation}
            annotations={visibleAnnotations}
            selectedAnnotationId={selectedAnnotationId}
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
            onLocateAnnotation={(annotation) => goToAnnotation(annotation.id, annotation.page_no, true)}
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
