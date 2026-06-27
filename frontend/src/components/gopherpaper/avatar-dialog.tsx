"use client";

import { useEffect, useRef, useState, type ChangeEvent, type PointerEvent } from "react";
import { ImagePlus, Loader2, RotateCcw, Upload } from "lucide-react";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";

const MAX_AVATAR_BYTES = 2 * 1024 * 1024;
const OUTPUT_SIZE = 512;
const PREVIEW_SIZE = 176;
const acceptedTypes = new Set(["image/jpeg", "image/png", "image/webp"]);

interface AvatarDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  currentAvatarUrl?: string;
  fallback: string;
  onSave: (file: File) => Promise<void>;
  onClear: () => Promise<void>;
}

interface DragState {
  pointerId: number;
  startX: number;
  startY: number;
  offsetX: number;
  offsetY: number;
}

function clamp(value: number, min: number, max: number) {
  return Math.min(max, Math.max(min, value));
}

function drawAvatar(
  canvas: HTMLCanvasElement,
  image: HTMLImageElement,
  zoom: number,
  offsetX: number,
  offsetY: number,
) {
  const size = canvas.width;
  const ctx = canvas.getContext("2d");
  if (!ctx) return;
  ctx.clearRect(0, 0, size, size);
  ctx.fillStyle = "#fff";
  ctx.fillRect(0, 0, size, size);

  const baseScale = Math.max(size / image.naturalWidth, size / image.naturalHeight);
  const scale = baseScale * zoom;
  const width = image.naturalWidth * scale;
  const height = image.naturalHeight * scale;
  const maxX = Math.max(0, width - size) / 2;
  const maxY = Math.max(0, height - size) / 2;
  const x = (size - width) / 2 + clamp(offsetX, -1, 1) * maxX;
  const y = (size - height) / 2 + clamp(offsetY, -1, 1) * maxY;

  ctx.drawImage(image, x, y, width, height);
}

function canvasToBlob(canvas: HTMLCanvasElement, quality: number) {
  return new Promise<Blob>((resolve, reject) => {
    canvas.toBlob(
      (blob) => {
        if (blob) resolve(blob);
        else reject(new Error("头像生成失败"));
      },
      "image/webp",
      quality,
    );
  });
}

export function AvatarDialog({
  open,
  onOpenChange,
  currentAvatarUrl,
  fallback,
  onSave,
  onClear,
}: AvatarDialogProps) {
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const imageRef = useRef<HTMLImageElement | null>(null);
  const dragRef = useRef<DragState | null>(null);
  const [sourceUrl, setSourceUrl] = useState("");
  const [sourceIsObjectUrl, setSourceIsObjectUrl] = useState(false);
  const [fileName, setFileName] = useState("");
  const [zoom, setZoom] = useState(1);
  const [offsetX, setOffsetX] = useState(0);
  const [offsetY, setOffsetY] = useState(0);
  const [imageReady, setImageReady] = useState(false);
  const [saving, setSaving] = useState(false);
  const [clearing, setClearing] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!open) {
      setSourceUrl("");
      setSourceIsObjectUrl(false);
      setFileName("");
      setZoom(1);
      setOffsetX(0);
      setOffsetY(0);
      setError("");
      dragRef.current = null;
      return;
    }

    if (!sourceUrl && currentAvatarUrl) {
      setSourceUrl(currentAvatarUrl);
      setSourceIsObjectUrl(false);
      setFileName("当前头像");
      setZoom(1);
      setOffsetX(0);
      setOffsetY(0);
      setError("");
    }
  }, [currentAvatarUrl, open, sourceUrl]);

  useEffect(() => {
    if (!sourceUrl) {
      imageRef.current = null;
      setImageReady(false);
      return;
    }

    let cancelled = false;
    const objectUrl = sourceIsObjectUrl ? sourceUrl : "";
    const image = new Image();
    image.onload = () => {
      if (cancelled) return;
      imageRef.current = image;
      setImageReady(true);
    };
    image.onerror = () => {
      if (!cancelled) setError("图片读取失败，请换一张图片");
    };
    image.src = sourceUrl;

    return () => {
      cancelled = true;
      if (objectUrl) URL.revokeObjectURL(objectUrl);
    };
  }, [sourceIsObjectUrl, sourceUrl]);

  useEffect(() => {
    const canvas = canvasRef.current;
    const image = imageRef.current;
    if (!canvas || !imageReady || !image) return;
    drawAvatar(canvas, image, zoom, offsetX, offsetY);
  }, [imageReady, offsetX, offsetY, zoom]);

  function pickFile(file: File | undefined | null) {
    if (!file) return;
    if (!acceptedTypes.has(file.type)) {
      setError("仅支持 jpg、png、webp 图片");
      return;
    }
    if (file.size > MAX_AVATAR_BYTES) {
      setError("图片不能超过 2MB");
      return;
    }
    setError("");
    setZoom(1);
    setOffsetX(0);
    setOffsetY(0);
    setFileName(file.name);
    setSourceIsObjectUrl(true);
    setSourceUrl(URL.createObjectURL(file));
  }

  function onFileChange(event: ChangeEvent<HTMLInputElement>) {
    pickFile(event.target.files?.[0]);
    event.target.value = "";
  }

  function onPointerDown(event: PointerEvent<HTMLCanvasElement>) {
    if (!sourceUrl) return;
    event.currentTarget.setPointerCapture(event.pointerId);
    dragRef.current = {
      pointerId: event.pointerId,
      startX: event.clientX,
      startY: event.clientY,
      offsetX,
      offsetY,
    };
  }

  function onPointerMove(event: PointerEvent<HTMLCanvasElement>) {
    const drag = dragRef.current;
    if (!drag || drag.pointerId !== event.pointerId) return;
    const deltaX = (event.clientX - drag.startX) / (PREVIEW_SIZE / 2);
    const deltaY = (event.clientY - drag.startY) / (PREVIEW_SIZE / 2);
    setOffsetX(clamp(drag.offsetX + deltaX, -1, 1));
    setOffsetY(clamp(drag.offsetY + deltaY, -1, 1));
  }

  function onPointerEnd(event: PointerEvent<HTMLCanvasElement>) {
    if (dragRef.current?.pointerId === event.pointerId) dragRef.current = null;
  }

  async function buildAvatarFile() {
    const image = imageRef.current;
    if (!image || !imageReady) throw new Error("请先选择一张图片");
    const canvas = document.createElement("canvas");
    canvas.width = OUTPUT_SIZE;
    canvas.height = OUTPUT_SIZE;
    drawAvatar(canvas, image, zoom, offsetX, offsetY);

    let blob = await canvasToBlob(canvas, 0.9);
    if (blob.size > MAX_AVATAR_BYTES) blob = await canvasToBlob(canvas, 0.78);
    if (blob.size > MAX_AVATAR_BYTES) throw new Error("头像生成后仍超过 2MB，请换一张图片");
    return new File([blob], "avatar.webp", { type: "image/webp" });
  }

  async function saveAvatar() {
    try {
      setSaving(true);
      setError("");
      await onSave(await buildAvatarFile());
      onOpenChange(false);
    } catch (err) {
      setError(err instanceof Error ? err.message : "头像保存失败");
    } finally {
      setSaving(false);
    }
  }

  async function clearAvatar() {
    try {
      setClearing(true);
      setError("");
      await onClear();
      onOpenChange(false);
    } catch (err) {
      setError(err instanceof Error ? err.message : "恢复默认头像失败");
    } finally {
      setClearing(false);
    }
  }

  const busy = saving || clearing;

  return (
    <Dialog open={open} onOpenChange={(next) => !busy && onOpenChange(next)}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>更改头像</DialogTitle>
          <DialogDescription>
            上传 jpg、png 或 webp 图片，最大 2MB。已有头像也可以直接拖动、缩放后重新保存。
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4">
          <label
            className={cn(
              "flex cursor-pointer flex-col items-center justify-center rounded-xl border border-dashed bg-muted/30 px-4 py-6 text-center transition hover:border-primary/50 hover:bg-muted/50",
              sourceUrl && sourceIsObjectUrl && "border-primary/50 bg-primary/5",
            )}
            onDragOver={(event) => event.preventDefault()}
            onDrop={(event) => {
              event.preventDefault();
              pickFile(event.dataTransfer.files?.[0]);
            }}
          >
            <Input
              type="file"
              accept="image/jpeg,image/png,image/webp,.jpg,.jpeg,.png,.webp"
              className="sr-only"
              onChange={onFileChange}
            />
            <ImagePlus className="mb-2 size-6 text-muted-foreground" />
            <span className="max-w-full truncate text-sm font-medium">
              {fileName || "选择或拖入头像图片"}
            </span>
            <span className="mt-1 text-xs text-muted-foreground">
              可换图，也可直接编辑当前头像
            </span>
          </label>

          <div className="flex flex-col items-center gap-2">
            <div className="relative grid size-44 place-items-center overflow-hidden rounded-2xl border bg-muted/40 shadow-inner">
              {sourceUrl ? (
                <>
                  <canvas
                    ref={canvasRef}
                    width={PREVIEW_SIZE}
                    height={PREVIEW_SIZE}
                    className="size-44 touch-none cursor-grab active:cursor-grabbing"
                    onPointerDown={onPointerDown}
                    onPointerMove={onPointerMove}
                    onPointerUp={onPointerEnd}
                    onPointerCancel={onPointerEnd}
                  />
                  <div className="pointer-events-none absolute inset-0 rounded-2xl ring-2 ring-primary/30" />
                  <div className="pointer-events-none absolute inset-x-0 top-1/2 border-t border-white/70" />
                  <div className="pointer-events-none absolute inset-y-0 left-1/2 border-l border-white/70" />
                </>
              ) : (
                <span className="font-serif text-5xl font-semibold text-primary">{fallback}</span>
              )}
            </div>
            <span className="text-xs text-muted-foreground">
              {sourceUrl ? "按住图片拖动，可实时调整头像裁剪位置" : "当前为默认头像"}
            </span>
          </div>

          <label className="block space-y-2 text-sm">
            <div className="flex items-center justify-between">
              <span className="text-muted-foreground">缩放</span>
              <span className="font-mono text-xs text-muted-foreground">{zoom.toFixed(2)}x</span>
            </div>
            <input
              type="range"
              min={1}
              max={2}
              step={0.01}
              value={zoom}
              disabled={!sourceUrl}
              className="w-full accent-primary disabled:opacity-40"
              onChange={(event) => setZoom(Number(event.target.value))}
            />
          </label>

          {error && <p className="rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">{error}</p>}
        </div>

        <DialogFooter>
          <Button type="button" variant="outline" disabled={busy} onClick={clearAvatar}>
            {clearing ? <Loader2 className="size-4 animate-spin" /> : <RotateCcw className="size-4" />}
            恢复默认
          </Button>
          <Button type="button" disabled={busy || !sourceUrl} onClick={saveAvatar}>
            {saving ? <Loader2 className="size-4 animate-spin" /> : <Upload className="size-4" />}
            保存头像
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
