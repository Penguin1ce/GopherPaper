import type { NextConfig } from "next";
import path from "node:path";

// /api/v1/* 不再走 rewrite(会缓冲 SSE),改由 src/app/api/v1/[...path]/route.ts 流式代理。
const reactDomClientPatch = "./src/lib/react-dom-client-idempotent.ts";
const reactDomClientPatchPath = path.resolve(
  __dirname,
  reactDomClientPatch,
);
const turbopackReactDomClientAliases = {
  "react-dom/client": reactDomClientPatch,
  "react-dom/client.js": reactDomClientPatch,
};
const webpackReactDomClientAliases = {
  "react-dom/client": reactDomClientPatchPath,
  "react-dom/client.js": reactDomClientPatchPath,
  "react-dom/client$": reactDomClientPatchPath,
  "react-dom/client.js$": reactDomClientPatchPath,
};

const nextConfig: NextConfig = {
  allowedDevOrigins: ["localhost", "127.0.0.1"],
  turbopack: {
    resolveAlias: turbopackReactDomClientAliases,
  },
  webpack(config) {
    config.resolve ??= {};
    config.resolve.alias ??= {};
    Object.assign(config.resolve.alias, webpackReactDomClientAliases);
    return config;
  },
};

export default nextConfig;
