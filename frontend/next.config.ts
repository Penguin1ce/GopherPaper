import type { NextConfig } from "next";

// /api/v1/* 不再走 rewrite(会缓冲 SSE),改由 src/app/api/v1/[...path]/route.ts 流式代理。
const nextConfig: NextConfig = {};

export default nextConfig;
