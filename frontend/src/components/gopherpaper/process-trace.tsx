"use client";

import { Check, ChevronDown } from "lucide-react";
import { useEffect, useState } from "react";

import { Shimmer } from "@/components/ai-elements/shimmer";
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/components/ui/collapsible";
import type { PlanStep } from "@/lib/gopherpaper/types";
import { cn } from "@/lib/utils";

// 后端 react planner 的阶段名 → 论文助教面向用户的中文标签。action 在 RAG 语境即检索。
const PHASE_LABEL: Record<string, string> = {
  preparing: "准备",
  researching: "找资料",
  writing: "写报告",
  reviewing: "评审",
  planning: "规划",
  replanning: "重新规划",
  action: "检索",
  reasoning: "思考",
  failed: "失败",
};

const PHASE_STYLE: Record<string, { dot: string; title: string }> = {
  preparing: { dot: "bg-primary", title: "text-primary" },
  researching: { dot: "bg-primary", title: "text-primary" },
  writing: { dot: "bg-primary", title: "text-primary" },
  reviewing: { dot: "bg-muted-foreground", title: "text-foreground/70" },
  planning: { dot: "bg-primary", title: "text-primary" },
  replanning: { dot: "bg-primary", title: "text-primary" },
  action: { dot: "bg-sienna", title: "text-sienna" },
  reasoning: { dot: "bg-muted-foreground", title: "text-foreground/70" },
  failed: { dot: "bg-destructive", title: "text-destructive" },
};
const PHASE_STYLE_FALLBACK = PHASE_STYLE.reasoning;

function phaseLabel(phase: string, overrides?: Record<string, string>) {
  return overrides?.[phase] || PHASE_LABEL[phase] || phase;
}

function compactText(text: string) {
  return text.trim().replace(/\s+/g, " ");
}

function ProcessStep({
  step,
  isLast,
  live,
  phaseLabels,
}: {
  step: PlanStep;
  isLast: boolean;
  live: boolean;
  phaseLabels?: Record<string, string>;
}) {
  const ps = PHASE_STYLE[step.phase] || PHASE_STYLE_FALLBACK;
  const label = phaseLabel(step.phase, phaseLabels);
  const text = step.text.trim();
  const [open, setOpen] = useState(live);

  useEffect(() => {
    if (live) setOpen(true);
  }, [live]);

  return (
    <li className="relative pb-3 pl-5 last:pb-0">
      {!isLast && (
        <span className="absolute bottom-0 left-[3px] top-3 w-px bg-border" aria-hidden />
      )}
      <span
        className={cn("absolute left-0 top-[5px] size-1.5 rounded-full ring-3 ring-background", ps.dot)}
        aria-hidden
      />
      <button
        type="button"
        onClick={() => setOpen((o) => !o)}
        className="group flex w-full items-center gap-1.5 text-left"
      >
        <span className={cn("text-xs font-semibold", ps.title)}>
          {live ? <Shimmer as="span">{label}</Shimmer> : label}
        </span>
        <ChevronDown
          className={cn(
            "size-3 shrink-0 text-muted-foreground/50 transition-transform group-hover:text-muted-foreground",
            open && "rotate-180",
          )}
          aria-hidden
        />
      </button>
      {open ? (
        <p className="mt-1 whitespace-pre-wrap text-xs leading-5 text-muted-foreground">
          {live ? <Shimmer as="span">{text || "处理中..."}</Shimmer> : text}
        </p>
      ) : (
        <p className="mt-0.5 line-clamp-1 text-xs text-muted-foreground/60">
          {compactText(text)}
        </p>
      )}
    </li>
  );
}

// ProcessTrace 是论文助教内联的「执行过程」活动条。
// 默认只露出当前阶段摘要,展开后呈现统一的精简时间线。
export function ProcessTrace({
  steps,
  live,
  phaseLabels,
}: {
  steps: PlanStep[];
  live?: boolean;
  phaseLabels?: Record<string, string>;
}) {
  const [open, setOpen] = useState(Boolean(live));
  useEffect(() => {
    if (live) setOpen(true);
  }, [live]);

  if (!steps || steps.length === 0) return null;
  const lastIdx = steps.length - 1;
  const active = steps[lastIdx];
  const activeLabel = phaseLabel(active.phase, phaseLabels);
  const activeText = compactText(active.text);

  return (
    <Collapsible
      open={open}
      onOpenChange={setOpen}
      className="mb-3 overflow-hidden rounded-lg border bg-muted/20"
    >
      <CollapsibleTrigger
        render={
          <button
            type="button"
            className="flex w-full items-center gap-2 px-3 py-2 text-left text-xs text-foreground/80 transition-colors hover:bg-muted/45"
          />
        }
      >
        {live ? (
          <span className="size-2 shrink-0 animate-pulse rounded-full bg-primary" aria-hidden />
        ) : (
          <Check className="size-3.5 shrink-0 text-primary" aria-hidden />
        )}
        <span className="min-w-0 flex-1">
          <span className="flex min-w-0 items-center gap-2">
            <span className="shrink-0 font-medium">
              {live ? <Shimmer as="span">执行过程</Shimmer> : "执行过程"}
            </span>
            <span className="shrink-0 text-muted-foreground">· {steps.length} 步</span>
            <span className="min-w-0 truncate font-medium text-foreground/70">
              {activeLabel}
            </span>
          </span>
          {activeText && (
            <span className="mt-0.5 block truncate text-[11px] font-normal text-muted-foreground">
              {activeText}
            </span>
          )}
        </span>
        <ChevronDown
          className={cn("size-4 shrink-0 text-muted-foreground transition-transform", open && "rotate-180")}
          aria-hidden
        />
      </CollapsibleTrigger>
      <CollapsibleContent>
        <ol className="relative border-t px-3 py-2.5">
          {steps.map((s, i) => (
            <ProcessStep
              key={i}
              step={s}
              isLast={i === lastIdx}
              live={!!live && i === lastIdx}
              phaseLabels={phaseLabels}
            />
          ))}
        </ol>
      </CollapsibleContent>
    </Collapsible>
  );
}
