"use client";

import "@/lib/pdfjs-global";
import {
  BookMarked,
  Edit3,
  Languages,
  Loader2,
  MessageCircleQuestionMark,
  MessageSquarePlus,
  Trash2,
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
  DrawingHighlight,
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
  annotationDisplayText,
  annotationKind,
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
const FREETEXT_MIN_WIDTH = 64;
const FREETEXT_TEXT_MIN_HEIGHT = 18;
const FREETEXT_HEIGHT_EPSILON = 1;
const FREETEXT_RESIZE_ENABLE = {
  top: false,
  right: true,
  bottom: false,
  left: false,
  topRight: false,
  bottomRight: false,
  bottomLeft: false,
  topLeft: false,
};
const FREETEXT_RESIZE_HANDLE_STYLES = {
  right: {
    right: -6,
    width: 12,
    cursor: "ew-resize",
  },
};
const FREETEXT_RESIZE_HANDLE_CLASSES = {
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

function HighlightTip({
  annotation,
  onDelete,
}: {
  annotation: PaperAnnotation;
  onDelete: (annotation: PaperAnnotation) => void;
}) {
  return (
    <div className="max-w-xs rounded-lg border bg-popover p-3 text-xs text-popover-foreground shadow-lg">
      <div className="line-clamp-3 leading-5">{annotationDisplayText(annotation)}</div>
      {annotation.translation && annotationKind(annotation) !== "drawing" && (
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
        ".FreetextHighlight__text",
        ".DrawingHighlight__toolbar",
        ".DrawingHighlight__style-controls",
      ].join(", "),
    ),
  );
}

function shouldSelectFreetextInput(target: EventTarget | null) {
  if (!(target instanceof HTMLElement)) return false;
  return Boolean(target.closest(".FreetextHighlight__edit-button, .FreetextHighlight__text"));
}

function focusFreetextInputEnd(root: HTMLElement | null) {
  const input = root?.querySelector<HTMLTextAreaElement>(".FreetextHighlight__input");
  if (!input) return false;
  input.focus();
  input.setSelectionRange(input.value.length, input.value.length);
  return true;
}

function scaledFromViewportRect(rect: LTWHP, utils: PdfHighlighterUtils) {
  const viewer = utils.getViewer();
  if (!viewer) return null;
  return viewportPositionToScaled({ boundingRect: rect, rects: [] }, viewer);
}

function clamp(value: number, min: number, max: number) {
  return Math.min(max, Math.max(min, value));
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
  color,
  backgroundColor,
  fontSize,
  onChange,
  onTextChange,
  onEditStart,
  onEditEnd,
  onDelete,
}: {
  highlight: ViewportHighlight<ReaderHighlight>;
  isScrolledTo: boolean;
  color: string;
  backgroundColor: string;
  fontSize: string;
  onChange: (rect: LTWHP) => void;
  onTextChange: (text: string) => void;
  onEditStart: () => void;
  onEditEnd?: () => void;
  onDelete: () => void;
}) {
  const rect = highlight.position.boundingRect;
  const width = Math.max(FREETEXT_MIN_WIDTH, rect.width || FREETEXT_DEFAULT_WIDTH);
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
  }, [highlight.content?.text, onEditStart, text]);

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
      onChange(rectWith({ left: data.x, top: data.y }));
    },
    [onChange, rectWith],
  );

  const handleResizeStop: RndResizeCallback = useCallback(
    (_event, _direction, ref, _delta, position) => {
      const nextWidth = Math.max(FREETEXT_MIN_WIDTH, ref.offsetWidth);
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
    [measuredHeight, onChange, rectWith],
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
          border: "1px solid rgba(15, 23, 42, 0.14)",
          borderRadius: 6,
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
        minWidth={FREETEXT_MIN_WIDTH}
        minHeight={FREETEXT_DEFAULT_HEIGHT}
        enableResizing={FREETEXT_RESIZE_ENABLE}
        resizeHandleClasses={FREETEXT_RESIZE_HANDLE_CLASSES}
        resizeHandleStyles={FREETEXT_RESIZE_HANDLE_STYLES}
        onDragStart={() => onEditStart()}
        onDragStop={handleDragStop}
        onResizeStart={() => onEditStart()}
        onResize={(_event, _direction, ref) => {
          setDraftWidth(ref.offsetWidth);
          window.requestAnimationFrame(syncVisualHeight);
        }}
        onResizeStop={handleResizeStop}
        cancel=".FreetextHighlight__text, .FreetextHighlight__input, .FreetextHighlight__edit-button, .FreetextHighlight__delete-button"
      >
        <div className="FreetextHighlight__container" data-reader-freetext-container="true">
          <div className="FreetextHighlight__toolbar">
            <button
              type="button"
              title="Edit text"
              className="FreetextHighlight__edit-button"
              onMouseDown={(event) => event.preventDefault()}
              onClick={(event) => {
                event.stopPropagation();
                startEditing();
              }}
            >
              <Edit3 className="size-3.5" />
            </button>
            <button
              type="button"
              title="Delete"
              className="FreetextHighlight__delete-button"
              onClick={(event) => {
                event.stopPropagation();
                onDelete();
              }}
            >
              <Trash2 className="size-3.5" />
            </button>
          </div>
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
                  event.stopPropagation();
                  startEditing();
                }}
                onClick={(event) => {
                  event.stopPropagation();
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

function HighlightContainer({
  onDelete,
  onUpdatePosition,
  onUpdateText,
  onUpdateDrawing,
  locatedAnnotationId,
  pendingFreetextFocusId,
  suppressTipAnnotationId,
  onFreetextFocusHandled,
}: {
  onDelete: (annotation: PaperAnnotation) => void;
  onUpdatePosition: (
    annotation: PaperAnnotation,
    position: ScaledPosition,
    snapshot?: string,
  ) => void;
  onUpdateText: (annotation: PaperAnnotation, text: string) => void;
  onUpdateDrawing: (annotation: PaperAnnotation, image: string, strokes: DrawingStroke[]) => void;
  locatedAnnotationId: number | null;
  pendingFreetextFocusId: number | null;
  suppressTipAnnotationId: number | null;
  onFreetextFocusHandled: (annotationID: number) => void;
}) {
  const { highlight, isScrolledTo } = useHighlightContainerContext<ReaderHighlight>();
  const utils = usePdfHighlighterContext();
  const rootRef = useRef<HTMLDivElement>(null);
  const annotation = highlight.annotation;
  const scrolled = isScrolledTo || locatedAnnotationId === annotation.id;

  const openAnnotationTip = (event: ReactMouseEvent<HTMLDivElement>) => {
    if (highlight.type === "drawing") return;
    if (highlight.type === "freetext") return;
    if (annotation.id === suppressTipAnnotationId) return;
    if (shouldIgnoreAnnotationClick(event.target)) return;
    utils.setTip({
      position: highlight.position,
      content: (
        <HighlightTip
          annotation={annotation}
          onDelete={(item) => {
            utils.setTip(null);
            onDelete(item);
          }}
        />
      ),
    });
  };

  const selectFreetextInputAfterEdit = (event: ReactMouseEvent<HTMLDivElement>) => {
    if (highlight.type !== "freetext" || !shouldSelectFreetextInput(event.target)) return;
    window.setTimeout(() => {
      if (focusFreetextInputEnd(rootRef.current)) return;
      window.requestAnimationFrame(() => {
        focusFreetextInputEnd(rootRef.current);
      });
    }, 0);
  };

  useEffect(() => {
    if (highlight.type !== "freetext" || pendingFreetextFocusId !== annotation.id) return;
    let cancelled = false;
    const frame = window.requestAnimationFrame(() => {
      if (cancelled) return;
      const root = rootRef.current;
      if (focusFreetextInputEnd(root)) {
        onFreetextFocusHandled(annotation.id);
        return;
      }
      root?.querySelector<HTMLElement>(".FreetextHighlight__text")?.click();
      window.setTimeout(() => {
        if (cancelled) return;
        focusFreetextInputEnd(root);
        onFreetextFocusHandled(annotation.id);
      }, 0);
    });
    return () => {
      cancelled = true;
      window.cancelAnimationFrame(frame);
    };
  }, [annotation.id, highlight.type, onFreetextFocusHandled, pendingFreetextFocusId]);

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
    content = (
      <ReaderFreetextHighlight
        highlight={highlight}
        isScrolledTo={scrolled}
        color={style.color}
        backgroundColor={style.backgroundColor}
        fontSize={`${style.fontSize}px`}
        onChange={handleChange}
        onTextChange={(text) => onUpdateText(annotation, text)}
        onEditStart={() => utils.setTip(null)}
        onDelete={() => onDelete(annotation)}
      />
    );
  } else if (highlight.type === "drawing") {
    content = (
      <DrawingHighlight
        highlight={highlight}
        isScrolledTo={scrolled}
        onChange={handleChange}
        onStyleChange={(image, strokes) => onUpdateDrawing(annotation, image, strokes)}
        onDelete={() => onDelete(annotation)}
      />
    );
  } else {
    content = (
      <TextHighlight
        highlight={highlight}
        isScrolledTo={scrolled}
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
      onClickCapture={selectFreetextInputAfterEdit}
      onClick={openAnnotationTip}
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
  onUpdateAnnotationDrawing,
  onDeleteAnnotation,
  locatedAnnotationId,
  pendingFreetextFocusId,
  suppressTipAnnotationId,
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
  onUpdateAnnotationDrawing: (annotation: PaperAnnotation, image: string, strokes: DrawingStroke[]) => void;
  onDeleteAnnotation: (annotation: PaperAnnotation) => void;
  locatedAnnotationId: number | null;
  pendingFreetextFocusId: number | null;
  suppressTipAnnotationId: number | null;
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
              scrollbarThumbColor: "oklch(0.72 0.01 230)",
              scrollbarTrackColor: "oklch(0.92 0.004 230)",
            }}
            style={{ height: "100%" }}
          >
            <HighlightContainer
              onDelete={onDeleteAnnotation}
              onUpdatePosition={onUpdateAnnotationPosition}
              onUpdateText={onUpdateAnnotationText}
              onUpdateDrawing={onUpdateAnnotationDrawing}
              locatedAnnotationId={locatedAnnotationId}
              pendingFreetextFocusId={pendingFreetextFocusId}
              suppressTipAnnotationId={suppressTipAnnotationId}
              onFreetextFocusHandled={onFreetextFocusHandled}
            />
          </PdfHighlighter>
        </>
      )}
    </ReaderPdfLoader>
  );
}

export { annotationToHighlight };
