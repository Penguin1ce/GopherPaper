"use client";

import {
  ArrowLeft,
  BookMarked,
  Check,
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  Languages,
  ListTree,
  Maximize2,
  MessageSquarePlus,
  MousePointer2,
  Network,
  PencilLine,
  RotateCcw,
  Trash2,
  Type,
  ZoomIn,
  ZoomOut,
} from "lucide-react";
import { useState, type FormEvent, type ReactNode } from "react";
import type { PdfScaleValue } from "react-pdf-highlighter-plus";

import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Slider } from "@/components/ui/slider";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import {
  COLOR_KEYS,
  COLOR_META,
  DRAWING_SIZE_MAX,
  DRAWING_SIZE_MIN,
  DRAWING_SIZE_STEP,
  type AnnotationColor,
  type ReaderTool,
} from "@/app/reader/lib/annotations";
import { cn } from "@/lib/utils";

export interface ReaderToolbarPrefs {
  translateOpen: boolean;
  annotationsOpen: boolean;
  mindMapOpen: boolean;
  qaOpen: boolean;
  color: AnnotationColor;
  activeTool: ReaderTool;
  textSize: number;
  drawingSize: number;
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

function firstSliderValue(value: number | readonly number[], fallback: number) {
  if (Array.isArray(value)) return value[0] ?? fallback;
  return value;
}

function AnnotationStyleMenu({
  prefs,
  onColorChange,
  onTextSizeChange,
  onDrawingSizeChange,
  onDeleteLatestDrawing,
}: {
  prefs: ReaderToolbarPrefs;
  onColorChange: (color: AnnotationColor) => void;
  onTextSizeChange: (size: number) => void;
  onDrawingSizeChange: (size: number) => void;
  onDeleteLatestDrawing: () => void;
}) {
  const [open, setOpen] = useState(false);
  const selected = COLOR_META[prefs.color];
  const isFreetext = prefs.activeTool === "freetext";
  const isDrawing = prefs.activeTool === "drawing";
  const showSize = isFreetext || isDrawing;
  const colorLabel = isDrawing ? "画笔颜色" : isFreetext ? "文字颜色" : "高亮颜色";
  const triggerSizeText = isDrawing ? prefs.drawingSize.toFixed(1) : prefs.textSize.toFixed(0);
  return (
    <DropdownMenu open={open} onOpenChange={(nextOpen) => setOpen(nextOpen)}>
      <DropdownMenuTrigger
        render={
          <Button
            type="button"
            variant="ghost"
            size="sm"
            aria-label="批注颜色和尺寸"
            className="h-8 gap-1 rounded-md px-2"
          >
            <span className={cn("size-4 rounded-sm border", selected.className)} />
            {showSize && (
              <span className="min-w-5 text-xs tabular-nums text-muted-foreground">
                {triggerSizeText}
              </span>
            )}
            <ChevronDown className="size-3.5 text-muted-foreground" />
          </Button>
        }
      />
      <DropdownMenuContent align="center" className="w-60 p-2">
        <div className="px-2 pb-1 text-xs font-medium text-muted-foreground">{colorLabel}</div>
        <div className="space-y-1">
          {COLOR_KEYS.map((key) => {
            const meta = COLOR_META[key];
            const active = prefs.color === key;
            return (
              <button
                key={key}
                type="button"
                onClick={() => {
                  onColorChange(key);
                  setOpen(false);
                }}
                className="flex h-9 w-full items-center gap-3 rounded-md px-2 text-sm transition hover:bg-accent"
              >
                <Check className={cn("size-4", active ? "opacity-100" : "opacity-0")} />
                <span className={cn("size-5 rounded-sm border", meta.className)} />
                <span>{meta.label}</span>
              </button>
            );
          })}
        </div>
        {isFreetext && (
          <>
            <DropdownMenuSeparator />
            <div className="space-y-3 px-2 py-2">
              <div className="flex items-center justify-between text-sm">
                <span>文字大小</span>
                <span className="tabular-nums text-muted-foreground">
                  {prefs.textSize.toFixed(0)}
                </span>
              </div>
              <Slider
                min={10}
                max={28}
                step={1}
                value={[prefs.textSize]}
                onValueChange={(value) => onTextSizeChange(firstSliderValue(value, prefs.textSize))}
              />
            </div>
          </>
        )}
        {isDrawing && (
          <>
            <DropdownMenuSeparator />
            <div className="space-y-3 px-2 py-2">
              <div className="flex items-center justify-between text-sm">
                <span>画笔大小</span>
                <span className="tabular-nums text-muted-foreground">
                  {prefs.drawingSize.toFixed(1)}
                </span>
              </div>
              <Slider
                min={DRAWING_SIZE_MIN}
                max={DRAWING_SIZE_MAX}
                step={DRAWING_SIZE_STEP}
                value={[prefs.drawingSize]}
                onValueChange={(value) => onDrawingSizeChange(firstSliderValue(value, prefs.drawingSize))}
              />
            </div>
            <DropdownMenuSeparator />
            <DropdownMenuItem onClick={onDeleteLatestDrawing}>
              <Trash2 className="size-3.5" />
              删除最新绘画
            </DropdownMenuItem>
          </>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

export function ReaderToolbar({
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
  onToolChange,
  onColorChange,
  onTextSizeChange,
  onDrawingSizeChange,
  onDeleteLatestDrawing,
}: {
  title: string;
  currentPage: number;
  numPages: number;
  pageDraft: string;
  scaleValue: PdfScaleValue;
  prefs: ReaderToolbarPrefs;
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
  onToolChange: (tool: ReaderTool) => void;
  onColorChange: (color: AnnotationColor) => void;
  onTextSizeChange: (size: number) => void;
  onDrawingSizeChange: (size: number) => void;
  onDeleteLatestDrawing: () => void;
}) {
  const zoomText = typeof scaleValue === "number" ? `${Math.round(scaleValue * 100)}%` : "适宽";

  const submitPage = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    onPageSubmit();
  };

  return (
    <header className="relative z-[80] grid h-12 shrink-0 grid-cols-[minmax(0,1fr)_auto_minmax(0,1fr)] items-center gap-3 border-b bg-background/95 px-3 shadow-sm backdrop-blur">
      <div className="flex min-w-0 items-center gap-2">
        <ToolbarButton label="返回工作台" onClick={onClose}>
          <ArrowLeft className="size-4" />
        </ToolbarButton>
        <div className="min-w-0">
          <div className="truncate text-sm font-semibold">{title}</div>
        </div>
      </div>

      <div className="flex min-w-0 items-center justify-center gap-1">
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
          <ZoomOut className="size-4" />
        </ToolbarButton>
        <ToolbarButton label="放大" onClick={onZoomIn}>
          <ZoomIn className="size-4" />
        </ToolbarButton>
        <DropdownMenu>
          <DropdownMenuTrigger
            render={
              <Button type="button" variant="ghost" size="sm" className="h-8 min-w-12 rounded-md px-2 text-xs">
                {zoomText}
              </Button>
            }
          />
          <DropdownMenuContent align="center" className="w-36">
            <DropdownMenuItem onClick={onResetZoom}>
              <RotateCcw className="size-3.5" />
              100%
            </DropdownMenuItem>
            <DropdownMenuItem onClick={onFitWidth}>
              <Maximize2 className="size-3.5" />
              适宽
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>

        <ToolbarDivider />
        <ToolbarButton label="选择文本" active={prefs.activeTool === "select"} onClick={() => onToolChange("select")}>
          <MousePointer2 className="size-4" />
        </ToolbarButton>
        <ToolbarButton label="新增文字" active={prefs.activeTool === "freetext"} onClick={() => onToolChange("freetext")}>
          <Type className="size-4" />
        </ToolbarButton>
        <ToolbarButton label="绘画" active={prefs.activeTool === "drawing"} onClick={() => onToolChange("drawing")}>
          <PencilLine className="size-4" />
        </ToolbarButton>
        <AnnotationStyleMenu
          prefs={prefs}
          onColorChange={onColorChange}
          onTextSizeChange={onTextSizeChange}
          onDrawingSizeChange={onDrawingSizeChange}
          onDeleteLatestDrawing={onDeleteLatestDrawing}
        />
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

export function ReaderLeftRail({
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
