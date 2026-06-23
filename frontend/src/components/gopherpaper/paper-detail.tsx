"use client";

import { ExternalLink } from "lucide-react";
import { useEffect, useState } from "react";

import { Badge } from "@/components/ui/badge";
import { buttonVariants } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { ScrollArea } from "@/components/ui/scroll-area";
import * as api from "@/lib/gopherpaper/api";
import { useApp } from "@/lib/gopherpaper/store";
import type { PaperDetail as Detail } from "@/lib/gopherpaper/types";
import { paperTitle } from "@/lib/gopherpaper/utils";
import { Empty, SkeletonLines } from "./app-ui";

function TextBlock({ title, children }: { title: string; children?: React.ReactNode }) {
  return (
    <Card>
      <CardHeader className="pb-2">
        <CardTitle className="font-serif text-sm">{title}</CardTitle>
      </CardHeader>
      <CardContent className="text-sm leading-6 text-muted-foreground">
        {children || "—"}
      </CardContent>
    </Card>
  );
}

// 短词(作者/关键词)用气泡;整句条目用项目符号,避免把长句塞进气泡显得拥挤。
function ListItems({ items }: { items?: string[] | null }) {
  if (!items || items.length === 0) return <>—</>;
  return (
    <div className="flex flex-wrap gap-2">
      {items.map((item, i) => (
        <Badge key={`${item}-${i}`} variant="secondary" className="rounded-full font-normal">
          {item}
        </Badge>
      ))}
    </div>
  );
}

function Bullets({ items }: { items?: string[] | null }) {
  if (!items || items.length === 0) return <>—</>;
  return (
    <ul className="space-y-1.5">
      {items.map((item, i) => (
        <li key={`${item}-${i}`} className="flex gap-2.5">
          <span className="mt-[0.5em] size-1 shrink-0 rounded-full bg-sienna/60" />
          <span className="min-w-0">{item}</span>
        </li>
      ))}
    </ul>
  );
}

export function PaperDetail() {
  const { activePaper } = useApp();
  const [detail, setDetail] = useState<Detail | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!activePaper?.id) {
      setDetail(null);
      return;
    }
    let cancelled = false;
    setLoading(true);
    setError("");
    api
      .paperDetail(activePaper.id)
      .then((res) => {
        if (!cancelled) setDetail(res);
      })
      .catch((err) => {
        if (!cancelled) setError((err as Error)?.message || "加载失败");
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [activePaper?.id]);

  if (!activePaper) {
    return (
      <div className="p-4">
        <Empty title="尚未选择论文" text="在左侧选择一篇论文后查看详情。" />
      </div>
    );
  }

  return (
    <ScrollArea className="h-full">
      <div className="space-y-4 p-4">
        <div className="flex items-start justify-between gap-3">
          <div className="min-w-0">
            <h2 className="text-lg font-semibold tracking-tight">{paperTitle(activePaper)}</h2>
            <p className="mt-1 text-sm text-muted-foreground">{activePaper.file_name}</p>
          </div>
          <a
            className={buttonVariants({ variant: "secondary" })}
            href={`/reader?id=${encodeURIComponent(activePaper.id)}`}
            target="_blank"
            rel="noreferrer"
          >
            精读
            <ExternalLink className="size-4" />
          </a>
        </div>
        {loading ? (
          <div className="grid gap-4 lg:grid-cols-2">
            <Card><CardContent className="p-6"><SkeletonLines lines={6} /></CardContent></Card>
            <Card><CardContent className="p-6"><SkeletonLines lines={6} /></CardContent></Card>
          </div>
        ) : error ? (
          <Empty title="加载失败" text={error} />
        ) : (
          <>
            <div className="grid gap-4 lg:grid-cols-2">
              <TextBlock title="作者">
                <ListItems items={detail?.meta?.authors} />
              </TextBlock>
              <TextBlock title="关键词">
                <ListItems items={detail?.meta?.keywords} />
              </TextBlock>
              <TextBlock title="摘要">{detail?.meta?.abstract || "—"}</TextBlock>
              <TextBlock title="研究问题">
                <Bullets items={detail?.meta?.research_questions} />
              </TextBlock>
              <TextBlock title="方法">{detail?.meta?.methods || "—"}</TextBlock>
              <TextBlock title="实验">{detail?.meta?.experiments || "—"}</TextBlock>
              <TextBlock title="结果">{detail?.meta?.results || "—"}</TextBlock>
              <TextBlock title="创新点">
                <Bullets items={detail?.meta?.innovations} />
              </TextBlock>
              <TextBlock title="局限">
                <Bullets items={detail?.meta?.limitations} />
              </TextBlock>
              <TextBlock title="未来工作">
                <Bullets items={detail?.meta?.future_work} />
              </TextBlock>
            </div>
            <Card>
              <CardHeader>
                <CardTitle className="font-serif text-sm">章节大纲</CardTitle>
              </CardHeader>
              <CardContent>
                {!detail?.sections || detail.sections.length === 0 ? (
                  <p className="text-sm text-muted-foreground">暂无章节信息</p>
                ) : (
                  <ol className="space-y-2">
                    {detail.sections.map((s) => (
                      <li key={s.id} className="flex items-center gap-3 rounded-lg border p-3 text-sm">
                        <span className="w-10 shrink-0 font-mono text-xs text-muted-foreground">
                          L{s.level}
                        </span>
                        <span className="min-w-0 flex-1 truncate">{s.title || "未命名章节"}</span>
                        {s.page_no > 0 && (
                          <Badge variant="outline" className="rounded-full font-normal">
                            p.{s.page_no}
                          </Badge>
                        )}
                      </li>
                    ))}
                  </ol>
                )}
              </CardContent>
            </Card>
          </>
        )}
      </div>
    </ScrollArea>
  );
}
