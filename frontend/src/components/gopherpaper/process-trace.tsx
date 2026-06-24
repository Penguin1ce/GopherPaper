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
  planning: "规划",
  replanning: "重新规划",
  action: "检索",
  reasoning: "思考",
};

// ProcessTrace 是论文助教内联的「执行过程」活动条(Claude 网页版式):
// 流式中默认展开、按阶段逐行流出、末行 shimmer 呼吸;答完收成单行可点开回看。
// live 跟随消息 streaming 状态:进行中自动展开,答完自动收起(仍可手动展开)。
export function ProcessTrace({ steps, live }: { steps: PlanStep[]; live?: boolean }) {
  const [open, setOpen] = useState(!!live);
  // live 变化时同步开合:进行中展开,答完收起;两次 live 变化之间用户的手动开合保留。
  useEffect(() => {
    setOpen(!!live);
  }, [live]);

  if (!steps || steps.length === 0) return null;
  const lastIdx = steps.length - 1;

  return (
    <Collapsible
      open={open}
      onOpenChange={setOpen}
      className="mb-3 overflow-hidden rounded-xl border bg-muted/30"
    >
      <CollapsibleTrigger
        render={
          <button
            type="button"
            className="flex w-full items-center gap-2 px-3 py-2 text-left text-xs font-medium text-foreground/80 transition-colors hover:bg-muted/50"
          />
        }
      >
        {live ? (
          <span className="size-2 shrink-0 animate-pulse rounded-full bg-primary" aria-hidden />
        ) : (
          <Check className="size-3.5 shrink-0 text-primary" aria-hidden />
        )}
        <span className="flex-1">
          {live ? (
            <Shimmer as="span">执行过程</Shimmer>
          ) : (
            `执行过程 · ${steps.length} 步`
          )}
        </span>
        <ChevronDown
          className={cn("size-4 shrink-0 text-muted-foreground transition-transform", open && "rotate-180")}
          aria-hidden
        />
      </CollapsibleTrigger>
      <CollapsibleContent>
        <div className="space-y-2 border-t px-3 py-2.5">
          {steps.map((s, i) => (
            <div key={i} className="flex gap-2.5 text-sm leading-6">
              <span className="mt-px w-12 shrink-0 font-medium text-primary">
                {PHASE_LABEL[s.phase] || s.phase}
              </span>
              <span className="min-w-0 flex-1 whitespace-pre-wrap text-foreground/75">
                {live && i === lastIdx ? <Shimmer as="span">{s.text.trim()}</Shimmer> : s.text.trim()}
              </span>
            </div>
          ))}
        </div>
      </CollapsibleContent>
    </Collapsible>
  );
}
