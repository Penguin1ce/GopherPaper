# GopherPaper · 科研文献智能解析与知识服务系统

<p align="center">
  <img src="doc/产品.png" alt="GopherPaper 产品图标" width="460">
</p>

基于 [trpc-agent-go](https://github.com/trpc-group/trpc-agent-go) 编排的多智能体论文解析与问答系统:论文上传 → MinerU 解析 → 结构化抽取 → 知识库构建 → 多轮问答(带页码出处与召回配图)→ 研读报告生成。

## 启动

```bash
# 1. 基础设施
docker compose -f deploy/docker-compose.yml up -d
#   Milvus 可视化 Attu: http://localhost:8000
#   RabbitMQ 管理台:    http://localhost:15672 (gopher/gopher)
# 2. 配置：填入大模型 API key、MinerU API token、SMTP 授权码等
cp config/config.example.toml config/config.toml

# 3. 启动前端后端
cd web && npm run build && cd .. && go run cmd/server/main.go
```

具体细节请参考这篇博客[世界的尽头](https://muzimi.org/zh/docs/projects/GopherPaper/Windows%E7%8E%AF%E5%A2%83%E9%85%8D%E7%BD%AE)

## 核心接口

受保护接口走 `Authorization: Bearer <jwt>`;浏览器直连的 WebSocket、图片与 PDF 文件接口带不了头,鉴权改走 `?token=<jwt>`。

| 鉴权 | 方法 | 路径 | 说明 |
| --- | --- | --- | --- |
| 公开 | POST | `/api/v1/auth/token` | 按 `student_id`、`class_id` 签发调试 JWT |
| 公开 | POST | `/api/v1/user/send-code` | 下发邮箱验证码,body `email` |
| 公开 | POST | `/api/v1/user/register` | 校验验证码并注册,body `student_id`、`name`、`email`、`class_id`、`password`、`code` |
| 公开 | POST | `/api/v1/user/login` | 学号密码登录,返回 JWT 与用户信息 |
| Query Token | GET | `/api/v1/ws?token=<jwt>` | WebSocket 订阅论文解析进度 |
| Query Token | GET | `/api/v1/papers/:id/figures/:name?token=<jwt>` | 取问答召回引用的图片 |
| Query Token | GET | `/api/v1/papers/:id/file?token=<jwt>` | 取原始 PDF,供精读页 pdf.js 渲染 |
| JWT | POST | `/api/v1/user/logout` | 注销当前登录,清服务端登录态与用户模型缓存 |
| JWT | POST | `/api/v1/papers` | 上传 PDF,multipart `file`,上限 50MB,触发异步解析 |
| JWT | GET | `/api/v1/papers` | 列出当前用户论文 |
| JWT | GET | `/api/v1/papers/search?q=` | 历史文献检索 |
| JWT | GET | `/api/v1/papers/:id` | 论文详情、结构化元信息与章节大纲 |
| JWT | GET | `/api/v1/papers/:id/status` | 查询解析状态,作为 WebSocket 断线兜底 |
| JWT | POST | `/api/v1/papers/:id/report` | 生成或读取研读报告,body `type`: `quickread`、`method`、`result`、`innovation`、`compare`、`future` |
| JWT | POST | `/api/v1/papers/:id/translate` | 精读页选段翻译,body `text`,单次最多 4000 字符 |
| JWT | POST | `/api/v1/sessions` | 新建会话,body 可带 `paper_id`、`title`、`agent_type` |
| JWT | GET | `/api/v1/sessions` | 列出当前用户会话 |
| JWT | DELETE | `/api/v1/sessions/:id` | 删除会话 |
| JWT | GET | `/api/v1/sessions/:id/messages` | 拉取历史消息,含出处与内联图元数据 |
| JWT | POST | `/api/v1/sessions/:id/messages` | 会话内发消息,body `query`,SSE 返回 `plan`、`tool_call`、`tool_result`、`delta`、`done`、`error` 事件 |
