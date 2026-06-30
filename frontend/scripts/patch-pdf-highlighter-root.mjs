import { readFileSync, writeFileSync } from "node:fs";
import { resolve } from "node:path";

const entry = resolve("node_modules/react-pdf-highlighter-plus/dist/esm/index.js");
const originalImport = 'import { createRoot } from "react-dom/client";';
const patchedImport =
  'import { createRoot } from "../../../../src/lib/react-dom-client-idempotent.ts";';

const source = readFileSync(entry, "utf8");

if (source.includes(patchedImport)) {
  console.log("react-pdf-highlighter-plus createRoot patch already applied");
} else if (source.includes(originalImport)) {
  writeFileSync(entry, source.replace(originalImport, patchedImport));
  console.log("patched react-pdf-highlighter-plus createRoot import");
} else {
  throw new Error(
    "Unable to patch react-pdf-highlighter-plus createRoot import; package layout may have changed.",
  );
}
