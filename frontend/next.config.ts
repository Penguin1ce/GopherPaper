import type { NextConfig } from "next";

// /api/v1/* 不再走 rewrite(会缓冲 SSE),改由 src/app/api/v1/[...path]/route.ts 流式代理。
const reactDomClientPatch = "./src/lib/react-dom-client-idempotent.ts";
const turbopackReactDomClientAliases = {
  "react-dom/client": reactDomClientPatch,
  "react-dom/client.js": reactDomClientPatch,
};

const nextConfig: NextConfig = {
  allowedDevOrigins: ["localhost", "127.0.0.1"],
  turbopack: {
    resolveAlias: turbopackReactDomClientAliases,
  },
};

export default nextConfig;
