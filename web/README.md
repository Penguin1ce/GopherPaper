# GopherPaper 前端

Vite + React 18 + TypeScript。构建产物输出到 `../internal/web/public`,由 Go 经
`//go:embed public` 内嵌进单二进制。该目录里的 hash 产物不入库,构建 Go 前先跑前端构建。

## 开发

```bash
cd web
npm install
npm run dev          # http://localhost:5173, /api 与 /ws 代理到 :8080 后端
```

先把后端跑起来(默认 `:8080`),再开 `npm run dev` 即可热更联调。

## 构建

```bash
npm run build        # tsc 类型检查 + vite 打包 -> ../internal/web/public
```

`../internal/web/public` 只保留 `.gitkeep` 占位,实际 `index.html`、`reader.html` 与
`static/assets/*` 都由上面的命令生成。干净 checkout 后若直接跑 Go 服务,页面资源不会存在。

产物布局(与 Go 的路由约定对齐):

| 文件 | Go 出口 |
| --- | --- |
| `public/index.html` | `GET /` 直出页面壳 |
| `public/reader.html` | `GET /reader` 精读页页面壳 |
| `public/static/assets/*` | `StaticFS("/static")` 服务,带内容哈希 |

改完前端记得重新 `npm run build`,再重新编译 Go,内嵌的就是新产物。

## 结构

```text
src/
  api.ts          类型化 API 客户端 + WebSocket(统一 code/message/data 信封)
  store.tsx       全局状态与动作(Context),含 WS 进度推送与轮询兜底
  types.ts        与后端 model/dto 对齐的类型
  utils.ts        格式化与状态机辅助
  components/
    AuthView      登录 / 注册 / 邮箱验证码
    Sidebar       用户卡 + 历史会话
    PaperPane     上传 + 解析进度 + 论文列表/检索
    RightPane     右栏 tab:问答 / 论文详情 / 研读报告
    ChatPane      多轮问答 + 引用出处渲染
    PaperDetail   GET /papers/:id 结构化元信息 + 章节大纲
    ReportPanel   POST /papers/:id/report 六类研读报告
```
