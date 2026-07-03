"use client";

import {
  CheckCircle2,
  ChevronDown,
  Circle,
  CircleAlert,
  ClipboardCheck,
  FilePenLine,
  Loader2,
  SearchCheck,
  type LucideIcon,
} from "lucide-react";
import { useMemo, useState } from "react";

import { Shimmer } from "@/components/ai-elements/shimmer";
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/components/ui/collapsible";
import type { PlanStep } from "@/lib/gopherpaper/types";
import { cn } from "@/lib/utils";

type RelayState = "waiting" | "running" | "done" | "failed";

interface StageDef {
  phase: "researching" | "writing" | "reviewing";
  title: string;
  role: string;
  desc: string;
  icon: LucideIcon;
}

interface StageRun extends StageDef {
  state: RelayState;
  steps: PlanStep[];
  detail: string;
}

const STAGES: StageDef[] = [
  {
    phase: "researching",
    title: "研究员",
    role: "找证据",
    desc: "检索正文、图表与出处",
    icon: SearchCheck,
  },
  {
    phase: "writing",
    title: "撰写员",
    role: "写报告",
    desc: "整理成结构化报告",
    icon: FilePenLine,
  },
  {
    phase: "reviewing",
    title: "评审员",
    role: "核事实",
    desc: "核对出处与结论",
    icon: ClipboardCheck,
  },
];

const STATE_LABELS: Record<RelayState, string> = {
  waiting: "待接力",
  running: "进行中",
  done: "完成",
  failed: "失败",
};

const STATE_STYLE: Record<
  RelayState,
  { segment: string; icon: string; text: string; bar: string; status: string }
> = {
  waiting: {
    segment: "bg-muted/20 text-muted-foreground",
    icon: "bg-muted text-muted-foreground ring-border",
    text: "text-muted-foreground",
    bar: "bg-muted",
    status: "text-muted-foreground",
  },
  running: {
    segment: "bg-primary/[0.055] text-foreground ring-1 ring-primary/20",
    icon: "bg-primary/10 text-primary ring-primary/15",
    text: "text-primary",
    bar: "bg-primary",
    status: "text-primary",
  },
  done: {
    segment: "bg-emerald-50/70 text-foreground dark:bg-emerald-400/10",
    icon: "bg-emerald-500/10 text-emerald-700 ring-emerald-500/10 dark:text-emerald-300",
    text: "text-emerald-700 dark:text-emerald-300",
    bar: "bg-emerald-500",
    status: "text-emerald-700 dark:text-emerald-300",
  },
  failed: {
    segment: "bg-destructive/[0.055] text-foreground ring-1 ring-destructive/20",
    icon: "bg-destructive/10 text-destructive ring-destructive/10",
    text: "text-destructive",
    bar: "bg-destructive",
    status: "text-destructive",
  },
};

const PLANNER_PHASE_LABELS: Record<string, string> = {
  thinking: "思考",
  planning: "规划",
  replanning: "调整",
  action: "检索",
  reasoning: "思考",
};

const STAGE_PHASE_INDEX = new Map(STAGES.map((stage, index) => [stage.phase, index]));
const LOW_LEVEL_PHASES = new Set(["thinking", "planning", "replanning", "action", "reasoning"]);

function cleanDetail(text: string) {
  return text
    .trim()
    .replace(/^找资料[:：]\s*/, "")
    .replace(/^写报告[:：]\s*/, "")
    .replace(/^评审[:：]\s*/, "")
    .replace(/\s+/g, " ");
}

function latestDetail(steps: PlanStep[], fallback: string) {
  for (let i = steps.length - 1; i >= 0; i -= 1) {
    const text = cleanDetail(steps[i].text || "");
    if (text) return text;
  }
  return fallback;
}

function splitRelay(steps: PlanStep[], live: boolean, failed: boolean) {
  const grouped = STAGES.map((stage) => ({ ...stage, steps: [] as PlanStep[] }));
  let kickoff = "";
  let current = -1;
  let active = -1;

  for (const step of steps) {
    const phase = step.phase;
    if (phase === "preparing") {
      kickoff = cleanDetail(step.text);
      continue;
    }
    if (phase === "failed") {
      if (current < 0) current = Math.max(active, 0);
      grouped[current].steps.push(step);
      active = current;
      continue;
    }
    const stageIndex = STAGE_PHASE_INDEX.get(phase as StageDef["phase"]);
    if (stageIndex !== undefined) {
      current = stageIndex;
      active = stageIndex;
      grouped[current].steps.push(step);
      continue;
    }
    if (LOW_LEVEL_PHASES.has(phase)) {
      if (current < 0) current = 0;
      active = current;
      grouped[current].steps.push(step);
      continue;
    }
    if (current >= 0) grouped[current].steps.push(step);
  }

  if (active < 0 && live) active = 0;

  const stages: StageRun[] = grouped.map((stage, index) => {
    const hasWork = stage.steps.length > 0;
    let state: RelayState = "waiting";
    if (failed && index === active) state = "failed";
    else if (live && index === active) state = "running";
    else if (index < active || (!live && hasWork)) state = "done";
    else if (!live && active >= 0 && index <= active) state = "done";

    return {
      ...stage,
      state,
      detail: latestDetail(stage.steps, stage.desc),
    };
  });

  return { kickoff, stages, active };
}

function StateIcon({ state }: { state: RelayState }) {
  if (state === "running") return <Loader2 className="size-3 animate-spin" aria-hidden />;
  if (state === "done") return <CheckCircle2 className="size-3" aria-hidden />;
  if (state === "failed") return <CircleAlert className="size-3" aria-hidden />;
  return <Circle className="size-3" aria-hidden />;
}

function StageSegment({ stage }: { stage: StageRun }) {
  const Icon = stage.icon;
  const style = STATE_STYLE[stage.state];
  const progressWidth =
    stage.state === "done" || stage.state === "failed"
      ? "w-full"
      : stage.state === "running"
        ? "w-1/2"
        : "w-0";

  return (
    <section className={cn("min-w-0 rounded-md px-2.5 py-2", style.segment)}>
      <div className="flex min-w-0 items-center gap-2">
        <span
          className={cn("flex size-6 shrink-0 items-center justify-center rounded-md ring-1", style.icon)}
          aria-hidden
        >
          <Icon className="size-3.5" />
        </span>
        <div className="min-w-0 flex-1">
          <div className="flex min-w-0 items-center gap-1.5">
            <span className="truncate text-xs font-semibold text-foreground">
              {stage.title}
            </span>
            <span className={cn("inline-flex shrink-0 items-center gap-1 text-[10px]", style.status)}>
              <StateIcon state={stage.state} />
              {STATE_LABELS[stage.state]}
            </span>
          </div>
          <p className="mt-0.5 truncate text-[11px] text-muted-foreground">{stage.role}</p>
        </div>
      </div>
      <div className="mt-2 h-1 overflow-hidden rounded-full bg-background/70">
        <div
          className={cn(
            "h-full rounded-full transition-[width] duration-200",
            style.bar,
            progressWidth,
            stage.state === "running" && "animate-pulse",
          )}
          aria-hidden
        />
      </div>
    </section>
  );
}

function DetailList({ stages }: { stages: StageRun[] }) {
  return (
    <div className="border-t px-3 py-2.5">
      <ol className="space-y-2">
        {stages.map((stage) => {
          const Icon = stage.icon;
          return (
            <li key={stage.phase} className="grid min-w-0 grid-cols-[88px_1fr] gap-3 text-xs">
              <span className="flex min-w-0 items-center gap-1.5 font-medium text-foreground/80">
                <Icon className="size-3.5 shrink-0 text-primary" aria-hidden />
                {stage.title}
              </span>
              <span className="min-w-0 text-muted-foreground">
                {stage.steps.length > 0
                  ? stage.steps
                      .slice(-3)
                      .map((step) => {
                        const label = PLANNER_PHASE_LABELS[step.phase] || stage.role;
                        const detail = cleanDetail(step.text);
                        return detail ? `${label}: ${detail}` : label;
                      })
                      .join(" / ")
                  : "等待接力"}
              </span>
            </li>
          );
        })}
      </ol>
    </div>
  );
}

export function ReportRelayTrace({
  steps,
  live,
  failed,
}: {
  steps: PlanStep[];
  live?: boolean;
  failed?: boolean;
}) {
  const [open, setOpen] = useState(false);
  const { kickoff, stages, active } = useMemo(
    () => splitRelay(steps, Boolean(live), Boolean(failed)),
    [steps, live, failed],
  );
  const current = stages[Math.max(active, 0)] || stages[0];
  const currentStyle = STATE_STYLE[current.state];

  return (
    <Collapsible
      open={open}
      onOpenChange={setOpen}
      className="overflow-hidden rounded-lg border bg-card/75 shadow-[0_1px_2px_rgba(15,23,42,0.035)]"
    >
      <div className="px-3 py-2.5">
        <div className="flex min-w-0 items-center gap-2">
          <span
            className={cn(
              "flex size-6 shrink-0 items-center justify-center rounded-md ring-1",
              currentStyle.icon,
            )}
            aria-hidden
          >
            <StateIcon state={current.state} />
          </span>
          <div className="min-w-0 flex-1">
            <div className="flex min-w-0 items-center gap-2">
              <h3 className="truncate text-sm font-semibold text-foreground">小囊鼠接力</h3>
              <span className="shrink-0 text-xs text-muted-foreground">
                研究员 / 撰写员 / 评审员
              </span>
            </div>
            <p className="mt-0.5 truncate text-xs text-muted-foreground">
              {kickoff || "三位 agent 正在按顺序处理研读报告。"}
            </p>
          </div>
          <CollapsibleTrigger
            render={
              <button
                type="button"
                className="inline-flex size-7 shrink-0 items-center justify-center rounded-md text-muted-foreground transition-[background-color,transform] hover:bg-muted/50 active:scale-95"
              />
            }
          >
            <ChevronDown
              className={cn("size-4 transition-transform", open && "rotate-180")}
              aria-hidden
            />
            <span className="sr-only">展开接力明细</span>
          </CollapsibleTrigger>
        </div>

        <div className="mt-3 grid gap-1.5 md:grid-cols-3">
          {stages.map((stage) => (
            <StageSegment key={stage.phase} stage={stage} />
          ))}
        </div>

        <p className={cn("mt-2 truncate text-xs", currentStyle.text)}>
          {current.state === "running" ? (
            <Shimmer as="span">{`${current.title}: ${current.detail}`}</Shimmer>
          ) : (
            `${current.title}: ${current.detail}`
          )}
        </p>
      </div>

      <CollapsibleContent>
        <DetailList stages={stages} />
      </CollapsibleContent>
    </Collapsible>
  );
}
