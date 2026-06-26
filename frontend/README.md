# GopherPaper Frontend

Next.js + shadcn/ui 独立前端。Go 服务只提供 `/api/v1/*` 与 `/healthz`。

## 开发

```bash
npm install
npm run dev
```

默认访问 `http://localhost:3000`。开发期 `/api/v1/*` 由
`next.config.ts` 代理到 `http://127.0.0.1:8080`。

如后端地址不同:

```bash
GOPHERPAPER_API_ORIGIN=http://127.0.0.1:8081 npm run dev
```

WebSocket 如需绕过 Next 代理,可设置:

```bash
NEXT_PUBLIC_WS_BASE=ws://127.0.0.1:8080 npm run dev
```

## 构建

```bash
npm run build
npm run start
```
