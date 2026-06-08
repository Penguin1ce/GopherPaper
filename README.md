# GopherPaper · 科研文献智能解析与知识服务系统

基于 [trpc-agent-go](https://github.com/trpc-group/trpc-agent-go) 编排的多智能体论文解析与问答系统:论文上传 → MinerU 解析 → 结构化抽取 → 知识库构建 → 多轮问答(带页码出处与召回配图)→ 研读报告生成。

## 启动

```bash
# 1. 基础设施
docker compose -f deploy/docker-compose.yml up -d
#   Milvus 可视化 Attu: http://localhost:8000
#   RabbitMQ 管理台:    http://localhost:15672 (gopher/gopher)

# 2. 本地 ollama 拉 embedding 模型
ollama pull bge-m3

# 3. 配置：填入大模型 API key、MinerU API token、SMTP 授权码等
cp config/config.example.toml config/config.toml
```

## 核心接口

受保护接口走 `Authorization: Bearer <jwt>`;浏览器直连的 WebSocket 与取图接口带不了头,鉴权改走 `?token=<jwt>`。

| 方法   | 路径                                  | 说明                                       |
| ------ | ------------------------------------- | ------------------------------------------ |
| POST   | `/api/v1/user/register`               | 注册(邮箱验证码)                           |
| POST   | `/api/v1/user/login`                  | 登录签发 JWT                               |
| POST   | `/api/v1/papers`                      | 上传 PDF(multipart `file`),触发异步解析    |
| GET    | `/api/v1/papers`                      | 论文列表                                   |
| GET    | `/api/v1/papers/search?q=`            | 历史文献检索                               |
| GET    | `/api/v1/papers/:id`                  | 论文详情与结构化元信息                     |
| GET    | `/api/v1/papers/:id/status`           | 解析状态兜底查询                           |
| GET    | `/api/v1/papers/:id/figures/:name?token=<jwt>` | 取问答召回引用的图片(query token 鉴权) |
| POST   | `/api/v1/papers/:id/report`           | 生成研读报告(body `type`)                  |
| GET    | `/api/v1/ws?token=<jwt>`              | WebSocket 订阅解析进度                     |
| POST   | `/api/v1/sessions`                    | 新建会话(可绑定 `paper_id`)                |
| GET    | `/api/v1/sessions`                    | 会话列表                                   |
| DELETE | `/api/v1/sessions/:id`                | 删除会话                                   |
| GET    | `/api/v1/sessions/:id/messages`       | 拉取历史消息(含出处与内联图)               |
| POST   | `/api/v1/sessions/:id/messages`       | 多轮论文问答                               |
