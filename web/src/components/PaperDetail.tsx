import { useEffect, useState } from "react";

import * as api from "../api";
import { useApp } from "../store";
import { paperTitle } from "../utils";
import type { PaperDetail as Detail } from "../types";
import { Empty, Skeleton } from "./ui";

function TagList({ items }: { items?: string[] | null }) {
  if (!items || items.length === 0) return <p className="detail-empty">—</p>;
  return (
    <div className="tag-list">
      {items.map((t, i) => (
        <span key={i} className="tag">
          {t}
        </span>
      ))}
    </div>
  );
}

function Bullets({ items }: { items?: string[] | null }) {
  if (!items || items.length === 0) return <p className="detail-empty">—</p>;
  return (
    <ul className="detail-bullets">
      {items.map((t, i) => (
        <li key={i}>{t}</li>
      ))}
    </ul>
  );
}

function Prose({ text }: { text?: string }) {
  if (!text || !text.trim()) return <p className="detail-empty">—</p>;
  return <p className="detail-prose">{text}</p>;
}

export function PaperDetail() {
  const { activePaper, activePaperID } = useApp();
  const [detail, setDetail] = useState<Detail | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!activePaperID) {
      setDetail(null);
      return;
    }
    let cancelled = false;
    setLoading(true);
    setError("");
    setDetail(null);
    api
      .paperDetail(activePaperID)
      .then((d) => {
        if (!cancelled) setDetail(d);
      })
      .catch((e) => {
        if (!cancelled) setError((e as Error)?.message || "加载失败");
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [activePaperID]);

  if (!activePaper) {
    return (
      <div className="detail-scroll">
        <Empty title="尚未选择论文" text="在中栏选择一篇论文,查看其结构化详情。" />
      </div>
    );
  }

  if (loading) {
    return (
      <div className="detail-scroll">
        <Skeleton lines={4} />
        <Skeleton lines={5} />
      </div>
    );
  }

  if (error) {
    return (
      <div className="detail-scroll">
        <Empty title="无法加载详情" text={error} />
      </div>
    );
  }

  const meta = detail?.meta;
  const sections = detail?.sections ?? [];

  return (
    <div className="detail-scroll">
      <header className="detail-head">
        <h2 className="detail-title">{paperTitle(activePaper)}</h2>
        <TagList items={meta?.authors} />
      </header>

      {!meta ? (
        <Empty
          title="结构化信息尚未就绪"
          text="论文解析与抽取完成后,这里会展示摘要、方法、结果等结构化字段。"
          inline
        />
      ) : (
        <div className="detail-grid">
          <section className="detail-block span-2">
            <h3>摘要</h3>
            <Prose text={meta.abstract} />
          </section>

          <section className="detail-block">
            <h3>关键词</h3>
            <TagList items={meta.keywords} />
          </section>
          <section className="detail-block">
            <h3>研究机构</h3>
            <TagList items={meta.affiliations} />
          </section>

          <section className="detail-block span-2">
            <h3>研究问题</h3>
            <Bullets items={meta.research_questions} />
          </section>

          <section className="detail-block">
            <h3>研究方法</h3>
            <Prose text={meta.methods} />
          </section>
          <section className="detail-block">
            <h3>实验设置</h3>
            <Prose text={meta.experiments} />
          </section>

          <section className="detail-block span-2">
            <h3>实验结果</h3>
            <Prose text={meta.results} />
          </section>

          <section className="detail-block">
            <h3>创新点</h3>
            <Bullets items={meta.innovations} />
          </section>
          <section className="detail-block">
            <h3>不足</h3>
            <Bullets items={meta.limitations} />
          </section>

          <section className="detail-block span-2">
            <h3>未来工作</h3>
            <Bullets items={meta.future_work} />
          </section>
        </div>
      )}

      {sections.length > 0 && (
        <section className="detail-block outline">
          <h3>章节大纲</h3>
          <ul className="outline-list">
            {sections.map((s) => (
              <li
                key={s.id}
                className="outline-item"
                style={{ paddingLeft: `${Math.max(0, s.level - 1) * 16}px` }}
              >
                <span className="outline-title">{s.title || "未命名章节"}</span>
                {s.page_no > 0 && (
                  <span className="outline-page">p.{s.page_no}</span>
                )}
              </li>
            ))}
          </ul>
        </section>
      )}
    </div>
  );
}
