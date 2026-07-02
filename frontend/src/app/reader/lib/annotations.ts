import type {
  DrawingStroke,
  Highlight,
  Scaled,
  ScaledPosition,
} from "react-pdf-highlighter-plus";

import type {
  AnnotationRect,
  PaperAnnotation,
  PaperAnnotationKind,
} from "@/lib/gopherpaper/types";

export type AnnotationColor =
  | "yellow"
  | "red"
  | "green"
  | "blue"
  | "purple"
  | "magenta"
  | "orange"
  | "gray"
  | "black";

export type ReaderTool = "select" | "freetext" | "drawing";

export interface ColorMeta {
  label: string;
  className: string;
  value: string;
  solid: string;
  foreground: string;
}

export const COLOR_META: Record<AnnotationColor, ColorMeta> = {
  yellow: {
    label: "黄色",
    className: "bg-amber-300",
    value: "rgba(255, 226, 143, 0.62)",
    solid: "#facc15",
    foreground: "#713f12",
  },
  red: {
    label: "红色",
    className: "bg-red-400",
    value: "rgba(248, 113, 113, 0.5)",
    solid: "#f87171",
    foreground: "#7f1d1d",
  },
  green: {
    label: "绿色",
    className: "bg-green-500",
    value: "rgba(74, 222, 128, 0.48)",
    solid: "#22c55e",
    foreground: "#14532d",
  },
  blue: {
    label: "蓝色",
    className: "bg-sky-400",
    value: "rgba(56, 189, 248, 0.42)",
    solid: "#38bdf8",
    foreground: "#0c4a6e",
  },
  purple: {
    label: "紫色",
    className: "bg-violet-400",
    value: "rgba(167, 139, 250, 0.46)",
    solid: "#a78bfa",
    foreground: "#3b0764",
  },
  magenta: {
    label: "洋红色",
    className: "bg-fuchsia-400",
    value: "rgba(217, 70, 239, 0.38)",
    solid: "#d946ef",
    foreground: "#701a75",
  },
  orange: {
    label: "橙色",
    className: "bg-orange-400",
    value: "rgba(251, 146, 60, 0.48)",
    solid: "#fb923c",
    foreground: "#7c2d12",
  },
  gray: {
    label: "灰色",
    className: "bg-neutral-400",
    value: "rgba(163, 163, 163, 0.45)",
    solid: "#a3a3a3",
    foreground: "#262626",
  },
  black: {
    label: "黑色",
    className: "bg-black",
    value: "rgba(24, 24, 27, 0.3)",
    solid: "#000000",
    foreground: "#000000",
  },
};

export const COLOR_KEYS = Object.keys(COLOR_META) as AnnotationColor[];
export const DEFAULT_COLOR: AnnotationColor = "yellow";
export const DEFAULT_TEXT_SIZE = 14;
export const DEFAULT_DRAWING_SIZE = 2;
export const FREETEXT_DEFAULT_WIDTH = 104;
export const FREETEXT_DEFAULT_HEIGHT = 32;
export const DRAWING_LABEL = "手绘标注";
export const FREETEXT_LABEL = "文字批注";
export const FREETEXT_CREATE_TEXT = "新增文字";
export const FREETEXT_EMPTY_DRAFT = "\u200B";

export function normalizeFreetextText(text: string) {
  return text.replace(/\u200B/g, "").trim();
}

function freetextDraftText(text: string) {
  const normalized = normalizeFreetextText(text);
  return !normalized || normalized === FREETEXT_CREATE_TEXT ? FREETEXT_EMPTY_DRAFT : text;
}

export interface ReaderHighlight extends Highlight {
  annotation: PaperAnnotation;
  type: "text" | "freetext" | "drawing";
  content: {
    text?: string;
    image?: string;
    strokes?: DrawingStroke[];
  };
}

export function annotationKind(annotation: PaperAnnotation): PaperAnnotationKind {
  if (
    annotation.kind === "freetext" ||
    annotation.kind === "drawing" ||
    annotation.kind === "selection"
  ) {
    return annotation.kind;
  }
  return "selection";
}

export function isAnnotationColor(color: string): color is AnnotationColor {
  return COLOR_KEYS.includes(color as AnnotationColor);
}

export function annotationColor(color?: string): AnnotationColor {
  return color && isAnnotationColor(color) ? color : DEFAULT_COLOR;
}

export function colorValue(color?: string) {
  return COLOR_META[annotationColor(color)].value;
}

export function colorSolid(color?: string) {
  return COLOR_META[annotationColor(color)].solid;
}

export function colorForeground(color?: string) {
  return COLOR_META[annotationColor(color)].foreground;
}

export function freetextStyleForColor(color?: string, fontSize = DEFAULT_TEXT_SIZE) {
  return {
    color: colorForeground(color),
    backgroundColor: colorValue(color),
    fontSize,
  };
}

export function drawingStyleForColor(color?: string, strokeWidth = DEFAULT_DRAWING_SIZE) {
  return {
    strokeColor: colorSolid(color),
    strokeWidth,
  };
}

export function rectToScaled(rect: AnnotationRect): Scaled {
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

export function scaledToRect(rect: Scaled): AnnotationRect {
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

export function positionToRects(position: ScaledPosition) {
  const boundingRect = scaledToRect(position.boundingRect);
  const rects = position.rects.length > 0 ? position.rects.map(scaledToRect) : [boundingRect];
  return { boundingRect, rects };
}

export function resizeScaledPosition(
  position: ScaledPosition,
  desiredWidth: number,
  desiredHeight: number,
): ScaledPosition {
  const rect = position.boundingRect;
  const width = Math.min(desiredWidth, Math.max(1, rect.width));
  const height = Math.min(desiredHeight, Math.max(1, rect.height));
  const left = Math.min(Math.max(0, rect.x1), Math.max(0, rect.width - width));
  const top = Math.min(Math.max(0, rect.y1), Math.max(0, rect.height - height));
  const boundingRect: Scaled = {
    ...rect,
    x1: left,
    y1: top,
    x2: left + width,
    y2: top + height,
  };
  return { boundingRect, rects: [] };
}

export function defaultFreetextPosition(position: ScaledPosition): ScaledPosition {
  return resizeScaledPosition(position, FREETEXT_DEFAULT_WIDTH, FREETEXT_DEFAULT_HEIGHT);
}

export function freetextStyle(annotation: PaperAnnotation) {
  const style = annotation.style_json ?? {};
  const fontSize =
    typeof style.font_size === "number"
      ? style.font_size
      : typeof style.fontSize === "number"
        ? style.fontSize
        : DEFAULT_TEXT_SIZE;
  const color =
    typeof style.text_color === "string"
      ? style.text_color
      : typeof style.color === "string"
        ? style.color
        : colorForeground(annotation.color);
  const backgroundColor =
    typeof style.background_color === "string"
      ? style.background_color
      : typeof style.backgroundColor === "string"
        ? style.backgroundColor
        : colorValue(annotation.color);
  return { color, backgroundColor, fontSize };
}

export function drawingStyle(annotation: PaperAnnotation) {
  const style = annotation.style_json ?? {};
  const strokeWidth =
    typeof style.stroke_width === "number"
      ? style.stroke_width
      : typeof style.strokeWidth === "number"
        ? style.strokeWidth
        : DEFAULT_DRAWING_SIZE;
  const strokeColor =
    typeof style.stroke_color === "string"
      ? style.stroke_color
      : typeof style.strokeColor === "string"
        ? style.strokeColor
        : colorSolid(annotation.color);
  return { strokeColor, strokeWidth };
}

function drawingStrokes(content: Record<string, unknown> | undefined) {
  const strokes = content?.strokes;
  return Array.isArray(strokes) ? (strokes as DrawingStroke[]) : undefined;
}

export function annotationDisplayText(annotation: PaperAnnotation) {
  const kind = annotationKind(annotation);
  if (kind === "drawing") return annotation.text?.trim() || DRAWING_LABEL;
  if (kind === "freetext") {
    const text = normalizeFreetextText(annotation.text);
    return text && text !== FREETEXT_CREATE_TEXT ? text : FREETEXT_LABEL;
  }
  return annotation.text?.trim() || annotation.note?.trim() || DRAWING_LABEL;
}

export function annotationToHighlight(annotation: PaperAnnotation): ReaderHighlight {
  const kind = annotationKind(annotation);
  const content = annotation.content_json ?? {};
  const position = {
    boundingRect: rectToScaled(annotation.bounding_rect),
    rects: (annotation.rects?.length ? annotation.rects : [annotation.bounding_rect]).map(rectToScaled),
  };

  if (kind === "freetext") {
    return {
      id: String(annotation.id),
      type: "freetext",
      annotation,
      content: { text: freetextDraftText(annotation.text) },
      position,
    };
  }

  if (kind === "drawing") {
    const image = typeof content.image === "string" ? content.image : "";
    return {
      id: String(annotation.id),
      type: "drawing",
      annotation,
      content: { image, strokes: drawingStrokes(content) },
      position,
    };
  }

  return {
    id: String(annotation.id),
    type: "text",
    annotation,
    content: { text: annotation.text },
    position,
  };
}
