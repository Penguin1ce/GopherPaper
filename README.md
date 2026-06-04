# GopherPaper · 科研文献智能解析与知识服务系统

基于 [eino](https://github.com/cloudwego/eino) 编排的多智能体论文解析与问答系统:论文上传 → MinerU 解析 → 结构化抽取 → 知识库构建 → 多轮问答(带页码出处)→ 研读报告生成。

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

| 方法 | 路径                            | 说明                                    |
| ---- | ------------------------------- | --------------------------------------- |
| POST | `/api/v1/papers`                | 上传 PDF(multipart `file`),触发异步解析 |
| GET  | `/api/v1/papers`                | 论文列表                                |
| GET  | `/api/v1/papers/search?q=`      | 历史文献检索                            |
| GET  | `/api/v1/papers/:id`            | 论文详情与结构化元信息                  |
| GET  | `/api/v1/papers/:id/status`     | 解析状态兜底查询                        |
| POST | `/api/v1/papers/:id/report`     | 生成研读报告(body `type`)               |
| GET  | `/api/v1/ws?token=<jwt>`        | WebSocket 订阅解析进度                  |
| POST | `/api/v1/sessions`              | 新建会话(可绑定 `paper_id`)             |
| POST | `/api/v1/sessions/:id/messages` | 多轮论文问答                            |
