import type { Reference } from "./types";

const SOURCE_TAG_RE = /\[\[(原文|出处|来源)(?:[:：]\s*([^\]\n]{1,240}))?\]\]/g;
const OPEN_SOURCE_TAG_RE = /\[\[(原文|出处|来源)(?:[:：]\s*([^\]\n]{1,240}))?(?=\n|$)/g;
const SOURCE_LINK_RE = /\[[^\]\n]{1,240}\]\(source:\/\/([^)]+)\)/g;

function sourcePageNo(value: string): number | null {
  const match = value.match(/第\s*(\d+)\s*页|p\.?\s*(\d+)/i);
  const raw = match?.[1] || match?.[2] || "";
  const page = Number.parseInt(raw, 10);
  return Number.isFinite(page) && page > 0 ? page : null;
}

function sourceText(ref: Reference): string {
  return [ref.source_file, ref.source_uri].filter(Boolean).join(" ");
}

function fileName(value?: string): string {
  return (value || "").split(/[\\/]/).filter(Boolean).pop() || "";
}

function refMatchesLabel(ref: Reference, label: string): boolean {
  return [ref.source_file, fileName(ref.source_file), ref.source_uri, fileName(ref.source_uri)]
    .filter((part): part is string => Boolean(part))
    .some((part) => label.includes(part));
}

function sourceTagLabel(kind: string, detail?: string): string {
  return [kind, (detail || "").trim()].filter(Boolean).join(" · ");
}

function readerHref(docID: string, pageNo: number): string {
  return `/reader?id=${encodeURIComponent(docID)}&page=${encodeURIComponent(String(pageNo))}`;
}

function decodeSourceURL(value: string): string {
  try {
    return decodeURIComponent(value);
  } catch {
    return value;
  }
}

function sourceRefKey(ref: Reference): string {
  return [
    ref.id,
    ref.doc_id,
    ref.source_file,
    ref.source_uri,
    ref.page_no,
    ref.chunk_index,
    ref.block_type,
    ref.img_name,
  ]
    .filter((part) => part !== undefined && part !== null && part !== "")
    .join("|");
}

function sourceTagLabels(markdown: string): string[] {
  const labels: string[] = [];
  const add = (label: string) => {
    const text = label.trim();
    if (text) labels.push(text);
  };
  for (const re of [SOURCE_TAG_RE, OPEN_SOURCE_TAG_RE]) {
    re.lastIndex = 0;
    for (let match = re.exec(markdown); match; match = re.exec(markdown)) {
      add(sourceTagLabel(match[1] || "", match[2]));
    }
  }
  SOURCE_LINK_RE.lastIndex = 0;
  for (let match = SOURCE_LINK_RE.exec(markdown); match; match = SOURCE_LINK_RE.exec(markdown)) {
    add(decodeSourceURL(match[1] || ""));
  }
  return labels;
}

export function sourceReferenceForLabel(
  label: string,
  refs: Reference[],
): Reference | null {
  const pageNo = sourcePageNo(label);
  if (!pageNo) return null;

  const candidates = refs.filter((ref) => ref.page_no === pageNo);
  const named = candidates.find((ref) => {
    const text = sourceText(ref);
    return text && refMatchesLabel(ref, label);
  });
  return named || candidates.find((item) => item.doc_id) || candidates[0] || null;
}

export function referenceReaderHref(
  ref: Reference,
  fallbackDocID?: string,
  allowedDocIDs?: Set<string>,
): string | null {
  const pageNo = typeof ref.page_no === "number" && ref.page_no > 0 ? ref.page_no : null;
  if (!pageNo) return null;
  const docID = ref.doc_id || fallbackDocID || "";
  if (!docID || (allowedDocIDs && !allowedDocIDs.has(docID))) return null;
  return readerHref(docID, pageNo);
}

export function sourceTagReaderHref(
  label: string,
  refs: Reference[],
  fallbackDocID?: string,
  allowedDocIDs?: Set<string>,
): string | null {
  const pageNo = sourcePageNo(label);
  if (!pageNo) return null;

  const ref = sourceReferenceForLabel(label, refs);
  if (ref) return referenceReaderHref(ref, fallbackDocID, allowedDocIDs);
  if (!fallbackDocID || (allowedDocIDs && !allowedDocIDs.has(fallbackDocID))) return null;
  return readerHref(fallbackDocID, pageNo);
}

export function referencesUsedBySourceTags(markdown: string, refs: Reference[]): Reference[] {
  if (!markdown || refs.length === 0) return [];
  const out: Reference[] = [];
  const seen = new Set<string>();
  for (const label of sourceTagLabels(markdown)) {
    const ref = sourceReferenceForLabel(label, refs);
    if (!ref) continue;
    const key = sourceRefKey(ref);
    if (seen.has(key)) continue;
    seen.add(key);
    out.push(ref);
  }
  return out;
}
