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
  type RndDragCallback,
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
  colorSolid,
  colorValue,
  FREETEXT_DEFAULT_HEIGHT,
  FREETEXT_DEFAULT_WIDTH,
  freetextStyle,
  normalizeFreetextText,
  type AnnotationColor,
  type ReaderHighlight,
  type ReaderTool,
} from "@/app/reader/lib/annotations";

const PDF_WORKER = "/pdfjs/pdf.worker.min.mjs";
const LOCATE_TOP_GAP = 32;
const SNAPSHOT_PADDING = 24;
const SNAPSHOT_MAX_SIDE = 560;
const ANNOTATION_DRAGGING_CLASS = "reader-annotation-dragging";
const FREETEXT_MIN_WIDTH = 24;
const FREETEXT_TEXT_MIN_HEIGHT = 18;
const FREETEXT_HEIGHT_EPSILON = 1;
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

function clamp(value: number, min: number, max: number) {
  return Math.min(max, Math.max(min, value));
}

function freetextMinWidth(fontSize: string) {
  const size = Number.parseFloat(fontSize);
  const textColumn = Number.isFinite(size) && size > 0 ? size : FREETEXT_TEXT_MIN_HEIGHT;
  return Math.max(FREETEXT_MIN_WIDTH, Math.ceil(textColumn * 1.25 + 18));
}

function annotationViewportScale(annotation: PaperAnnotation, rect: LTWHP) {
  const storedPageWidth = annotation.bounding_rect?.width;
  if (typeof storedPageWidth !== "number" || !Number.isFinite(storedPageWidth) || storedPageWidth <= 0) {
    return 1;
  }
  const scaledRectWidth = annotation.bounding_rect.x2 - annotation.bounding_rect.x1;
  if (!Number.isFinite(scaledRectWidth) || scaledRectWidth <= 0) return 1;
  const currentPageWidth = rect.width * (storedPageWidth / scaledRectWidth);
  if (!Number.isFinite(currentPageWidth) || currentPageWidth <= 0) return 1;
  return currentPageWidth / storedPageWidth;
}

function drawingPath(points: DrawingStroke["points"]) {
  if (points.length === 0) return "";
  const [first, ...rest] = points;
  return [`M ${first.x} ${first.y}`, ...rest.map((point) => `L ${point.x} ${point.y}`)].join(" ");
}

function drawingViewBox(annotation: PaperAnnotation, strokes: DrawingStroke[]) {
  const rect = annotation.bounding_rect;
  const rectWidth = Math.max(1, rect.x2 - rect.x1);
  const rectHeight = Math.max(1, rect.y2 - rect.y1);
  let maxX = rectWidth;
  let maxY = rectHeight;

  for (const stroke of strokes) {
    const padding = Math.max(1, stroke.width || 1) * 2;
    for (const point of stroke.points) {
      maxX = Math.max(maxX, point.x + padding);
      maxY = Math.max(maxY, point.y + padding);
    }
  }

  return { width: Math.ceil(maxX), height: Math.ceil(maxY) };
}

function cropPageSnapshot(
  utils: PdfHighlighterUtils | null,
  position: ScaledPosition,
): string | undefined {
  try {
    const viewer = utils?.getViewer();
    if (!viewer) return undefined;
    const viewportPosition = scaledPositionToViewport(position, viewer);
    const rect = viewportPosition.boundingRect;
    const pageView = viewer.getPageView(rect.pageNumber - 1);
    const pageElement = pageView?.div as HTMLElement | undefined;
    const canvas = pageElement?.querySelector<HTMLCanvasElement>("canvas");
    if (!pageElement || !canvas) return undefined;

    const pageRect = pageElement.getBoundingClientRect();
    const canvasRect = canvas.getBoundingClientRect();
    if (canvasRect.width <= 0 || canvasRect.height <= 0) return undefined;

    const canvasOffsetX = canvasRect.left - pageRect.left;
    const canvasOffsetY = canvasRect.top - pageRect.top;
    const scaleX = canvas.width / canvasRect.width;
    const scaleY = canvas.height / canvasRect.height;
    const sourceLeftCss = rect.left - canvasOffsetX - SNAPSHOT_PADDING;
    const sourceTopCss = rect.top - canvasOffsetY - SNAPSHOT_PADDING;
    const sourceWidthCss = rect.width + SNAPSHOT_PADDING * 2;
    const sourceHeightCss = rect.height + SNAPSHOT_PADDING * 2;

    const sx = clamp(Math.round(sourceLeftCss * scaleX), 0, canvas.width);
    const sy = clamp(Math.round(sourceTopCss * scaleY), 0, canvas.height);
    const sw = clamp(Math.round(sourceWidthCss * scaleX), 1, canvas.width - sx);
    const sh = clamp(Math.round(sourceHeightCss * scaleY), 1, canvas.height - sy);
    if (sw <= 0 || sh <= 0) return undefined;

    const outputScale = Math.min(1, SNAPSHOT_MAX_SIDE / Math.max(sw, sh));
    const output = document.createElement("canvas");
    output.width = Math.max(1, Math.round(sw * outputScale));
    output.height = Math.max(1, Math.round(sh * outputScale));
    const ctx = output.getContext("2d");
    if (!ctx) return undefined;
    ctx.drawImage(canvas, sx, sy, sw, sh, 0, 0, output.width, output.height);
    return output.toDataURL("image/png");
  } catch {
    return undefined;
  }
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
      unlockAnnotationTextSelection();
      onChange(rectWith({ left: data.x, top: data.y }));
    },
    [onChange, rectWith],
  );

  const handleDragStart = useCallback(
    (event: RndDragEvent) => {
      event.preventDefault();
      lockAnnotationTextSelection();
      onSelect();
      onEditStart();
    },
    [onEditStart, onSelect],
  );

  useEffect(() => unlockAnnotationTextSelection, []);

  const handleResizeStop: RndResizeCallback = useCallback(
    (_event, _direction, ref, _delta, position) => {
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
  const rndKey = `${rect.left}:${rect.top}`;

  return (
    <div
      ref={wrapperRef}
      className={className}
      data-reader-freetext-id={highlight.annotation.id}
      data-reader-freetext-editing={isEditing ? "true" : "false"}
    >
      <Rnd
        key={rndKey}
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
        default={{
          x: rect.left,
          y: rect.top,
          width: draftWidth,
          height,
        }}
        size={{ width: draftWidth, height }}
        minWidth={minWidth}
        minHeight={FREETEXT_DEFAULT_HEIGHT}
        disableDragging={!selected || isEditing}
        enableResizing={isEditing ? FREETEXT_RESIZE_ENABLE : false}
        resizeHandleClasses={FREETEXT_RESIZE_HANDLE_CLASSES}
        resizeHandleStyles={FREETEXT_RESIZE_HANDLE_STYLES}
        onDragStart={handleDragStart}
        onDragStop={handleDragStop}
        onResizeStart={() => onEditStart()}
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
  onChange,
  onEditStart,
  onEditEnd,
}: {
  highlight: ViewportHighlight<ReaderHighlight>;
  isScrolledTo: boolean;
  onChange: (rect: LTWHP) => void;
  onEditStart: () => void;
  onEditEnd: () => void;
}) {
  const rect = highlight.position.boundingRect;
  const strokes = highlight.content?.strokes ?? [];
  const imageUrl = highlight.content?.image;
  const viewBox = drawingViewBox(highlight.annotation, strokes);
  const className = ["DrawingHighlight", isScrolledTo ? "DrawingHighlight--scrolledTo" : ""]
    .filter(Boolean)
    .join(" ");

  // 使用受控模式：缩放时直接更新 position/size，避免 key 变化导致的卸载/重挂载闪烁
  const [interacting, setInteracting] = useState(false);
  const [dragPos, setDragPos] = useState({ x: rect.left, y: rect.top });
  const [dragSize, setDragSize] = useState({ width: rect.width || 150, height: rect.height || 100 });

  // 非交互时跟随 viewport rect 更新（缩放/翻页等场景）
  const pos = interacting ? dragPos : { x: rect.left, y: rect.top };
  const size = interacting ? dragSize : { width: rect.width || 150, height: rect.height || 100 };

  const handleDragStart = useCallback(() => {
    setInteracting(true);
    onEditStart();
  }, [onEditStart]);

  const handleDrag: RndDragCallback = useCallback((_event, data) => {
    setDragPos({ x: data.x, y: data.y });
  }, []);

  const handleDragStop = useCallback(
    (_event: RndDragEvent, data: DraggableData) => {
      setInteracting(false);
      onChange({ ...rect, left: data.x, top: data.y });
      onEditEnd();
    },
    [onChange, onEditEnd, rect],
  );

  const handleResizeStart = useCallback(() => {
    setInteracting(true);
    onEditStart();
  }, [onEditStart]);

  const handleResizeStop: RndResizeCallback = useCallback(
    (_event, _direction, ref, _delta, position) => {
      setInteracting(false);
      onChange({
        pageNumber: rect.pageNumber,
        left: position.x,
        top: position.y,
        width: ref.offsetWidth,
        height: ref.offsetHeight,
      });
      onEditEnd();
    },
    [onChange, onEditEnd, rect.pageNumber],
  );

  return (
    <div className={className} data-reader-drawing-id={highlight.annotation.id}>
      <Rnd
        className="DrawingHighlight__rnd"
        position={pos}
        size={size}
        minWidth={30}
        minHeight={30}
        onDragStart={handleDragStart}
        onDrag={handleDrag}
        onDragStop={handleDragStop}
        onResizeStart={handleResizeStart}
        onResize={(_e, _dir, ref, _delta, pos) => {
          setDragPos({ x: pos.x, y: pos.y });
          setDragSize({ width: ref.offsetWidth, height: ref.offsetHeight });
        }}
        onResizeStop={handleResizeStop}
      >
        <div className="DrawingHighlight__container">
          <div className="DrawingHighlight__content">
            {strokes.length > 0 ? (
              <svg
                className="DrawingHighlight__svg"
                viewBox={`0 0 ${viewBox.width} ${viewBox.height}`}
                preserveAspectRatio="xMidYMid meet"
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
      </Rnd>
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
    if (highlight.type !== "freetext" && highlight.type !== "drawing") return;
    if (shouldIgnoreAnnotationClick(event.target)) return;
    event.preventDefault();
    lockAnnotationTextSelection();
    window.addEventListener("mouseup", unlockAnnotationTextSelection, { once: true });
  };

  const handleChange = (rect: LTWHP) => {
    const position = scaledFromViewportRect(rect, utils);
    if (!position) return;
    onUpdatePosition(
      annotation,
      position,
      highlight.type === "drawing" ? cropPageSnapshot(utils, position) : undefined,
    );
  };

  let content: ReactNode;
  if (highlight.type === "freetext") {
    const style = freetextStyle(annotation);
    const scale = annotationViewportScale(annotation, highlight.position.boundingRect);
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
        onEditStart={lockAnnotationTextSelection}
        onEditEnd={unlockAnnotationTextSelection}
        onChange={handleChange}
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
  color,
  drawingSize,
  onPageChange,
  onPageCount,
  onTranslateSelection,
  onSaveHighlight,
  onAnnotateSelection,
  onAskSelection,
  onCreateFreetext,
  onCreateDrawing,
  onDrawingCancel,
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
  color: AnnotationColor;
  drawingSize: number;
  onPageChange: (page: number) => void;
  onPageCount: (pages: number) => void;
  onTranslateSelection: (selection: PdfSelection) => void;
  onSaveHighlight: (selection: PdfSelection) => void;
  onAnnotateSelection: (selection: PdfSelection) => void;
  onAskSelection: (selection: PdfSelection) => void;
  onCreateFreetext: (position: ScaledPosition) => void;
  onCreateDrawing: (
    dataUrl: string,
    position: ScaledPosition,
    strokes: DrawingStroke[],
    snapshot?: string,
  ) => void;
  onDrawingCancel: () => void;
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

  useEffect(() => {
    if (activeTool !== "drawing") return;
    const viewer = utilsRef.current?.getViewer();
    const scrollElement = viewer?.container;
    if (!scrollElement) return;

    let canvas: HTMLCanvasElement | null = null;
    const handleWheel = (event: WheelEvent) => {
      if (event.ctrlKey || event.metaKey) return;
      if (event.deltaX === 0 && event.deltaY === 0) return;

      const deltaScale =
        event.deltaMode === WheelEvent.DOM_DELTA_LINE
          ? 16
          : event.deltaMode === WheelEvent.DOM_DELTA_PAGE
            ? scrollElement.clientHeight
            : 1;

      event.preventDefault();
      event.stopPropagation();
      scrollElement.scrollBy({
        left: event.deltaX * deltaScale,
        top: event.deltaY * deltaScale,
      });
    };

    const frame = window.requestAnimationFrame(() => {
      canvas =
        scrollElement.querySelector<HTMLCanvasElement>(".DrawingCanvas") ??
        document.querySelector<HTMLCanvasElement>(".DrawingCanvas");
      canvas?.addEventListener("wheel", handleWheel, { passive: false });
    });

    return () => {
      window.cancelAnimationFrame(frame);
      canvas?.removeEventListener("wheel", handleWheel);
    };
  }, [activeTool, utilsVersion]);

  const handleDrawingComplete = useCallback(
    (dataUrl: string, position: ScaledPosition, strokes: DrawingStroke[]) => {
      onCreateDrawing(dataUrl, position, strokes, cropPageSnapshot(utilsRef.current, position));
    },
    [onCreateDrawing],
  );

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
            enableDrawingMode={activeTool === "drawing"}
            onDrawingComplete={handleDrawingComplete}
            onDrawingCancel={onDrawingCancel}
            drawingStrokeColor={colorSolid(color)}
            drawingStrokeWidth={drawingSize}
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
        </>
      )}
    </ReaderPdfLoader>
  );
}

export { annotationToHighlight };
