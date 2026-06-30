import type { NextConfig } from "next";
import path from "node:path";

// /api/v1/* 不再走 rewrite(会缓冲 SSE),改由 src/app/api/v1/[...path]/route.ts 流式代理。
const reactDomClientPatch = "./src/lib/react-dom-client-idempotent.ts";

const nextConfig: NextConfig = {
  allowedDevOrigins: ["localhost", "127.0.0.1"],
  turbopack: {
    resolveAlias: {
      "react-dom/client": reactDomClientPatch,
    },
  },
  webpack(config) {
    config.resolve ??= {};
    config.resolve.alias ??= {};
    config.resolve.alias["react-dom/client$"] = path.resolve(__dirname, reactDomClientPatch);
    return config;
  },
};

export default nextConfig;
