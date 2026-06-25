"use client";

import { ChevronDown, Sparkles } from "lucide-react";
import { useEffect, useRef, useState } from "react";

import { Badge } from "@/components/ui/badge";
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/components/ui/collapsible";
import * as api from "@/lib/gopherpaper/api";
import { useApp } from "@/lib/gopherpaper/store";
import type { PaperMeta, PaperSection } from "@/lib/gopherpaper/types";
import { cn } from "@/lib/utils";
import { SkeletonLines } from "./app-ui";

// PaperOverview 是小囊鼠报告区顶部的「论文速览」卡:展示入库前切窗抽取的结构化字段
// (摘要/关键词/方法/实验/结果/创新点/局限/未来工作 与章节目录),数据取 paperDetail 的 meta。
// 抽取在解析流水线 extracted 阶段落库,故论文 ready 前 meta 可能为空,此时整卡不渲染。
export function PaperOverview() {
  const { activePaperID } = useApp();
  const [meta, setMeta] = useState<PaperMeta | null>(null);
  const [sections, setSections] = useState<PaperSection[]>([]);
  const [loading, setLoading] = useState(false);
  const [open, setOpen] = useState(true);
  // 当前论文 ID 镜像:异步结果回来校验仍是这篇才落地,切走后不覆盖。
  const reqRef = useRef(activePaperID);

  useEffect(() => {
    reqRef.current = activePaperID;
    if (!activePaperID) {
      setMeta(null);
      setSections([]);
      return;
    }
    setLoading(true);
    api
      .paperDetail(activePaperID)
      .then((d) => {
        if (reqRef.current !== activePaperID) return;
        setMeta(d.meta);
        setSections(d.sections ?? []);
      })
      .catch(() => {
        if (reqRef.current !== activePaperID) return;
        setMeta(null);
        setSections([]);
      })
      .finally(() => {
        if (reqRef.current === activePaperID) setLoading(false);
      });
  }, [activePaperID]);

  if (loading) {
    return (
      <div className="rounded-xl border bg-card p-4">
        <SkeletonLines lines={4} />
      </div>
    );
  }
  // meta 为空(尚未抽取或抽取失败)不展示速览,交由报告卡区域提示状态。
  if (!meta) return null;

  return (
    <Collapsible open={open} onOpenChange={setOpen} className="overflow-hidden rounded-xl border bg-card">
      <CollapsibleTrigger
        render={
          <button
            type="button"
            className="flex w-full items-center gap-2 px-4 py-3 text-left transition-colors hover:bg-accent/40"
          />
        }
      >
        <span className="flex size-7 shrink-0 items-center justify-center rounded-md border border-sienna/30 bg-sienna/10 text-sienna">
          <Sparkles className="size-3.5" />
        </span>
        <span className="min-w-0 flex-1">
          <span className="block font-serif text-sm font-semibold">论文速览</span>
          <span className="block truncate text-xs text-muted-foreground">
            解析时抽取的结构化要点
          </span>
        </span>
        <ChevronDown
          className={cn(
            "size-4 shrink-0 text-muted-foreground transition-transform",
            open && "rotate-180",
          )}
        />
      </CollapsibleTrigger>
      <CollapsibleContent>
        <div className="space-y-4 border-t px-4 py-4">
          <PeopleRow label="作者" items={meta.authors} />
          <PeopleRow label="机构" items={meta.affiliations} />
          <TextField label="摘要" value={meta.abstract} />
          <Tags label="关键词" items={meta.keywords} />
          <ListField label="研究问题" items={meta.research_questions} />
          <TextField label="方法" value={meta.methods} />
          <TextField label="实验" value={meta.experiments} />
          <TextField label="结果" value={meta.results} />
          <ListField label="创新点" items={meta.innovations} />
          <ListField label="局限" items={meta.limitations} />
          <ListField label="未来工作" items={meta.future_work} />
          <Toc sections={sections} />
        </div>
      </CollapsibleContent>
    </Collapsible>
  );
}

// FieldLabel 是各小节统一的标题样式。
function FieldLabel({ children }: { children: React.ReactNode }) {
  return (
    <h4 className="mb-1.5 font-serif text-xs font-semibold uppercase tracking-wide text-muted-foreground">
      {children}
    </h4>
  );
}

// TextField 渲染单段文本字段(方法/实验/结果/摘要),空则不渲染。
function TextField({ label, value }: { label: string; value?: string | null }) {
  if (!value || !value.trim()) return null;
  return (
    <section>
      <FieldLabel>{label}</FieldLabel>
      <p className="whitespace-pre-wrap text-sm leading-relaxed text-foreground/90">{value}</p>
    </section>
  );
}

// ListField 渲染字符串数组字段(创新点/局限/未来工作/研究问题),空则不渲染。
function ListField({ label, items }: { label: string; items?: string[] | null }) {
  const list = (items ?? []).filter((s) => s && s.trim());
  if (list.length === 0) return null;
  return (
    <section>
      <FieldLabel>{label}</FieldLabel>
      <ul className="space-y-1">
        {list.map((s, i) => (
          <li key={i} className="flex gap-2 text-sm leading-relaxed text-foreground/90">
            <span className="mt-1.5 size-1 shrink-0 rounded-full bg-sienna/60" />
            <span>{s}</span>
          </li>
        ))}
      </ul>
    </section>
  );
}

// Tags 把关键词渲染为徽章组。
function Tags({ label, items }: { label: string; items?: string[] | null }) {
  const list = (items ?? []).filter((s) => s && s.trim());
  if (list.length === 0) return null;
  return (
    <section>
      <FieldLabel>{label}</FieldLabel>
      <div className="flex flex-wrap gap-1.5">
        {list.map((s, i) => (
          <Badge key={i} variant="secondary" className="rounded-full font-normal">
            {s}
          </Badge>
        ))}
      </div>
    </section>
  );
}

// PeopleRow 渲染作者/机构等顿号分隔的行内列表。
function PeopleRow({ label, items }: { label: string; items?: string[] | null }) {
  const list = (items ?? []).filter((s) => s && s.trim());
  if (list.length === 0) return null;
  return (
    <section>
      <FieldLabel>{label}</FieldLabel>
      <p className="text-sm leading-relaxed text-foreground/90">{list.join("、")}</p>
    </section>
  );
}

// Toc 按层级缩进渲染章节目录,带页码。
function Toc({ sections }: { sections: PaperSection[] }) {
  if (sections.length === 0) return null;
  const ordered = [...sections].sort((a, b) => a.order_idx - b.order_idx);
  return (
    <section>
      <FieldLabel>章节目录</FieldLabel>
      <ul className="space-y-0.5">
        {ordered.map((s) => (
          <li
            key={s.id}
            className="flex items-baseline justify-between gap-2 text-sm text-foreground/80"
            style={{ paddingLeft: `${Math.max(0, (s.level - 1)) * 12}px` }}
          >
            <span className="truncate">{s.title}</span>
            {s.page_no > 0 && (
              <span className="shrink-0 text-xs tabular-nums text-muted-foreground">
                p.{s.page_no}
              </span>
            )}
          </li>
        ))}
      </ul>
    </section>
  );
}
