// /api/v1/* 流式透传代理。
// 迁到 Next 后 next.config 的 rewrite 会把 SSE 整条缓冲(先锋者计划/正文憋到最后才出),
// 这里改用 route handler 把上游响应体(ReadableStream)原样流式返回,逐块下发不缓冲。
// 同源、dev/prod 通用,无需后端 CORS。

import type { NextRequest } from "next/server";

export const dynamic = "force-dynamic";
export const runtime = "nodejs";

const API_ORIGIN = process.env.GOPHERPAPER_API_ORIGIN ?? "http://127.0.0.1:8080";

// 透传响应时需剥离的逐跳/会破坏流的头,其余(含 content-type/cache-control)原样带回。
const STRIP_RESPONSE_HEADERS = new Set([
  "content-encoding",
  "content-length",
  "transfer-encoding",
  "connection",
]);

async function proxy(req: NextRequest, path: string[]): Promise<Response> {
  const target = `${API_ORIGIN}/api/v1/${path.map(encodeURIComponent).join("/")}${req.nextUrl.search}`;

  const headers = new Headers(req.headers);
  headers.delete("host");

  const hasBody = req.method !== "GET" && req.method !== "HEAD";
  const init: RequestInit & { duplex?: "half" } = {
    method: req.method,
    headers,
    redirect: "manual",
    // 流式转发请求体(上传大文件也不整块进内存);带 body 必须声明 duplex
    body: hasBody ? req.body : undefined,
    duplex: hasBody ? "half" : undefined,
  };

  const upstream = await fetch(target, init);

  const respHeaders = new Headers();
  upstream.headers.forEach((value, key) => {
    if (!STRIP_RESPONSE_HEADERS.has(key.toLowerCase())) respHeaders.set(key, value);
  });
  // 兜底关掉中间层缓冲(如有 nginx),保证 SSE 实时
  respHeaders.set("X-Accel-Buffering", "no");

  return new Response(upstream.body, {
    status: upstream.status,
    statusText: upstream.statusText,
    headers: respHeaders,
  });
}

type Ctx = { params: Promise<{ path: string[] }> };

const handler = async (req: NextRequest, { params }: Ctx) => {
  const { path } = await params;
  return proxy(req, path);
};

export {
  handler as GET,
  handler as POST,
  handler as PUT,
  handler as PATCH,
  handler as DELETE,
  handler as OPTIONS,
  handler as HEAD,
};
