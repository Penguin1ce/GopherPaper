"use client";

import { cn } from "@/lib/utils";

export const AGENT_INTROS = {
  reports: {
    label: "研读与报告",
    desc: "速读、方法、结果、相关研究",
    agent: "小囊鼠",
  },
  graph: {
    label: "论文关系图谱",
    desc: "查看论文、作者、关键词关系",
    agent: "知识库",
  },
  pioneer: {
    label: "学术探索引擎",
    desc: "Ask me anything",
    agent: "小云雀",
  },
} as const;

export type AgentIntroKind = keyof typeof AGENT_INTROS;

export function AgentIntro({
  kind,
  className,
}: {
  kind: AgentIntroKind;
  className?: string;
}) {
  const intro = AGENT_INTROS[kind];

  return (
    <div className={cn("min-w-0", className)}>
      <div className="flex min-w-0 items-center gap-2">
        <div className="truncate text-sm font-semibold leading-5">{intro.label}</div>
        <span className="shrink-0 rounded border border-border/70 px-1.5 py-0.5 text-[10px] leading-none text-muted-foreground">
          {intro.agent}
        </span>
      </div>
      <div className="truncate text-xs leading-4 text-muted-foreground">
        {intro.desc}
      </div>
    </div>
  );
}
