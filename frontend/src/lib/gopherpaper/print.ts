// 浏览器端把一段已渲染的报告 HTML 导出成 PDF:写进隐藏 iframe 再调浏览器打印,
// 用户在打印对话框「另存为 PDF」。隔离 iframe 避免应用样式与打印样式互相干扰,长报告也能正常分页。
// 不引第三方库;中文/分页交给浏览器,语义标签由下面的打印样式负责排版。

const PRINT_CSS = `
  @page { margin: 18mm 16mm; }
  * { box-sizing: border-box; }
  body { margin: 0; }
  .report {
    font-family: "Songti SC", "Noto Serif CJK SC", "Source Han Serif SC", Georgia, serif;
    color: #1a1a1a;
    font-size: 12pt;
    line-height: 1.7;
    max-width: 100%;
  }
  .report h1 { font-size: 20pt; margin: 0 0 12pt; }
  .report h2 { font-size: 15pt; margin: 18pt 0 8pt; border-bottom: 1px solid #ddd; padding-bottom: 4pt; }
  .report h3 { font-size: 13pt; margin: 14pt 0 6pt; }
  .report h4 { font-size: 12pt; margin: 12pt 0 6pt; }
  .report p { margin: 0 0 8pt; }
  .report ul, .report ol { margin: 0 0 8pt; padding-left: 20pt; }
  .report li { margin: 0 0 4pt; }
  .report blockquote { margin: 0 0 8pt; padding-left: 12pt; border-left: 3px solid #ccc; color: #555; }
  .report code { font-family: "SF Mono", Menlo, Consolas, monospace; font-size: 10.5pt; background: #f4f4f4; padding: 1pt 3pt; border-radius: 3px; }
  .report pre { background: #f6f6f6; padding: 8pt; border-radius: 4px; overflow-x: auto; page-break-inside: avoid; }
  .report pre code { background: none; padding: 0; }
  .report table { border-collapse: collapse; width: 100%; margin: 0 0 10pt; font-size: 10.5pt; page-break-inside: avoid; }
  .report th, .report td { border: 1px solid #ccc; padding: 4pt 6pt; text-align: left; }
  .report th { background: #f4f4f4; }
  .report img { max-width: 100%; height: auto; page-break-inside: avoid; }
  .report a { color: #1a1a1a; text-decoration: underline; }
  .report h1, .report h2, .report h3, .report h4 { page-break-after: avoid; }
`;

function escapeHTML(s: string): string {
  return s.replace(/[&<>"]/g, (c) =>
    ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" })[c] as string,
  );
}

// printReport 把报告标题与正文 HTML 写进隐藏 iframe 并触发打印;打印结束后清理 iframe。
export function printReport(title: string, bodyHTML: string): void {
  if (typeof document === "undefined") return;
  const iframe = document.createElement("iframe");
  iframe.setAttribute("aria-hidden", "true");
  iframe.style.cssText = "position:fixed;right:0;bottom:0;width:0;height:0;border:0;";
  document.body.appendChild(iframe);

  const win = iframe.contentWindow;
  const doc = win?.document;
  if (!win || !doc) {
    iframe.remove();
    return;
  }

  doc.open();
  doc.write(
    `<!doctype html><html><head><meta charset="utf-8"><title>${escapeHTML(title)}</title>` +
      `<style>${PRINT_CSS}</style></head>` +
      `<body><h1 class="report" style="margin-bottom:14pt">${escapeHTML(title)}</h1>` +
      `<main class="report">${bodyHTML}</main></body></html>`,
  );
  doc.close();

  // 浏览器「另存为 PDF」的默认文件名取顶层页面的 document.title,不是 iframe 的 title;
  // 故打印前临时把顶层标题改成本报告标题,让每类报告导出不同文件名,打印后再还原。
  const prevTitle = document.title;
  let cleaned = false;
  const cleanup = () => {
    if (cleaned) return;
    cleaned = true;
    document.title = prevTitle;
    iframe.remove();
  };
  win.onafterprint = cleanup;
  // 给布局与图片一拍时间再打印;onafterprint 不被部分浏览器触发时兜底延时清理。
  setTimeout(() => {
    document.title = title;
    win.focus();
    win.print();
    setTimeout(cleanup, 60000);
  }, 200);
}
