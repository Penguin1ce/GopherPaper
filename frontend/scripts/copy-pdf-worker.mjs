// 把 pdf.js 的 worker 从 node_modules 拷到 public/pdfjs/,供 react-pdf 以
// /pdfjs/pdf.worker.min.mjs 静态路径加载。dev/build 前自动执行(见 package.json
// 的 predev/prebuild),故 worker 始终与已装的 pdfjs-dist 版本一致、无需提交进仓库。
// 用 Node 内置 fs 而非 shell cp,Windows/macOS/Linux 行为一致。
import { copyFileSync, mkdirSync } from "node:fs";
import { dirname, resolve } from "node:path";

const src = resolve("node_modules/pdfjs-dist/build/pdf.worker.min.mjs");
const dest = resolve("public/pdfjs/pdf.worker.min.mjs");

mkdirSync(dirname(dest), { recursive: true });
copyFileSync(src, dest);
console.log("copied pdf.worker ->", dest);
