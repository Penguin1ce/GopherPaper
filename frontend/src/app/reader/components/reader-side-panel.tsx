"use client";

import {
  BookMarked,
  Check,
  Languages,
  Loader2,
  MoreHorizontal,
  PencilLine,
  Trash2,
  Type,
  X,
} from "lucide-react";
import type { KeyboardEvent, MouseEvent } from "react";

import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { ScrollArea } from "@/components/ui/scroll-area";
import type { PaperAnnotation } from "@/lib/gopherpaper/types";
import {
  COLOR_KEYS,
  COLOR_META,
  annotationColor,
  annotationDisplayText,
  annotationKind,
  freetextStyle,
  type AnnotationColor,
} from "@/app/reader/lib/annotations";
import { cn } from "@/lib/utils";

const RIGHT_PANEL_WIDTH = "24rem";

export interface TranslationResult {
  original: string;
  translation: string;
  pageNo: number;
  loading: boolean;
  error: string;
}

function KindIcon({ annotation }: { annotation: PaperAnnotation }) {
  const kind = annotationKind(annotation);
  if (kind === "freetext") return <Type className="size-3.5 text-muted-foreground" />;
  if (kind === "drawing") return <PencilLine className="size-3.5 text-muted-foreground" />;
  return <BookMarked className="size-3.5 text-muted-foreground" />;
}

function kindLabel(annotation: PaperAnnotation) {
  const kind = annotationKind(annotation);
  if (kind === "freetext") return "页面文字";
  if (kind === "drawing") return "手绘标注";
  return annotation.note ? "文本批注" : "文本高亮";
}

function AnnotationColorMenu({
  value,
  disabled,
  onChange,
}: {
  value: string;
  disabled?: boolean;
  onChange: (color: AnnotationColor) => void;
}) {
  const color = annotationColor(value);
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={
          <Button
            type="button"
            variant="ghost"
            size="icon-xs"
            aria-label="修改颜色"
            disabled={disabled}
          >
            <span className={cn("size-3.5 rounded-sm border", COLOR_META[color].className)} />
          </Button>
        }
      />
      <DropdownMenuContent align="end" className="w-36">
        {COLOR_KEYS.map((key) => (
          <DropdownMenuItem key={key} onClick={() => onChange(key)}>
            <Check className={cn("size-3.5", key === color ? "opacity-100" : "opacity-0")} />
            <span className={cn("size-3.5 rounded-sm border", COLOR_META[key].className)} />
            {COLOR_META[key].label}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

function imageDataUrl(value: unknown) {
  if (typeof value === "string" && value) return value;
  if (value && typeof value === "object" && "dataUrl" in value) {
    const dataUrl = (value as { dataUrl?: unknown }).dataUrl;
    return typeof dataUrl === "string" ? dataUrl : "";
  }
  return "";
}

function drawingPreviewImages(annotation: PaperAnnotation) {
  const content = annotation.content_json ?? {};
  return {
    snapshot: imageDataUrl(content.snapshot),
    drawing: imageDataUrl(content.image),
  };
}

function isPanelControl(target: EventTarget | null) {
  if (!(target instanceof HTMLElement)) return false;
  return Boolean(target.closest("button, input, textarea, select, [role='menuitem']"));
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
  selected,
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
  selected: boolean;
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
  const kind = annotationKind(annotation);
  const canTranslate = kind === "selection" && annotation.text.trim().length > 0;
  const color = annotationColor(annotation.color);
  const drawingPreview = kind === "drawing" ? drawingPreviewImages(annotation) : null;
  const hasDrawingPreview = Boolean(drawingPreview?.snapshot || drawingPreview?.drawing);
  const freetextPanelStyle = kind === "freetext" ? freetextStyle(annotation) : null;
  const locateFromCard = (event: MouseEvent<HTMLElement>) => {
    if (isPanelControl(event.target)) return;
    onLocate(annotation);
  };
  const locateFromKeyboard = (event: KeyboardEvent<HTMLElement>) => {
    if (event.key !== "Enter" && event.key !== " ") return;
    if (isPanelControl(event.target)) return;
    event.preventDefault();
    onLocate(annotation);
  };
  return (
    <article
      data-reader-side-annotation-id={annotation.id}
      role="button"
      tabIndex={0}
      aria-current={selected ? "true" : undefined}
      onClick={locateFromCard}
      onKeyDown={locateFromKeyboard}
      className={cn(
        "rounded-md border bg-background p-3 shadow-sm outline-none transition hover:border-foreground/20 hover:shadow-md focus-visible:ring-2 focus-visible:ring-ring/30",
        selected &&
          "border-sky-300 bg-sky-50/40 shadow-[0_0_0_2px_rgba(14,165,233,0.16),0_14px_30px_rgba(15,23,42,0.16)]",
      )}
    >
      <div className="flex items-center justify-between gap-2">
        <button
          type="button"
          onClick={() => onLocate(annotation)}
          className="flex min-w-0 items-center gap-2 text-left text-sm font-semibold"
        >
          <span className={cn("size-3 shrink-0 rounded-sm", COLOR_META[color].className)} />
          <span className="truncate">页 {annotation.page_no}</span>
        </button>
        <div className="flex items-center gap-1">
          <AnnotationColorMenu
            value={annotation.color}
            disabled={busy === `color-${annotation.id}`}
            onChange={(nextColor) => onColorChange(annotation, nextColor)}
          />
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
              {canTranslate && (
                <DropdownMenuItem onClick={() => onRetryTranslate(annotation)}>
                  {annotation.translation ? "重新翻译" : "翻译"}
                </DropdownMenuItem>
              )}
              <DropdownMenuSeparator />
              <DropdownMenuItem
                disabled={busy === `delete-${annotation.id}`}
                onClick={() => onDelete(annotation)}
                variant="destructive"
              >
                <Trash2 className="size-3.5" />
                删除
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </div>
      <div className="mt-2 flex items-center gap-1.5 text-xs text-muted-foreground">
        <KindIcon annotation={annotation} />
        {kindLabel(annotation)}
      </div>
      <div className="mt-3 space-y-3 text-sm">
        {hasDrawingPreview ? (
          <button
            type="button"
            onClick={() => onLocate(annotation)}
            className="block w-full overflow-hidden rounded-md border bg-white text-left shadow-inner transition hover:border-foreground/30"
          >
            <span className="relative block h-32 w-full">
              {drawingPreview?.snapshot && (
                <img
                  src={drawingPreview.snapshot}
                  alt={`第 ${annotation.page_no} 页原文预览`}
                  className="absolute inset-0 size-full object-contain"
                />
              )}
              {drawingPreview?.drawing && (
                <img
                  src={drawingPreview.drawing}
                  alt={`第 ${annotation.page_no} 页手绘线条`}
                  className={cn(
                    "absolute inset-0 size-full object-contain",
                    drawingPreview.snapshot ? "" : "p-2",
                  )}
                />
              )}
            </span>
          </button>
        ) : (
          <button
            type="button"
            aria-expanded={textExpanded}
            onClick={() => {
              onLocate(annotation);
              if (kind === "freetext") {
                return;
              }
              onToggleTextExpanded(annotation);
            }}
            className={cn(
              "w-full rounded-md text-left leading-6 transition",
              kind === "freetext"
                ? "border p-2 hover:opacity-90"
                : "bg-muted px-3 py-2 text-muted-foreground hover:bg-muted/80",
              !textExpanded && "line-clamp-3",
            )}
            style={
              freetextPanelStyle
                ? {
                    backgroundColor: freetextPanelStyle.backgroundColor,
                    color: freetextPanelStyle.color,
                  }
                : undefined
            }
          >
            {annotationDisplayText(annotation)}
          </button>
        )}
        {canTranslate && (
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
                className={cn("w-full text-left leading-6 transition", !transExpanded && "line-clamp-3")}
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
        )}
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
      </div>
    </article>
  );
}

function AnnotationPanel({
  annotations,
  selectedAnnotationId,
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
  selectedAnnotationId: number | null;
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
              选中 PDF 原文添加高亮，或点击页面添加文字批注。
            </div>
          ) : (
            annotations.map((annotation) => (
              <AnnotationCard
                key={annotation.id}
                annotation={annotation}
                selected={selectedAnnotationId === annotation.id}
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

export function ReaderSidePanel({
  showTranslation,
  showAnnotations,
  translation,
  annotations,
  selectedAnnotationId,
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
  selectedAnnotationId: number | null;
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
        {showTranslation && <TranslationPanel translation={translation} onClear={onClearTranslation} />}
        {showAnnotations && (
          <AnnotationPanel
            annotations={annotations}
            selectedAnnotationId={selectedAnnotationId}
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
