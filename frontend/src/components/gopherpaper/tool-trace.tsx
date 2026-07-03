"use client";

import {
  BookOpenText,
  CheckCircle2,
  ChevronDown,
  CircleAlert,
  Clock3,
  Coffee,
  FileSearch,
  Globe2,
  GraduationCap,
  ImageIcon,
  Loader2,
  MapPin,
  PackageCheck,
  Puzzle,
  Trash2,
  Wrench,
  type LucideIcon,
} from "lucide-react";
import { useState } from "react";

import { Shimmer } from "@/components/ai-elements/shimmer";
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/components/ui/collapsible";
import { toolDisplayInfo, type ToolTone } from "@/lib/gopherpaper/tool-status";
import type { PlanStep } from "@/lib/gopherpaper/types";
import { toolPlanSteps } from "@/lib/gopherpaper/utils";
import { cn } from "@/lib/utils";

const TOOL_TONE_STYLE: Record<
  ToolTone,
  { chip: string; icon: string; dot: string; raw: string; row: string }
> = {
  paper: {
    chip: "border-primary/20 bg-primary/[0.06] text-primary",
    icon: "text-primary",
    dot: "bg-primary",
    raw: "text-primary/70",
    row: "bg-primary/[0.035]",
  },
  figure: {
    chip: "border-violet-200 bg-violet-50 text-violet-700 dark:border-violet-400/20 dark:bg-violet-400/10 dark:text-violet-300",
    icon: "text-violet-700 dark:text-violet-300",
    dot: "bg-violet-500",
    raw: "text-violet-700/65 dark:text-violet-300/70",
    row: "bg-violet-50/50 dark:bg-violet-400/5",
  },
  library: {
    chip: "border-slate-200 bg-slate-50 text-slate-700 dark:border-slate-400/20 dark:bg-slate-400/10 dark:text-slate-300",
    icon: "text-slate-700 dark:text-slate-300",
    dot: "bg-slate-500",
    raw: "text-slate-600 dark:text-slate-300/70",
    row: "bg-slate-50/60 dark:bg-slate-400/5",
  },
  web: {
    chip: "border-sky-200 bg-sky-50 text-sky-700 dark:border-sky-400/20 dark:bg-sky-400/10 dark:text-sky-300",
    icon: "text-sky-700 dark:text-sky-300",
    dot: "bg-sky-500",
    raw: "text-sky-700/65 dark:text-sky-300/70",
    row: "bg-sky-50/60 dark:bg-sky-400/5",
  },
  academic: {
    chip: "border-indigo-200 bg-indigo-50 text-indigo-700 dark:border-indigo-400/20 dark:bg-indigo-400/10 dark:text-indigo-300",
    icon: "text-indigo-700 dark:text-indigo-300",
    dot: "bg-indigo-500",
    raw: "text-indigo-700/65 dark:text-indigo-300/70",
    row: "bg-indigo-50/60 dark:bg-indigo-400/5",
  },
  location: {
    chip: "border-rose-200 bg-rose-50 text-rose-700 dark:border-rose-400/20 dark:bg-rose-400/10 dark:text-rose-300",
    icon: "text-rose-700 dark:text-rose-300",
    dot: "bg-rose-500",
    raw: "text-rose-700/65 dark:text-rose-300/70",
    row: "bg-rose-50/60 dark:bg-rose-400/5",
  },
  time: {
    chip: "border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-400/20 dark:bg-amber-400/10 dark:text-amber-300",
    icon: "text-amber-700 dark:text-amber-300",
    dot: "bg-amber-500",
    raw: "text-amber-700/65 dark:text-amber-300/70",
    row: "bg-amber-50/60 dark:bg-amber-400/5",
  },
  coffee: {
    chip: "border-orange-200 bg-orange-50 text-orange-700 dark:border-orange-400/20 dark:bg-orange-400/10 dark:text-orange-300",
    icon: "text-orange-700 dark:text-orange-300",
    dot: "bg-orange-500",
    raw: "text-orange-700/65 dark:text-orange-300/70",
    row: "bg-orange-50/60 dark:bg-orange-400/5",
  },
  order: {
    chip: "border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-400/20 dark:bg-emerald-400/10 dark:text-emerald-300",
    icon: "text-emerald-700 dark:text-emerald-300",
    dot: "bg-emerald-500",
    raw: "text-emerald-700/65 dark:text-emerald-300/70",
    row: "bg-emerald-50/60 dark:bg-emerald-400/5",
  },
  destructive: {
    chip: "border-destructive/20 bg-destructive/[0.06] text-destructive",
    icon: "text-destructive",
    dot: "bg-destructive",
    raw: "text-destructive/70",
    row: "bg-destructive/[0.035]",
  },
  skill: {
    chip: "border-fuchsia-200 bg-fuchsia-50 text-fuchsia-700 dark:border-fuchsia-400/20 dark:bg-fuchsia-400/10 dark:text-fuchsia-300",
    icon: "text-fuchsia-700 dark:text-fuchsia-300",
    dot: "bg-fuchsia-500",
    raw: "text-fuchsia-700/65 dark:text-fuchsia-300/70",
    row: "bg-fuchsia-50/60 dark:bg-fuchsia-400/5",
  },
  utility: {
    chip: "border-cyan-200 bg-cyan-50 text-cyan-700 dark:border-cyan-400/20 dark:bg-cyan-400/10 dark:text-cyan-300",
    icon: "text-cyan-700 dark:text-cyan-300",
    dot: "bg-cyan-500",
    raw: "text-cyan-700/65 dark:text-cyan-300/70",
    row: "bg-cyan-50/60 dark:bg-cyan-400/5",
  },
  default: {
    chip: "border-border bg-muted/45 text-foreground/75",
    icon: "text-muted-foreground",
    dot: "bg-muted-foreground/60",
    raw: "text-muted-foreground",
    row: "bg-muted/25",
  },
};

const TOOL_TONE_ICON: Record<ToolTone, LucideIcon> = {
  paper: BookOpenText,
  figure: ImageIcon,
  library: FileSearch,
  web: Globe2,
  academic: GraduationCap,
  location: MapPin,
  time: Clock3,
  coffee: Coffee,
  order: PackageCheck,
  destructive: Trash2,
  skill: Puzzle,
  utility: Wrench,
  default: Wrench,
};

function toolStepState(step: PlanStep, live: boolean) {
  if (step.status === "failed") return "failed";
  if (step.status === "running" && live) return "running";
  return "done";
}

function stateLabel(state: string) {
  if (state === "failed") return "失败";
  if (state === "running") return "调用中";
  return "已返回";
}

function ToolChip({ step, live }: { step: PlanStep; live: boolean }) {
  const info = toolDisplayInfo(step.tool || step.text);
  const style = TOOL_TONE_STYLE[info.tone];
  const Icon = TOOL_TONE_ICON[info.tone];
  const state = toolStepState(step, live);
  const running = state === "running";
  const StateIcon = state === "failed" ? CircleAlert : running ? Loader2 : CheckCircle2;

  return (
    <span
      className={cn(
        "inline-flex h-7 max-w-full items-center gap-1.5 rounded-full border px-2.5 text-[11px] font-medium",
        style.chip,
      )}
    >
      <span
        className={cn("size-1.5 shrink-0 rounded-full", style.dot, running && "animate-pulse")}
        aria-hidden
      />
      <Icon className={cn("size-3 shrink-0", style.icon)} aria-hidden />
      <span className="max-w-36 truncate">
        {running ? <Shimmer as="span">{info.name}</Shimmer> : info.name}
      </span>
      <StateIcon className={cn("size-3 shrink-0", running && "animate-spin")} aria-hidden />
    </span>
  );
}

function ToolRow({
  step,
  live,
  index,
}: {
  step: PlanStep;
  live: boolean;
  index: number;
}) {
  const info = toolDisplayInfo(step.tool || step.text);
  const style = TOOL_TONE_STYLE[info.tone];
  const Icon = TOOL_TONE_ICON[info.tone];
  const state = toolStepState(step, live);
  const running = state === "running";
  const StateIcon = state === "failed" ? CircleAlert : running ? Loader2 : CheckCircle2;
  const raw = info.raw !== info.name ? info.raw : "";

  return (
    <li className={cn("flex min-w-0 items-center gap-2 rounded-md px-2.5 py-2", style.row)}>
      <span className="w-4 shrink-0 text-right font-mono text-[10px] text-muted-foreground/65">
        {index + 1}
      </span>
      <Icon className={cn("size-3.5 shrink-0", style.icon)} aria-hidden />
      <span className="min-w-0 flex-1">
        <span className="block truncate text-xs font-medium text-foreground/85">
          {info.name}
        </span>
        {raw && (
          <span className={cn("mt-0.5 block truncate font-mono text-[10px]", style.raw)}>
            {raw}
          </span>
        )}
      </span>
      <span className="inline-flex shrink-0 items-center gap-1 text-[11px] text-muted-foreground">
        <StateIcon className={cn("size-3", running && "animate-spin")} aria-hidden />
        {stateLabel(state)}
      </span>
    </li>
  );
}

export function ToolTrace({
  steps,
  live,
}: {
  steps: PlanStep[];
  live?: boolean;
}) {
  const [open, setOpen] = useState(false);
  const tools = toolPlanSteps(steps);
  if (tools.length === 0) return null;

  return (
    <Collapsible
      open={open}
      onOpenChange={setOpen}
      className="mb-3 overflow-hidden rounded-lg border bg-card/70 shadow-[0_1px_2px_rgba(15,23,42,0.03)]"
    >
      <CollapsibleTrigger
        render={
          <button
            type="button"
            className="flex w-full items-center gap-2 px-3 py-2 text-left text-xs text-foreground/80 transition-[background-color,transform] hover:bg-muted/35 active:scale-[0.99]"
          />
        }
      >
        <Wrench className="size-3.5 shrink-0 text-primary" aria-hidden />
        <span className="min-w-0 flex-1">
          <span className="flex min-w-0 items-center gap-2">
            <span className="shrink-0 font-medium">
              {live ? <Shimmer as="span">工具轨迹</Shimmer> : "工具轨迹"}
            </span>
            <span className="shrink-0 text-muted-foreground">· {tools.length} 次</span>
            <span className="min-w-0 truncate text-muted-foreground">
              {tools.map((step) => toolDisplayInfo(step.tool || step.text).name).join(" / ")}
            </span>
          </span>
        </span>
        <ChevronDown
          className={cn("size-4 shrink-0 text-muted-foreground transition-transform", open && "rotate-180")}
          aria-hidden
        />
      </CollapsibleTrigger>
      <div className="flex flex-wrap gap-1.5 px-3 pb-2">
        {tools.map((step, index) => (
          <ToolChip key={`${step.tool || step.text}-${index}`} step={step} live={!!live} />
        ))}
      </div>
      <CollapsibleContent>
        <ol className="space-y-1.5 border-t px-3 py-2.5">
          {tools.map((step, index) => (
            <ToolRow
              key={`${step.tool || step.text}-${index}`}
              step={step}
              live={!!live}
              index={index}
            />
          ))}
        </ol>
      </CollapsibleContent>
    </Collapsible>
  );
}
