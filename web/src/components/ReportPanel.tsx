import { useEffect, useState } from "react";

import * as api from "../api";
import { useApp } from "../store";
import { paperTitle } from "../utils";
import type { ChatResponse, ReportType } from "../types";
import { Empty, Skeleton, useGuard } from "./ui";

const REPORTS: { type: ReportType; label: string; desc: string }[] = [
  { type: "quickread", label: "论文速读", desc: "一图看懂全文要点" },
  { type: "method", label: "研究方法", desc: "方法流程与设计总结" },
  { type: "result", label: "实验结果", desc: "关键指标与结论" },
  { type: "innovation", label: "创新与不足", desc: "贡献点与局限分析" },
  { type: "compare", label: "同类对比", desc: "与相关工作的异同" },
  { type: "future", label: "未来建议", desc: "可延展的研究方向" },
];

export function ReportPanel() {
  const { activePaper, activePaperID } = useApp();
  const guard = useGuard();
  const [active, setActive] = useState<ReportType | null>(null);
  const [loading, setLoading] = useState(false);
  const [report, setReport] = useState<ChatResponse | null>(null);

  // 切换论文时清空已生成的报告。
  useEffect(() => {
    setActive(null);
    setReport(null);
    setLoading(false);
  }, [activePaperID]);

  const run = (type: ReportType) => {
    if (!activePaperID || loading) return;
    setActive(type);
    setReport(null);
    setLoading(true);
    guard(async () => {
      const res = await api.generateReport(activePaperID, type);
      setReport(res);
    }).finally(() => setLoading(false));
  };

  if (!activePaper) {
    return (
      <div className="report-scroll">
        <Empty title="尚未选择论文" text="在中栏选择一篇论文,再生成研读报告。" />
      </div>
    );
  }

  const ready = activePaper.status === "ready";

  return (
    <div className="report-scroll">
      <div className="report-head">
        <div>
          <p className="eyebrow">研读报告 · {paperTitle(activePaper)}</p>
          <p className="report-hint">
            选择一种报告类型,系统基于结构化抽取与全文检索生成。
          </p>
        </div>
      </div>

      {!ready && (
        <p className="report-warn">
          论文尚未就绪(当前:{activePaper.status}),报告可能不完整,建议解析完成后再生成。
        </p>
      )}

      <div className="report-grid">
        {REPORTS.map((r) => (
          <button
            key={r.type}
            type="button"
            className={`report-card${active === r.type ? " active" : ""}`}
            disabled={loading}
            onClick={() => run(r.type)}
          >
            <strong>{r.label}</strong>
            <span>{r.desc}</span>
          </button>
        ))}
      </div>

      <div className="report-output">
        {loading ? (
          <div className="report-loading">
            <p className="report-loading-text">正在生成报告,稍候…</p>
            <Skeleton lines={5} />
            <Skeleton lines={4} />
          </div>
        ) : report ? (
          <article className="report-article">
            <header className="report-article-head">
              <h3>{REPORTS.find((r) => r.type === active)?.label}</h3>
              {report.intent && (
                <span className="intent-chip">{report.intent}</span>
              )}
            </header>
            <div className="report-content">{report.content}</div>
          </article>
        ) : (
          <Empty
            title="还没有生成报告"
            text="点击上方任一类型,生成对应的研读报告。"
            inline
          />
        )}
      </div>
    </div>
  );
}
