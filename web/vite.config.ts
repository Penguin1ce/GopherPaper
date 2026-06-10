import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// 构建产物直接落进 Go 的 embed 目录 internal/web/public:
//   index.html        -> public/index.html      (Go 在 / 处直出)
//   static/assets/*    -> public/static/assets/* (Go StaticFS 挂在 /static)
// 因此 base 用 /, 资源文件名前缀 static/assets, 让 HTML 引用 /static/assets/...
export default defineConfig({
  plugins: [react()],
  base: "/",
  build: {
    outDir: "../internal/web/public",
    emptyOutDir: true,
    assetsDir: "static/assets",
    rollupOptions: {
      // 多页:主应用 index.html 与独立精读页 reader.html 各一个入口,
      // pdf.js 等重依赖只进 reader 的 bundle,不拖累主应用。
      input: {
        main: "index.html",
        reader: "reader.html",
      },
      output: {
        entryFileNames: "static/assets/[name]-[hash].js",
        chunkFileNames: "static/assets/[name]-[hash].js",
        assetFileNames: "static/assets/[name]-[hash][extname]",
      },
    },
  },
  server: {
    port: 5173,
    proxy: {
      "/api": {
        target: "http://localhost:8080",
        changeOrigin: true,
        ws: true,
      },
    },
  },
});
