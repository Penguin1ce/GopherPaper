import { useRef, useState } from "react";

import { useApp } from "../store";
import {
  PARSE_STEPS,
  formatSize,
  formatTime,
  paperTitle,
  statusStep,
} from "../utils";
import type { Paper } from "../types";
import { Empty, StatusBadge, useGuard } from "./ui";

// 解析进度条:按状态机步进高亮,失败置红。
function ParseProgress({ paper }: { paper: Paper }) {
  const step = statusStep(paper.status);
  const failed = paper.status === "failed";
  return (
    <div className={`parse-progress${failed ? " failed" : ""}`}>
      {PARSE_STEPS.map((label, i) => (
        <div
          key={label}
          className={`parse-node${i <= step && !failed ? " done" : ""}${
            i === step && !failed ? " current" : ""
          }`}
        >
          <span className="parse-dot" />
          <span className="parse-label">{label}</span>
        </div>
      ))}
    </div>
  );
}

export function PaperPane() {
  const {
    papers,
    activePaper,
    activePaperID,
    selectPaper,
    uploadPaper,
    refreshPapers,
  } = useApp();
  const guard = useGuard();
  const fileRef = useRef<HTMLInputElement>(null);
  const [picked, setPicked] = useState<File | null>(null);
  const [uploading, setUploading] = useState(false);
  const [query, setQuery] = useState("");

  const onUpload = (e: React.FormEvent) => {
    e.preventDefault();
    if (!picked) return;
    setUploading(true);
    guard(async () => {
      await uploadPaper(picked);
      setPicked(null);
      if (fileRef.current) fileRef.current.value = "";
    }).finally(() => setUploading(false));
  };

  return (
    <section className="paper-pane">
      <div className="panel-header">
        <div>
          <p className="eyebrow">Library</p>
          <h2>论文上传与选择</h2>
        </div>
        <button
          type="button"
          className="secondary-button compact"
          onClick={() => guard(() => refreshPapers())}
        >
          刷新
        </button>
      </div>

      <form className="upload-box" onSubmit={onUpload}>
        <label
          className={`file-drop${picked ? " has-file" : ""}`}
          onDragOver={(e) => e.preventDefault()}
          onDrop={(e) => {
            e.preventDefault();
            const f = e.dataTransfer.files?.[0];
            if (f) setPicked(f);
          }}
        >
          <input
            ref={fileRef}
            type="file"
            accept="application/pdf,.pdf"
            onChange={(e) => setPicked(e.target.files?.[0] ?? null)}
          />
          <span className="file-icon" aria-hidden>
            PDF
          </span>
          <strong>{picked ? picked.name : "选择 / 拖入一篇 PDF 论文"}</strong>
          <small>
            {picked
              ? `${formatSize(picked.size)} · 等待上传`
              : "支持 50MB 以内的 PDF 文件"}
          </small>
        </label>
        <button
          type="submit"
          className="primary-button"
          disabled={uploading || !picked}
        >
          {uploading ? "上传中…" : "上传"}
        </button>
      </form>

      {activePaper && (
        <div className="active-paper">
          <div className="active-paper-head">
            <p className="eyebrow">当前论文</p>
            <StatusBadge status={activePaper.status} />
          </div>
          <h3>{paperTitle(activePaper)}</h3>
          <div className="muted-line">
            {[
              formatSize(activePaper.size),
              activePaper.page_count ? `${activePaper.page_count} 页` : "",
              formatTime(activePaper.updated_at || activePaper.created_at),
            ]
              .filter(Boolean)
              .join(" · ")}
          </div>
          {activePaper.status === "failed" && activePaper.fail_reason ? (
            <p className="fail-line">解析失败:{activePaper.fail_reason}</p>
          ) : (
            <ParseProgress paper={activePaper} />
          )}
        </div>
      )}

      <div className="section-heading paper-heading">
        <h3>我的论文</h3>
        <form
          className="search-form"
          onSubmit={(e) => {
            e.preventDefault();
            guard(() => refreshPapers(query));
          }}
        >
          <input
            value={query}
            placeholder="搜索标题或文件名"
            onChange={(e) => setQuery(e.target.value)}
          />
          <button type="submit" className="secondary-button compact">
            搜索
          </button>
        </form>
      </div>

      <div className="paper-list">
        {papers.length === 0 ? (
          <Empty title="还没有论文" text="上传 PDF 后会出现在这里。" inline />
        ) : (
          papers.map((p) => (
            <button
              key={p.id}
              type="button"
              className={`paper-item${p.id === activePaperID ? " active" : ""}`}
              onClick={() => selectPaper(p.id)}
            >
              <div className="paper-item-top">
                <strong>{paperTitle(p)}</strong>
                <StatusBadge status={p.status} />
              </div>
              <p>
                {[
                  p.file_name,
                  formatSize(p.size),
                  formatTime(p.updated_at || p.created_at),
                ]
                  .filter(Boolean)
                  .join(" · ")}
              </p>
            </button>
          ))
        )}
      </div>
    </section>
  );
}
