"use client";

import "@/lib/pdfjs-global";
import {
  BookMarked,
  Languages,
  Loader2,
  MessageCircleQuestionMark,
  MessageSquarePlus,
} from "lucide-react";
import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
  type KeyboardEvent as ReactKeyboardEvent,
  type MouseEvent as ReactMouseEvent,
  type ReactNode,
} from "react";
import {
  Rnd,
  type DraggableData,
  type RndDragEvent,
  type RndResizeCallback,
} from "react-rnd";
import {
  GlobalWorkerOptions,
  getDocument,
  type OnProgressParameters,
  type PDFDocumentProxy,
} from "pdfjs-dist";
import type { DocumentInitParameters } from "pdfjs-dist/types/src/display/api";
import {
  PdfHighlighter,
  TextHighlight,
  scaledPositionToViewport,
  useHighlightContainerContext,
  usePdfHighlighterContext,
  viewportPositionToScaled,
  type DrawingStroke,
  type LTWHP,
  type PdfHighlighterUtils,
  type PdfScaleValue,
  type PdfSelection,
  type ScaledPosition,
  type ViewportHighlight,
} from "react-pdf-highlighter-plus";

import { Button } from "@/components/ui/button";
import type { PaperAnnotation } from "@/lib/gopherpaper/types";
import {
  annotationToHighlight,
  colorValue,
  DRAWING_IDLE_SAVE_MS,
  FREETEXT_DEFAULT_HEIGHT,
  FREETEXT_DEFAULT_WIDTH,
  freetextStyle,
  normalizeFreetextText,
  type ReaderHighlight,
  type ReaderTool,
} from "@/app/reader/lib/annotations";

const PDF_WORKER = "/pdfjs/pdf.worker.min.mjs";
const LOCATE_TOP_GAP = 32;
const ANNOTATION_DRAGGING_CLASS = "reader-annotation-dragging";
const FREETEXT_MIN_WIDTH = 24;
const FREETEXT_TEXT_MIN_HEIGHT = 18;
const FREETEXT_HEIGHT_EPSILON = 1;
const FREETEXT_DRAG_EPSILON = 1;
const FREETEXT_RESIZE_ENABLE = {
  top: false,
  right: true,
  bottom: false,
  left: true,
  topRight: false,
  bottomRight: false,
  bottomLeft: false,
  topLeft: false,
};
const FREETEXT_RESIZE_HANDLE_STYLES = {
  left: {
    left: -6,
    width: 12,
    cursor: "ew-resize",
  },
  right: {
    right: -6,
    width: 12,
    cursor: "ew-resize",
  },
};
const FREETEXT_RESIZE_HANDLE_CLASSES = {
  left: "FreetextHighlight__resize-handle-left",
  right: "FreetextHighlight__resize-handle-right",
};

type EventBusCallback = (evt: { pageNumber?: number } | unknown) => void;

interface EventBusLike {
  on: (event: string, callback: EventBusCallback) => void;
  off: (event: string, callback: EventBusCallback) => void;
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

const horizontalLocateFrames = new WeakMap<HTMLElement, number>();

function prefersReducedMotion() {
  return typeof window !== "undefined" && window.matchMedia("(prefers-reduced-motion: reduce)").matches;
}

function smoothHorizontalLocate(container: HTMLElement, fromLeft: number, toLeft: number) {
  const currentFrame = horizontalLocateFrames.get(container);
  if (currentFrame != null) {
    window.cancelAnimationFrame(currentFrame);
    horizontalLocateFrames.delete(container);
  }

  const delta = toLeft - fromLeft;
  if (prefersReducedMotion() || Math.abs(delta) < 4) {
    container.scrollLeft = toLeft;
    return;
  }

  container.scrollTo({ left: fromLeft, top: container.scrollTop, behavior: "auto" });
  horizontalLocateFrames.set(
    container,
    window.requestAnimationFrame(() => {
      horizontalLocateFrames.delete(container);
      container.scrollTo({ left: toLeft, top: container.scrollTop, behavior: "smooth" });
    }),
  );
}

function smoothSamePageLocate(container: HTMLElement, rect: LTWHP) {
  const page = container.querySelector<HTMLElement>(`.page[data-page-number="${rect.pageNumber}"]`);
  if (!page) return false;

  const pageRect = page.getBoundingClientRect();
  if (pageRect.width <= 0 || pageRect.height <= 0) return false;

  const containerRect = container.getBoundingClientRect();
  const pageLeft = container.scrollLeft + pageRect.left - containerRect.left;
  const pageTop = container.scrollTop + pageRect.top - containerRect.top;
  const left = Math.max(0, pageLeft + rect.left + rect.width / 2 - container.clientWidth / 2);
  const top = Math.max(0, pageTop + rect.top - LOCATE_TOP_GAP);

  container.scrollTo({
    left,
    top,
    behavior: prefersReducedMotion() ? "auto" : "smooth",
  });
  return true;
}

export function scrollHighlightToTop(
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
  const container = viewer.container ?? null;
  const rect = viewportPosition.boundingRect;
  const targetLeft = Math.max(0, rect.left + rect.width / 2 - (container?.clientWidth ?? 0) / 2);
  const targetTop = Math.max(0, rect.top - LOCATE_TOP_GAP);
  const previousLeft = container?.scrollLeft ?? 0;
  const currentPageNumber = (viewer as { currentPageNumber?: unknown }).currentPageNumber;

  if (container && currentPageNumber === pageNumber && smoothSamePageLocate(container, rect)) {
    return container;
  }

  viewer.scrollPageIntoView({
    pageNumber,
    destArray: [
      null,
      { name: "XYZ" },
      ...viewport.convertToPdfPoint(targetLeft, targetTop),
      0,
    ],
  });
  if (container) {
    smoothHorizontalLocate(container, previousLeft, container.scrollLeft);
  }
  return container;
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
        <MessageSquarePlus className="size-3.5" />
        批注
      </Button>
      <Button type="button" size="sm" variant="ghost" onClick={() => applySelection(onAsk)}>
        <MessageCircleQuestionMark className="size-3.5" />
        问答
      </Button>
    </div>
  );
}


function shouldIgnoreAnnotationClick(target: EventTarget | null) {
  if (!(target instanceof HTMLElement)) return false;
  return Boolean(
    target.closest(
      [
        "button",
        "input",
        "textarea",
        "select",
        ".FreetextHighlight__toolbar",
        ".FreetextHighlight__style-panel",
        ".FreetextHighlight__input",
        ".DrawingHighlight__toolbar",
        ".DrawingHighlight__style-controls",
      ].join(", "),
    ),
  );
}

function lockAnnotationTextSelection() {
  document.body.classList.add(ANNOTATION_DRAGGING_CLASS);
}

function unlockAnnotationTextSelection() {
  document.body.classList.remove(ANNOTATION_DRAGGING_CLASS);
}

function scaledFromViewportRect(rect: LTWHP, utils: PdfHighlighterUtils) {
  const viewer = utils.getViewer();
  if (!viewer) return null;
  return viewportPositionToScaled({ boundingRect: rect, rects: [] }, viewer);
}

function finitePositiveNumber(value: unknown) {
  return typeof value === "number" && Number.isFinite(value) && value > 0 ? value : undefined;
}

function freetextMinWidth(fontSize: string) {
  const size = Number.parseFloat(fontSize);
  const textColumn = Number.isFinite(size) && size > 0 ? size : FREETEXT_TEXT_MIN_HEIGHT;
  return Math.max(FREETEXT_MIN_WIDTH, Math.ceil(textColumn * 1.25 + 18));
}

function annotationViewportScale(annotation: PaperAnnotation, rect: LTWHP, basePageWidth?: number) {
  const storedPageWidth = finitePositiveNumber(annotation.bounding_rect?.width);
  if (!storedPageWidth) return 1;
  const scaledRectWidth = annotation.bounding_rect.x2 - annotation.bounding_rect.x1;
  if (!Number.isFinite(scaledRectWidth) || scaledRectWidth <= 0) return 1;
  const currentPageWidth = rect.width * (storedPageWidth / scaledRectWidth);
  if (!Number.isFinite(currentPageWidth) || currentPageWidth <= 0) return 1;
  return currentPageWidth / (finitePositiveNumber(basePageWidth) ?? storedPageWidth);
}

function drawingPath(points: DrawingStroke["points"]) {
  if (points.length === 0) return "";
  const [first, ...rest] = points;
  return [`M ${first.x} ${first.y}`, ...rest.map((point) => `L ${point.x} ${point.y}`)].join(" ");
}

function drawingViewBox(annotation: PaperAnnotation, strokes: DrawingStroke[]) {
  const content = annotation.content_json ?? {};
  const storedWidth =
    finitePositiveNumber(content.canvasWidth) ??
    finitePositiveNumber(content.canvas_width) ??
    finitePositiveNumber(content.imageWidth) ??
    finitePositiveNumber(content.image_width);
  const storedHeight =
    finitePositiveNumber(content.canvasHeight) ??
    finitePositiveNumber(content.canvas_height) ??
    finitePositiveNumber(content.imageHeight) ??
    finitePositiveNumber(content.image_height);
  let maxX = storedWidth ?? 1;
  let maxY = storedHeight ?? 1;

  for (const stroke of strokes) {
    const padding = Math.max(1, stroke.width || 1) * 2;
    for (const point of stroke.points) {
      maxX = Math.max(maxX, point.x + padding);
      maxY = Math.max(maxY, point.y + padding);
    }
  }

  return { width: Math.ceil(maxX), height: Math.ceil(maxY) };
}

function drawingStrokeBounds(strokes: DrawingStroke[]) {
  let minX = Infinity;
  let minY = Infinity;
  let maxX = -Infinity;
  let maxY = -Infinity;

  for (const stroke of strokes) {
    for (const point of stroke.points) {
      minX = Math.min(minX, point.x);
      minY = Math.min(minY, point.y);
      maxX = Math.max(maxX, point.x);
      maxY = Math.max(maxY, point.y);
    }
  }

  if (!Number.isFinite(minX) || !Number.isFinite(minY)) return null;
  return { minX, minY, maxX, maxY };
}

function drawingRenderStrokes(annotation: PaperAnnotation, strokes: DrawingStroke[]) {
  const bounds = drawingStrokeBounds(strokes);
  if (!bounds) return strokes;

  const content = annotation.content_json ?? {};
  const coordinateSpace =
    typeof content.strokeCoordinateSpace === "string"
      ? content.strokeCoordinateSpace
      : typeof content.stroke_coordinate_space === "string"
        ? content.stroke_coordinate_space
        : "";
  if (coordinateSpace === "local") return strokes;

  const localWidth =
    finitePositiveNumber(content.canvasWidth) ??
    finitePositiveNumber(content.canvas_width) ??
    finitePositiveNumber(content.imageWidth) ??
    finitePositiveNumber(content.image_width) ??
    finitePositiveNumber(annotation.bounding_rect.x2 - annotation.bounding_rect.x1);
  const localHeight =
    finitePositiveNumber(content.canvasHeight) ??
    finitePositiveNumber(content.canvas_height) ??
    finitePositiveNumber(content.imageHeight) ??
    finitePositiveNumber(content.image_height) ??
    finitePositiveNumber(annotation.bounding_rect.y2 - annotation.bounding_rect.y1);
  if (!localWidth || !localHeight) return strokes;

  const maxStrokeWidth = Math.max(1, ...strokes.map((stroke) => stroke.width || 1));
  const tolerance = maxStrokeWidth * 4;
  const pageBasedX = coordinateSpace === "page" || bounds.maxX > localWidth + tolerance;
  const pageBasedY = coordinateSpace === "page" || bounds.maxY > localHeight + tolerance;
  if (!pageBasedX && !pageBasedY) return strokes;

  const offsetX = pageBasedX ? annotation.bounding_rect.x1 : 0;
  const offsetY = pageBasedY ? annotation.bounding_rect.y1 : 0;
  return strokes.map((stroke) => ({
    ...stroke,
    points: stroke.points.map((point) => ({
      ...point,
      x: point.x - offsetX,
      y: point.y - offsetY,
    })),
  }));
}

type DrawingPoint = DrawingStroke["points"][number];

interface DrawingPageTarget {
  pageNumber: number;
  element: HTMLElement;
  width: number;
  height: number;
}

interface DrawingDraftBounds {
  left: number;
  top: number;
  width: number;
  height: number;
}

export interface DrawingSaveMeta {
  width: number;
  height: number;
  snapshot?: string;
}

function clampDrawingValue(value: number, min: number, max: number) {
  return Math.min(max, Math.max(min, value));
}

function drawingHasInk(strokes: DrawingStroke[], currentStroke?: DrawingStroke | null) {
  if (currentStroke?.points.length) return true;
  return strokes.some((stroke) => stroke.points.length > 0);
}

function drawDrawingStroke(ctx: CanvasRenderingContext2D, stroke: DrawingStroke) {
  const points = stroke.points;
  if (points.length === 0) return;

  ctx.lineCap = "round";
  ctx.lineJoin = "round";
  ctx.strokeStyle = stroke.color;
  ctx.lineWidth = Math.max(1, stroke.width || 1);

  if (points.length === 1) {
    ctx.fillStyle = stroke.color;
    ctx.beginPath();
    ctx.arc(points[0].x, points[0].y, Math.max(1, ctx.lineWidth / 2), 0, Math.PI * 2);
    ctx.fill();
    return;
  }

  ctx.beginPath();
  ctx.moveTo(points[0].x, points[0].y);
  for (const point of points.slice(1)) {
    ctx.lineTo(point.x, point.y);
  }
  ctx.stroke();
}

function pageTargetAtPoint(container: HTMLElement, clientX: number, clientY: number): DrawingPageTarget | null {
  const pages = Array.from(container.querySelectorAll<HTMLElement>(".page[data-page-number]"));
  for (const page of pages) {
    const rect = page.getBoundingClientRect();
    if (
      rect.width <= 0 ||
      rect.height <= 0 ||
      clientX < rect.left ||
      clientX > rect.right ||
      clientY < rect.top ||
      clientY > rect.bottom
    ) {
      continue;
    }
    const pageNumber = Number(page.dataset.pageNumber);
    if (!Number.isFinite(pageNumber) || pageNumber <= 0) return null;
    return { pageNumber, element: page, width: rect.width, height: rect.height };
  }
  return null;
}

function pointForPageEvent(page: HTMLElement, event: PointerEvent): DrawingPoint {
  const rect = page.getBoundingClientRect();
  return {
    x: clampDrawingValue(event.clientX - rect.left, 0, Math.max(1, rect.width)),
    y: clampDrawingValue(event.clientY - rect.top, 0, Math.max(1, rect.height)),
  };
}

function setDrawingCanvasRect(
  canvas: HTMLCanvasElement,
  left: number,
  top: number,
  width: number,
  height: number,
) {
  const cssWidth = Math.max(1, Math.ceil(width));
  const cssHeight = Math.max(1, Math.ceil(height));
  const dpr = Math.max(1, window.devicePixelRatio || 1);
  const pixelWidth = Math.max(1, Math.ceil(cssWidth * dpr));
  const pixelHeight = Math.max(1, Math.ceil(cssHeight * dpr));

  canvas.style.left = `${left}px`;
  canvas.style.top = `${top}px`;
  canvas.style.width = `${cssWidth}px`;
  canvas.style.height = `${cssHeight}px`;
  if (canvas.width !== pixelWidth) canvas.width = pixelWidth;
  if (canvas.height !== pixelHeight) canvas.height = pixelHeight;

  const ctx = canvas.getContext("2d");
  ctx?.setTransform(dpr, 0, 0, dpr, 0, 0);
  return { width: cssWidth, height: cssHeight };
}

function drawingPageCanvasRect(container: HTMLElement, page: HTMLElement) {
  const containerRect = container.getBoundingClientRect();
  const pageRect = page.getBoundingClientRect();
  return {
    left: container.scrollLeft + pageRect.left - containerRect.left,
    top: container.scrollTop + pageRect.top - containerRect.top,
    width: pageRect.width,
    height: pageRect.height,
  };
}

function drawingBounds(strokes: DrawingStroke[], pageWidth: number, pageHeight: number): DrawingDraftBounds | null {
  let minX = Infinity;
  let minY = Infinity;
  let maxX = -Infinity;
  let maxY = -Infinity;

  for (const stroke of strokes) {
    const padding = Math.max(1, stroke.width || 1) * 2;
    for (const point of stroke.points) {
      minX = Math.min(minX, point.x - padding);
      minY = Math.min(minY, point.y - padding);
      maxX = Math.max(maxX, point.x + padding);
      maxY = Math.max(maxY, point.y + padding);
    }
  }

  if (!Number.isFinite(minX) || !Number.isFinite(minY)) return null;

  const left = clampDrawingValue(Math.floor(minX), 0, Math.max(0, pageWidth - 1));
  const top = clampDrawingValue(Math.floor(minY), 0, Math.max(0, pageHeight - 1));
  const right = clampDrawingValue(Math.ceil(maxX), left + 1, pageWidth);
  const bottom = clampDrawingValue(Math.ceil(maxY), top + 1, pageHeight);
  return { left, top, width: Math.max(1, right - left), height: Math.max(1, bottom - top) };
}

function localDrawingStrokes(strokes: DrawingStroke[], bounds: DrawingDraftBounds) {
  return strokes.map((stroke) => ({
    ...stroke,
    points: stroke.points.map((point) => ({
      x: point.x - bounds.left,
      y: point.y - bounds.top,
    })),
  }));
}

function drawingStrokesToImage(strokes: DrawingStroke[], width: number, height: number) {
  const canvas = document.createElement("canvas");
  canvas.width = Math.max(1, Math.ceil(width));
  canvas.height = Math.max(1, Math.ceil(height));
  const ctx = canvas.getContext("2d");
  if (!ctx) return "";
  for (const stroke of strokes) {
    drawDrawingStroke(ctx, stroke);
  }
  return canvas.toDataURL("image/png");
}

function drawingSnapshotImage(page: HTMLElement, bounds: DrawingDraftBounds) {
  const source = page.querySelector<HTMLCanvasElement>(".canvasWrapper canvas, canvas");
  if (!source || source.width <= 0 || source.height <= 0) return "";

  const pageRect = page.getBoundingClientRect();
  const sourceRect = source.getBoundingClientRect();
  if (sourceRect.width <= 0 || sourceRect.height <= 0) return "";

  const scaleX = source.width / sourceRect.width;
  const scaleY = source.height / sourceRect.height;
  const sourceOffsetX = sourceRect.left - pageRect.left;
  const sourceOffsetY = sourceRect.top - pageRect.top;
  const rawLeft = (bounds.left - sourceOffsetX) * scaleX;
  const rawTop = (bounds.top - sourceOffsetY) * scaleY;
  const rawRight = rawLeft + bounds.width * scaleX;
  const rawBottom = rawTop + bounds.height * scaleY;
  const sourceLeft = clampDrawingValue(Math.floor(rawLeft), 0, Math.max(0, source.width - 1));
  const sourceTop = clampDrawingValue(Math.floor(rawTop), 0, Math.max(0, source.height - 1));
  const sourceRight = clampDrawingValue(Math.ceil(rawRight), sourceLeft + 1, source.width);
  const sourceBottom = clampDrawingValue(Math.ceil(rawBottom), sourceTop + 1, source.height);
  const output = document.createElement("canvas");
  output.width = Math.max(1, Math.ceil(bounds.width));
  output.height = Math.max(1, Math.ceil(bounds.height));
  const ctx = output.getContext("2d");
  if (!ctx) return "";

  ctx.fillStyle = "#ffffff";
  ctx.fillRect(0, 0, output.width, output.height);
  ctx.drawImage(
    source,
    sourceLeft,
    sourceTop,
    sourceRight - sourceLeft,
    sourceBottom - sourceTop,
    0,
    0,
    output.width,
    output.height,
  );
  return output.toDataURL("image/png");
}

function ReaderDrawingLayer({
  active,
  utils,
  strokeColor,
  strokeWidth,
  onCreateDrawing,
  onSaveReady,
}: {
  active: boolean;
  utils: PdfHighlighterUtils | null;
  strokeColor: string;
  strokeWidth: number;
  onCreateDrawing: (
    image: string,
    position: ScaledPosition,
    strokes: DrawingStroke[],
    meta: DrawingSaveMeta,
  ) => Promise<boolean> | boolean;
  onSaveReady: (save: () => void) => void;
}) {
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const containerRef = useRef<HTMLElement | null>(null);
  const utilsRef = useRef<PdfHighlighterUtils | null>(utils);
  const pageRef = useRef<DrawingPageTarget | null>(null);
  const strokesRef = useRef<DrawingStroke[]>([]);
  const currentStrokeRef = useRef<DrawingStroke | null>(null);
  const drawingRef = useRef(false);
  const canvasSizeRef = useRef({ width: 1, height: 1 });
  const saveTimerRef = useRef<number | null>(null);
  const onCreateDrawingRef = useRef(onCreateDrawing);
  const strokeStyleRef = useRef({ strokeColor, strokeWidth });

  useEffect(() => {
    utilsRef.current = utils;
  }, [utils]);

  useEffect(() => {
    onCreateDrawingRef.current = onCreateDrawing;
  }, [onCreateDrawing]);

  useEffect(() => {
    strokeStyleRef.current = { strokeColor, strokeWidth };
  }, [strokeColor, strokeWidth]);

  const clearSaveTimer = useCallback(() => {
    if (saveTimerRef.current == null) return;
    window.clearTimeout(saveTimerRef.current);
    saveTimerRef.current = null;
  }, []);

  const redraw = useCallback(() => {
    const canvas = canvasRef.current;
    const ctx = canvas?.getContext("2d");
    if (!canvas || !ctx) return;
    const { width, height } = canvasSizeRef.current;
    ctx.clearRect(0, 0, width, height);
    for (const stroke of strokesRef.current) {
      drawDrawingStroke(ctx, stroke);
    }
    if (currentStrokeRef.current) {
      drawDrawingStroke(ctx, currentStrokeRef.current);
    }
  }, []);

  const placeCanvasOnViewport = useCallback(() => {
    const canvas = canvasRef.current;
    const container = containerRef.current;
    if (!canvas || !container) return;
    canvasSizeRef.current = setDrawingCanvasRect(
      canvas,
      container.scrollLeft,
      container.scrollTop,
      container.clientWidth,
      container.clientHeight,
    );
    redraw();
  }, [redraw]);

  const clearDraft = useCallback(() => {
    clearSaveTimer();
    strokesRef.current = [];
    currentStrokeRef.current = null;
    drawingRef.current = false;
    pageRef.current = null;
    placeCanvasOnViewport();
  }, [clearSaveTimer, placeCanvasOnViewport]);

  const saveDraft = useCallback(() => {
    clearSaveTimer();
    const utils = utilsRef.current;
    const viewer = utils?.getViewer();
    const target = pageRef.current;
    const strokes = [...strokesRef.current];
    if (currentStrokeRef.current?.points.length) {
      strokes.push(currentStrokeRef.current);
    }

    if (!viewer || !target || !drawingHasInk(strokes)) {
      clearDraft();
      return;
    }

    const bounds = drawingBounds(strokes, target.width, target.height);
    if (!bounds) {
      clearDraft();
      return;
    }

    const localStrokes = localDrawingStrokes(strokes, bounds);
    const image = drawingStrokesToImage(localStrokes, bounds.width, bounds.height);
    const snapshot = drawingSnapshotImage(target.element, bounds);
    if (!image) {
      clearDraft();
      return;
    }

    const position = viewportPositionToScaled(
      {
        boundingRect: {
          pageNumber: target.pageNumber,
          left: bounds.left,
          top: bounds.top,
          width: bounds.width,
          height: bounds.height,
        },
        rects: [],
      },
      viewer,
    );

    clearDraft();
    void Promise.resolve(
      onCreateDrawingRef.current(image, position, localStrokes, {
        width: bounds.width,
        height: bounds.height,
        snapshot,
      }),
    );
  }, [clearDraft, clearSaveTimer]);

  const scheduleSave = useCallback(() => {
    clearSaveTimer();
    saveTimerRef.current = window.setTimeout(() => {
      saveTimerRef.current = null;
      saveDraft();
    }, DRAWING_IDLE_SAVE_MS);
  }, [clearSaveTimer, saveDraft]);

  useEffect(() => {
    onSaveReady(saveDraft);
    return () => {
      onSaveReady(() => {});
    };
  }, [onSaveReady, saveDraft]);

  useEffect(() => {
    if (!active || !utils) return;
    const viewer = utils.getViewer();
    const container = viewer?.container;
    if (!container) return;

    const canvas = document.createElement("canvas");
    canvas.className = "ReaderDrawingCanvas";
    canvas.setAttribute("aria-hidden", "true");
    canvasRef.current = canvas;
    containerRef.current = container;

    const previousPosition = container.style.position;
    const shouldRestorePosition = getComputedStyle(container).position === "static";
    if (shouldRestorePosition) {
      container.style.position = "relative";
    }
    container.appendChild(canvas);
    placeCanvasOnViewport();

    const placeCanvasOnPage = (target: DrawingPageTarget) => {
      const rect = drawingPageCanvasRect(container, target.element);
      target.width = rect.width;
      target.height = rect.height;
      canvasSizeRef.current = setDrawingCanvasRect(canvas, rect.left, rect.top, rect.width, rect.height);
      redraw();
    };

    const startStroke = (event: PointerEvent) => {
      if (event.pointerType === "mouse" && event.button !== 0) return;
      const target = pageTargetAtPoint(container, event.clientX, event.clientY);
      if (!target) return;
      if (pageRef.current && pageRef.current.pageNumber !== target.pageNumber && drawingHasInk(strokesRef.current)) {
        saveDraft();
      }

      pageRef.current = target;
      placeCanvasOnPage(target);
      const point = pointForPageEvent(target.element, event);
      const style = strokeStyleRef.current;
      currentStrokeRef.current = {
        points: [point],
        color: style.strokeColor,
        width: style.strokeWidth,
      };
      drawingRef.current = true;
      clearSaveTimer();
      canvas.setPointerCapture(event.pointerId);
      redraw();
      event.preventDefault();
      event.stopPropagation();
    };

    const moveStroke = (event: PointerEvent) => {
      const target = pageRef.current;
      const stroke = currentStrokeRef.current;
      if (!drawingRef.current || !target || !stroke) return;
      const point = pointForPageEvent(target.element, event);
      const last = stroke.points[stroke.points.length - 1];
      if (!last || Math.abs(last.x - point.x) > 0.25 || Math.abs(last.y - point.y) > 0.25) {
        stroke.points.push(point);
        redraw();
      }
      event.preventDefault();
      event.stopPropagation();
    };

    const finishStroke = (event: PointerEvent) => {
      if (!drawingRef.current) return;
      const stroke = currentStrokeRef.current;
      if (stroke?.points.length) {
        strokesRef.current = [...strokesRef.current, stroke];
      }
      currentStrokeRef.current = null;
      drawingRef.current = false;
      try {
        canvas.releasePointerCapture(event.pointerId);
      } catch {
        // Pointer capture can already be released by the browser.
      }
      redraw();
      scheduleSave();
      event.preventDefault();
      event.stopPropagation();
    };

    const saveForScroll = () => {
      if (drawingHasInk(strokesRef.current, currentStrokeRef.current)) {
        saveDraft();
        return;
      }
      placeCanvasOnViewport();
    };

    canvas.addEventListener("pointerdown", startStroke);
    canvas.addEventListener("pointermove", moveStroke);
    canvas.addEventListener("pointerup", finishStroke);
    canvas.addEventListener("pointercancel", finishStroke);
    container.addEventListener("scroll", saveForScroll, { passive: true });
    container.addEventListener("wheel", saveForScroll, { passive: true });
    window.addEventListener("resize", saveForScroll);

    return () => {
      saveDraft();
      canvas.removeEventListener("pointerdown", startStroke);
      canvas.removeEventListener("pointermove", moveStroke);
      canvas.removeEventListener("pointerup", finishStroke);
      canvas.removeEventListener("pointercancel", finishStroke);
      container.removeEventListener("scroll", saveForScroll);
      container.removeEventListener("wheel", saveForScroll);
      window.removeEventListener("resize", saveForScroll);
      canvas.remove();
      canvasRef.current = null;
      containerRef.current = null;
      if (shouldRestorePosition) {
        container.style.position = previousPosition;
      }
    };
  }, [active, clearSaveTimer, placeCanvasOnViewport, redraw, saveDraft, scheduleSave, utils]);

  return null;
}

function ReaderFreetextHighlight({
  highlight,
  isScrolledTo,
  selected,
  color,
  backgroundColor,
  fontSize,
  onChange,
  onTextChange,
  onSelect,
  onEditStart,
  onEditEnd,
  autoFocusEditor,
  onAutoFocusHandled,
}: {
  highlight: ViewportHighlight<ReaderHighlight>;
  isScrolledTo: boolean;
  selected: boolean;
  color: string;
  backgroundColor: string;
  fontSize: string;
  onChange: (rect: LTWHP) => void;
  onTextChange: (text: string) => void;
  onSelect: () => void;
  onEditStart: () => void;
  onEditEnd?: () => void;
  autoFocusEditor: boolean;
  onAutoFocusHandled: () => void;
}) {
  const rect = highlight.position.boundingRect;
  const minWidth = freetextMinWidth(fontSize);
  const width = Math.max(minWidth, rect.width || FREETEXT_DEFAULT_WIDTH);
  const [isEditing, setIsEditing] = useState(false);
  const [text, setText] = useState(highlight.content?.text || "");
  const [draftWidth, setDraftWidth] = useState(width);
  const [visualHeight, setVisualHeight] = useState(FREETEXT_DEFAULT_HEIGHT);
  const wrapperRef = useRef<HTMLDivElement | null>(null);
  const contentRef = useRef<HTMLDivElement | null>(null);
  const textareaRef = useRef<HTMLTextAreaElement | null>(null);
  const isEditingRef = useRef(isEditing);
  const selectAllOnEditRef = useRef(false);
  const committedTextRef = useRef(highlight.content?.text || "");
  const editDirtyRef = useRef(false);
  const dragStartPosRef = useRef<{ x: number; y: number } | null>(null);
  const height = Math.max(FREETEXT_DEFAULT_HEIGHT, visualHeight);
  const className = [
    "FreetextHighlight",
    isScrolledTo ? "FreetextHighlight--scrolledTo" : "",
    selected ? "FreetextHighlight--selected" : "",
    isEditing ? "FreetextHighlight--editing" : "",
  ]
    .filter(Boolean)
    .join(" ");

  useEffect(() => {
    isEditingRef.current = isEditing;
  }, [isEditing]);

  useEffect(() => {
    setDraftWidth(width);
  }, [width]);

  useEffect(() => {
    if (isEditingRef.current) return;
    const nextText = highlight.content?.text || "";
    committedTextRef.current = nextText;
    setText(nextText);
  }, [highlight.content?.text]);

  const placeTextareaCursorEnd = useCallback(() => {
    const textarea = textareaRef.current;
    if (!textarea) return;
    textarea.focus({ preventScroll: true });
    textarea.setSelectionRange(textarea.value.length, textarea.value.length);
  }, []);

  const focusEditorAtEndSoon = useCallback(() => {
    window.requestAnimationFrame(() => {
      placeTextareaCursorEnd();
      window.requestAnimationFrame(placeTextareaCursorEnd);
    });
  }, [placeTextareaCursorEnd]);

  const resizeTextarea = useCallback(() => {
    const textarea = textareaRef.current;
    if (!textarea) return;
    textarea.style.height = "0px";
    textarea.style.height = `${Math.max(FREETEXT_TEXT_MIN_HEIGHT, textarea.scrollHeight)}px`;
  }, []);

  const measuredHeight = useCallback(() => {
    const contentHeight = contentRef.current?.offsetHeight || FREETEXT_DEFAULT_HEIGHT;
    return Math.max(FREETEXT_DEFAULT_HEIGHT, Math.ceil(contentHeight));
  }, []);

  const syncVisualHeight = useCallback(() => {
    setVisualHeight(measuredHeight());
  }, [measuredHeight]);

  useLayoutEffect(() => {
    syncVisualHeight();
  }, [fontSize, highlight.content?.text, isEditing, syncVisualHeight, text, draftWidth]);

  useEffect(() => {
    if (!isEditing) return;
    const frame = window.requestAnimationFrame(() => {
      resizeTextarea();
      if (!selectAllOnEditRef.current) return;
      placeTextareaCursorEnd();
      window.requestAnimationFrame(placeTextareaCursorEnd);
    });
    return () => window.cancelAnimationFrame(frame);
  }, [isEditing, placeTextareaCursorEnd, resizeTextarea]);

  useEffect(() => {
    if (!isEditing) return;
    resizeTextarea();
    syncVisualHeight();
  }, [fontSize, isEditing, resizeTextarea, syncVisualHeight, text, draftWidth]);

  const rectWith = useCallback(
    (patch: Partial<LTWHP> = {}): LTWHP => ({
      pageNumber: patch.pageNumber ?? rect.pageNumber,
      left: patch.left ?? rect.left,
      top: patch.top ?? rect.top,
      width: patch.width ?? draftWidth,
      height: patch.height ?? measuredHeight(),
    }),
    [draftWidth, measuredHeight, rect.left, rect.pageNumber, rect.top],
  );

  const commitAutoHeight = useCallback(() => {
    const nextHeight = measuredHeight();
    if (Math.abs(nextHeight - height) <= FREETEXT_HEIGHT_EPSILON) return;
    onChange(rectWith({ height: nextHeight }));
  }, [height, measuredHeight, onChange, rectWith]);

  useEffect(() => {
    if (isEditing) return;
    const frame = window.requestAnimationFrame(commitAutoHeight);
    return () => window.cancelAnimationFrame(frame);
  }, [commitAutoHeight, fontSize, highlight.content?.text, isEditing, width]);

  const startEditing = useCallback(() => {
    if (isEditingRef.current) return;
    committedTextRef.current = highlight.content?.text || text;
    editDirtyRef.current = false;
    selectAllOnEditRef.current = false;
    setText(committedTextRef.current);
    setIsEditing(true);
    onEditStart();
    focusEditorAtEndSoon();
  }, [focusEditorAtEndSoon, highlight.content?.text, onEditStart, text]);

  useEffect(() => {
    if (!autoFocusEditor) return;
    if (!isEditingRef.current) startEditing();
    const frame = window.requestAnimationFrame(() => {
      placeTextareaCursorEnd();
      onAutoFocusHandled();
    });
    return () => window.cancelAnimationFrame(frame);
  }, [autoFocusEditor, onAutoFocusHandled, placeTextareaCursorEnd, startEditing]);

  const finishEditing = useCallback(() => {
    if (!isEditingRef.current) return;
    const nextText = editDirtyRef.current ? text : committedTextRef.current;
    selectAllOnEditRef.current = false;
    editDirtyRef.current = false;
    isEditingRef.current = false;
    setIsEditing(false);
    setText(nextText);
    if (nextText !== committedTextRef.current) {
      onTextChange(nextText);
    }
    onEditEnd?.();
    window.setTimeout(commitAutoHeight, 0);
  }, [commitAutoHeight, onEditEnd, onTextChange, text]);

  useEffect(() => {
    if (!isEditing) return;
    const handleOutsidePointerDown = (event: PointerEvent) => {
      const target = event.target;
      if (target instanceof Node && wrapperRef.current?.contains(target)) return;
      finishEditing();
    };
    document.addEventListener("pointerdown", handleOutsidePointerDown, true);
    return () => {
      document.removeEventListener("pointerdown", handleOutsidePointerDown, true);
    };
  }, [finishEditing, isEditing]);

  const handleDragStop = useCallback(
    (_event: RndDragEvent, data: DraggableData) => {
      setInteracting(false);
      unlockAnnotationTextSelection();
      const startPos = dragStartPosRef.current;
      dragStartPosRef.current = null;
      if (
        startPos &&
        Math.abs(data.x - startPos.x) <= FREETEXT_DRAG_EPSILON &&
        Math.abs(data.y - startPos.y) <= FREETEXT_DRAG_EPSILON
      ) {
        return;
      }
      onChange(rectWith({ left: data.x, top: data.y }));
    },
    [onChange, rectWith],
  );

  const handleDragStart = useCallback(
    (event: RndDragEvent, data: DraggableData) => {
      if ("detail" in event && typeof event.detail === "number" && event.detail > 1) {
        return false;
      }
      event.preventDefault();
      dragStartPosRef.current = { x: data.x, y: data.y };
      setInteracting(true);
      lockAnnotationTextSelection();
      onSelect();
      onEditStart();
    },
    [onEditStart, onSelect],
  );

  useEffect(() => unlockAnnotationTextSelection, []);

  const handleResizeStop: RndResizeCallback = useCallback(
    (_event, _direction, ref, _delta, position) => {
      setInteracting(false);
      const nextWidth = Math.max(minWidth, ref.offsetWidth);
      setDraftWidth(nextWidth);
      const nextHeight = measuredHeight();
      setVisualHeight(nextHeight);
      onChange(
        rectWith({
          left: position.x,
          top: position.y,
          width: nextWidth,
          height: nextHeight,
        }),
      );
    },
    [measuredHeight, minWidth, onChange, rectWith],
  );

  const handleKeyDown = (event: ReactKeyboardEvent<HTMLTextAreaElement>) => {
    if (event.key === "Escape") {
      event.preventDefault();
      setText(committedTextRef.current);
      editDirtyRef.current = false;
      isEditingRef.current = false;
      setIsEditing(false);
      selectAllOnEditRef.current = false;
      onEditEnd?.();
      return;
    }
    if (event.key === "Enter" && !event.shiftKey) {
      event.preventDefault();
      finishEditing();
    }
    if (selectAllOnEditRef.current && (event.key === "Backspace" || event.key === "Delete")) {
      event.preventDefault();
      selectAllOnEditRef.current = false;
      editDirtyRef.current = true;
      setText("");
      return;
    }
    if (
      selectAllOnEditRef.current &&
      event.key.length === 1 &&
      !event.altKey &&
      !event.ctrlKey &&
      !event.metaKey
    ) {
      event.preventDefault();
      selectAllOnEditRef.current = false;
      editDirtyRef.current = true;
      setText(event.key);
    }
  };

  const displayText = normalizeFreetextText(text) ? text : "";

  // 受控模式：缩放时直接更新 position/size，避免 key 变化导致的卸载/重挂载闪烁
  const [interacting, setInteracting] = useState(false);
  const [dragPos, setDragPos] = useState({ x: rect.left, y: rect.top });

  const pos = interacting ? dragPos : { x: rect.left, y: rect.top };

  return (
    <div
      ref={wrapperRef}
      className={className}
      data-reader-freetext-id={highlight.annotation.id}
      data-reader-freetext-editing={isEditing ? "true" : "false"}
    >
      <Rnd
        className="FreetextHighlight__rnd"
        style={{
          backgroundColor,
          color,
          fontSize,
          border: 0,
          borderRadius: 0,
          boxShadow: "none",
          overflow: "visible",
        }}
        position={pos}
        size={{ width: draftWidth, height }}
        minWidth={minWidth}
        minHeight={FREETEXT_DEFAULT_HEIGHT}
        disableDragging={!selected || isEditing}
        enableResizing={isEditing ? FREETEXT_RESIZE_ENABLE : false}
        resizeHandleClasses={FREETEXT_RESIZE_HANDLE_CLASSES}
        resizeHandleStyles={FREETEXT_RESIZE_HANDLE_STYLES}
        onDragStart={handleDragStart}
        onDrag={(_event, data) => {
          setDragPos({ x: data.x, y: data.y });
        }}
        onDragStop={handleDragStop}
        onResizeStart={() => {
          setInteracting(true);
          onEditStart();
        }}
        onResize={(_event, _direction, ref) => {
          setDraftWidth(ref.offsetWidth);
          window.requestAnimationFrame(syncVisualHeight);
        }}
        onResizeStop={handleResizeStop}
        cancel={selected && !isEditing ? ".FreetextHighlight__input" : ".FreetextHighlight__text, .FreetextHighlight__input"}
      >
        <div className="FreetextHighlight__container" data-reader-freetext-container="true">
          <div ref={contentRef} className="FreetextHighlight__content">
            {isEditing ? (
              <textarea
                ref={textareaRef}
                className="FreetextHighlight__input"
                value={text}
                onChange={(event) => {
                  selectAllOnEditRef.current = false;
                  editDirtyRef.current = true;
                  setText(event.target.value);
                }}
                onBlur={finishEditing}
                onFocus={() => {
                  if (!selectAllOnEditRef.current) return;
                  window.requestAnimationFrame(placeTextareaCursorEnd);
                }}
                onMouseDown={(event) => {
                  if (!selectAllOnEditRef.current) return;
                  event.preventDefault();
                  placeTextareaCursorEnd();
                }}
                onKeyDown={handleKeyDown}
                onPaste={(event) => {
                  if (!selectAllOnEditRef.current) return;
                  event.preventDefault();
                  selectAllOnEditRef.current = false;
                  editDirtyRef.current = true;
                  setText(event.clipboardData.getData("text/plain"));
                }}
                onClick={(event) => {
                  event.stopPropagation();
                  if (!selectAllOnEditRef.current) return;
                  event.preventDefault();
                  placeTextareaCursorEnd();
                }}
              />
            ) : (
              <div
                className="FreetextHighlight__text"
                onMouseDown={(event) => {
                  event.preventDefault();
                  onSelect();
                  if (!selected) {
                    event.stopPropagation();
                  }
                }}
                onClick={(event) => {
                  event.stopPropagation();
                  onSelect();
                }}
                onDoubleClick={(event) => {
                  event.stopPropagation();
                  onSelect();
                  startEditing();
                }}
              >
                {displayText}
              </div>
            )}
          </div>
        </div>
      </Rnd>
    </div>
  );
}

function ReaderDrawingHighlight({
  highlight,
  isScrolledTo,
}: {
  highlight: ViewportHighlight<ReaderHighlight>;
  isScrolledTo: boolean;
}) {
  const rect = highlight.position.boundingRect;
  const rawStrokes = highlight.content?.strokes ?? [];
  const strokes = drawingRenderStrokes(highlight.annotation, rawStrokes);
  const imageUrl = highlight.content?.image;
  const viewBox = drawingViewBox(highlight.annotation, strokes);
  const className = ["DrawingHighlight", isScrolledTo ? "DrawingHighlight--scrolledTo" : ""]
    .filter(Boolean)
    .join(" ");
  const width = Math.max(1, rect.width || 150);
  const height = Math.max(1, rect.height || 100);

  return (
    <div
      className={className}
      data-reader-drawing-id={highlight.annotation.id}
      style={{
        position: "absolute",
        left: rect.left,
        top: rect.top,
        width,
        height,
      }}
    >
      <div className="DrawingHighlight__container">
        <div className="DrawingHighlight__content">
          {strokes.length > 0 ? (
            <svg
              className="DrawingHighlight__svg"
              viewBox={`0 0 ${viewBox.width} ${viewBox.height}`}
              preserveAspectRatio="none"
              aria-hidden="true"
            >
              {strokes.map((stroke, index) =>
                stroke.points.length === 1 ? (
                  <circle
                    key={index}
                    cx={stroke.points[0].x}
                    cy={stroke.points[0].y}
                    r={Math.max(1, stroke.width / 2)}
                    fill={stroke.color}
                  />
                ) : (
                  <path
                    key={index}
                    d={drawingPath(stroke.points)}
                    fill="none"
                    stroke={stroke.color}
                    strokeWidth={stroke.width}
                    strokeLinecap="round"
                    strokeLinejoin="round"
                  />
                ),
              )}
            </svg>
          ) : imageUrl ? (
            <img src={imageUrl} alt="Drawing" className="DrawingHighlight__image" draggable={false} />
          ) : null}
        </div>
      </div>
    </div>
  );
}

function HighlightContainer({
  onDelete,
  onUpdatePosition,
  onUpdateText,
  locatedAnnotationId,
  selectedAnnotationId,
  pendingFreetextFocusId,
  onSelectAnnotation,
  onFreetextFocusHandled,
}: {
  onDelete: (annotation: PaperAnnotation) => void;
  onUpdatePosition: (
    annotation: PaperAnnotation,
    position: ScaledPosition,
    snapshot?: string,
  ) => void;
  onUpdateText: (annotation: PaperAnnotation, text: string) => void;
  locatedAnnotationId: number | null;
  selectedAnnotationId: number | null;
  pendingFreetextFocusId: number | null;
  onSelectAnnotation: (annotation: PaperAnnotation) => void;
  onFreetextFocusHandled: (annotationID: number) => void;
}) {
  const { highlight, isScrolledTo } = useHighlightContainerContext<ReaderHighlight>();
  const utils = usePdfHighlighterContext();
  const rootRef = useRef<HTMLDivElement>(null);
  const annotation = highlight.annotation;
  const selected = selectedAnnotationId === annotation.id;
  const located = isScrolledTo || locatedAnnotationId === annotation.id;
  const active = located || selected;

  const selectAnnotationFromPdf = (event: ReactMouseEvent<HTMLDivElement>) => {
    if (shouldIgnoreAnnotationClick(event.target)) return;
    utils.setTip(null);
    onSelectAnnotation(annotation);
  };

  const prepareMovableAnnotationDrag = (event: ReactMouseEvent<HTMLDivElement>) => {
    if (highlight.type !== "freetext") return;
    if (shouldIgnoreAnnotationClick(event.target)) return;
    event.preventDefault();
    lockAnnotationTextSelection();
    window.addEventListener("mouseup", unlockAnnotationTextSelection, { once: true });
  };

  const handleChange = (rect: LTWHP) => {
    const position = scaledFromViewportRect(rect, utils);
    if (!position) return;
    onUpdatePosition(annotation, position);
  };

  let content: ReactNode;
  if (highlight.type === "freetext") {
    const style = freetextStyle(annotation);
    const scale = annotationViewportScale(annotation, highlight.position.boundingRect, style.basePageWidth);
    content = (
      <ReaderFreetextHighlight
        highlight={highlight}
        isScrolledTo={active}
        selected={selected}
        color={style.color}
        backgroundColor={style.backgroundColor}
        fontSize={`${style.fontSize * scale}px`}
        onChange={handleChange}
        onTextChange={(text) => onUpdateText(annotation, text)}
        onSelect={() => onSelectAnnotation(annotation)}
        onEditStart={() => utils.setTip(null)}
        autoFocusEditor={pendingFreetextFocusId === annotation.id}
        onAutoFocusHandled={() => onFreetextFocusHandled(annotation.id)}
      />
    );
  } else if (highlight.type === "drawing") {
    content = (
      <ReaderDrawingHighlight
        highlight={highlight}
        isScrolledTo={active}
      />
    );
  } else {
    content = (
      <TextHighlight
        highlight={highlight}
        isScrolledTo={located}
        highlightColor={colorValue(annotation.color)}
        copyText={annotation.text}
        onDelete={() => onDelete(annotation)}
      />
    );
  }

  return (
    <div
      ref={rootRef}
      className="contents"
      data-reader-annotation-id={annotation.id}
      data-reader-annotation-selected={selected ? "true" : undefined}
      onMouseDownCapture={prepareMovableAnnotationDrag}
      onClickCapture={selectAnnotationFromPdf}
    >
      {content}
    </div>
  );
}

function DocumentReadyEffect({
  pdfDocument,
  onPageCount,
  onDocumentReady,
}: {
  pdfDocument: PDFDocumentProxy;
  onPageCount: (pages: number) => void;
  onDocumentReady: (pdfDocument: PDFDocumentProxy) => void;
}) {
  useEffect(() => {
    onPageCount(pdfDocument.numPages);
    onDocumentReady(pdfDocument);
  }, [onDocumentReady, onPageCount, pdfDocument]);
  return null;
}

export function ReaderPdf({
  pdfUrl,
  highlights,
  initialPage,
  scaleValue,
  activeTool,
  onPageChange,
  onPageCount,
  onTranslateSelection,
  onSaveHighlight,
  onAnnotateSelection,
  onAskSelection,
  onCreateFreetext,
  drawingColor,
  drawingSize,
  onCreateDrawing,
  onDrawingDraftSaveReady,
  onUpdateAnnotationPosition,
  onUpdateAnnotationText,
  onDeleteAnnotation,
  locatedAnnotationId,
  selectedAnnotationId,
  pendingFreetextFocusId,
  onSelectAnnotation,
  onFreetextFocusHandled,
  onDocumentReady,
  onUtilsReady,
}: {
  pdfUrl: string;
  highlights: ReaderHighlight[];
  initialPage: number;
  scaleValue: PdfScaleValue;
  activeTool: ReaderTool;
  onPageChange: (page: number) => void;
  onPageCount: (pages: number) => void;
  onTranslateSelection: (selection: PdfSelection) => void;
  onSaveHighlight: (selection: PdfSelection) => void;
  onAnnotateSelection: (selection: PdfSelection) => void;
  onAskSelection: (selection: PdfSelection) => void;
  onCreateFreetext: (position: ScaledPosition) => void;
  drawingColor: string;
  drawingSize: number;
  onCreateDrawing: (
    image: string,
    position: ScaledPosition,
    strokes: DrawingStroke[],
    meta: DrawingSaveMeta,
  ) => Promise<boolean> | boolean;
  onDrawingDraftSaveReady: (save: () => void) => void;
  onUpdateAnnotationPosition: (
    annotation: PaperAnnotation,
    position: ScaledPosition,
    snapshot?: string,
  ) => void;
  onUpdateAnnotationText: (annotation: PaperAnnotation, text: string) => void;
  onDeleteAnnotation: (annotation: PaperAnnotation) => void;
  locatedAnnotationId: number | null;
  selectedAnnotationId: number | null;
  pendingFreetextFocusId: number | null;
  onSelectAnnotation: (annotation: PaperAnnotation) => void;
  onFreetextFocusHandled: (annotationID: number) => void;
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

  useEffect(() => {
    if (!pagesReady) return;
    const utils = utilsRef.current;
    const viewer = utils?.getViewer();
    if (!viewer) return;
    viewer.currentScaleValue = scaleValue.toString();
  }, [pagesReady, scaleValue, utilsVersion]);

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
        <>
          <DocumentReadyEffect
            pdfDocument={pdfDocument}
            onPageCount={onPageCount}
            onDocumentReady={onDocumentReady}
          />
          <PdfHighlighter
            pdfDocument={pdfDocument}
            highlights={highlights}
            pdfScaleValue={pagesReady ? scaleValue : "auto"}
            enableAreaSelection={() => false}
            enableFreetextCreation={() => activeTool === "freetext"}
            onFreetextClick={onCreateFreetext}
            textSelectionColor="rgba(14, 165, 233, 0.22)"
            selectionTip={
              activeTool === "select" ? (
                <SelectionToolbar
                  onTranslate={onTranslateSelection}
                  onHighlight={onSaveHighlight}
                  onAnnotate={onAnnotateSelection}
                  onAsk={onAskSelection}
                />
              ) : null
            }
            utilsRef={setUtils}
            theme={{
              mode: "light",
              containerBackgroundColor: "oklch(0.96 0.003 230)",
              scrollbarThumbColor: "oklch(0.66 0.004 230 / 0.45)",
              scrollbarTrackColor: "oklch(0.94 0.003 230 / 0.72)",
            }}
            style={{ height: "100%" }}
          >
            <HighlightContainer
              onDelete={onDeleteAnnotation}
              onUpdatePosition={onUpdateAnnotationPosition}
              onUpdateText={onUpdateAnnotationText}
              locatedAnnotationId={locatedAnnotationId}
              selectedAnnotationId={selectedAnnotationId}
              pendingFreetextFocusId={pendingFreetextFocusId}
              onSelectAnnotation={onSelectAnnotation}
              onFreetextFocusHandled={onFreetextFocusHandled}
            />
          </PdfHighlighter>
          <ReaderDrawingLayer
            active={activeTool === "drawing" && pagesReady}
            utils={utilsRef.current}
            strokeColor={drawingColor}
            strokeWidth={drawingSize}
            onCreateDrawing={onCreateDrawing}
            onSaveReady={onDrawingDraftSaveReady}
          />
        </>
      )}
    </ReaderPdfLoader>
  );
}

export { annotationToHighlight };
