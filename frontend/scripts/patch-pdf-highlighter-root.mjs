import { readFileSync, writeFileSync } from "node:fs";
import { resolve } from "node:path";

const entry = resolve("node_modules/react-pdf-highlighter-plus/dist/esm/index.js");
const originalImport = 'import { createRoot } from "react-dom/client";';
const patchedImport =
  'import { createRoot } from "../../../../src/lib/react-dom-client-idempotent.ts";';
const originalViewerOptions = `      removePageBorders: true,
      linkService: linkServiceRef.current`;
const patchedViewerOptions = `      removePageBorders: true,
      enableAutoLinking: false,
      linkService: linkServiceRef.current`;

let source = readFileSync(entry, "utf8");
let patched = false;

if (source.includes(patchedImport)) {
  console.log("react-pdf-highlighter-plus createRoot patch already applied");
} else if (source.includes(originalImport)) {
  source = source.replace(originalImport, patchedImport);
  patched = true;
  console.log("patched react-pdf-highlighter-plus createRoot import");
} else {
  throw new Error(
    "Unable to patch react-pdf-highlighter-plus createRoot import; package layout may have changed.",
  );
}

if (source.includes(patchedViewerOptions)) {
  console.log("react-pdf-highlighter-plus PDF.js autolink patch already applied");
} else if (source.includes(originalViewerOptions)) {
  source = source.replace(originalViewerOptions, patchedViewerOptions);
  patched = true;
  console.log("patched react-pdf-highlighter-plus PDF.js autolink option");
} else {
  throw new Error(
    "Unable to patch react-pdf-highlighter-plus PDFViewer options; package layout may have changed.",
  );
}

if (patched) {
  writeFileSync(entry, source);
}
