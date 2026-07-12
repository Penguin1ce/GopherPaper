# GopherPaper 部署文档

## 1. 文档说明

本文档用于说明 GopherPaper 科研文献智能解析与知识服务系统的部署方式、配置项、启动顺序、验证方法和运维注意事项。当前项目推荐采用“基础设施 Docker Compose + Go 后端 + Next.js 前端 + Nginx 统一入口”的单机部署方式，适用于本地开发、课程演示和小规模测试环境。

生产环境可以继续沿用本文档的启动顺序，但需要额外替换默认密码、接入 HTTPS、配置进程守护与备份策略。

## 2. 系统组成

GopherPaper 的主链路为：

```text
论文上传 -> MinerU 解析 -> 结构化抽取 -> 知识库构建 -> 多轮问答 -> 研读报告生成
```

核心服务如下：

| 模块 | 技术/服务 | 说明 |
| --- | --- | --- |
| 前端 | Next.js + shadcn/ui | 独立前端应用，默认监听 `3000` |
| 后端 | Go + Gin + trpc-agent-go | 提供 `/api/v1/*` 和 `/healthz`，默认监听 `8080` |
| 反向代理 | Nginx | 统一入口，默认监听 `8081` |
| 业务数据库 | MySQL 8.0 | 用户、论文、会话元数据、报告缓存 |
| 缓存 | Redis 7 | 会话缓存、登录态、限流和工作记忆 |
| 消息队列 | RabbitMQ | 论文解析与报告生成异步任务 |
| 向量库 | Milvus standalone | 论文 chunk 向量检索 |
| 对象存储 | MinIO | Milvus standalone 依赖 |
| 元数据存储 | etcd | Milvus standalone 依赖 |
| 知识图谱 | Neo4j 5 | 论文关系图谱与趋势分析 |
| 外部服务 | MinerU、模型 API、SMTP、学术检索 API | PDF 解析、模型推理、邮件验证码和工具调用 |

请求链路：

```text
浏览器
  -> Nginx :8081
      -> Next.js 前端 :3000
      -> Go 后端 :8080
          -> MySQL / Redis / RabbitMQ / Milvus / Neo4j
          -> MinerU / 大模型 API / SMTP / 学术检索 API
```

## 3. 环境要求

建议部署环境：

| 项目 | 要求 |
| --- | --- |
| 操作系统 | Linux、macOS 或 Windows + Docker Desktop |
| Docker | 支持 Docker Compose v2 |
| Go | 版本以 `go.mod` 为准，当前为 `1.26.3` |
| Node.js | 建议 Node.js 20 及以上 |
| npm | 使用项目内 `package-lock.json`，推荐 `npm ci` |
| 内存 | 建议 16GB 及以上，Milvus、Neo4j 和模型调用并发较高时建议更高 |
| 磁盘 | 视论文数量而定，需为 `data/`、MySQL、Milvus、MinIO、Neo4j 预留空间 |

外部账号和密钥：

| 配置 | 用途 | 是否必须 |
| --- | --- | --- |
| 大模型 API Key | 意图分类、问答、抽取、报告、视觉理解 | 必须 |
| Embedding API Key | 论文向量化与检索 | 必须 |
| Rerank API Key | RAG 精排，关闭后可退化为纯向量召回 | 可选 |
| MinerU API Token | PDF 在线解析 | 必须 |
| SMTP 授权码 | 注册、找回密码、验证码邮件 | 必须 |
| Tavily / Semantic Scholar / OpenAlex / SciVerse Key | 小云雀开放式学术检索工具 | 可选 |
| 百度地图 AK/SK | 地理编码工具 | 可选 |

## 4. 端口规划

`deploy/docker-compose.yml` 默认暴露以下端口：

| 端口 | 服务 | 说明 |
| --- | --- | --- |
| `8081` | Nginx | 统一入口，浏览器推荐访问此端口 |
| `8080` | Go 后端 | 后端 API、Swagger、健康检查 |
| `3000` | Next.js | 前端服务 |
| `3306` | MySQL | 业务数据库 |
| `6379` | Redis | 缓存 |
| `5772` | RabbitMQ AMQP | 后端连接 MQ 使用 |
| `15672` | RabbitMQ 管理台 | 默认账号 `gopher/gopher` |
| `19530` | Milvus gRPC | 后端连接向量库使用 |
| `9091` | Milvus 健康/指标 | 健康检查 |
| `8000` | Attu | Milvus 可视化管理界面 |
| `9000` | MinIO API | Milvus 依赖 |
| `9001` | MinIO Console | MinIO 控制台 |
| `7474` | Neo4j Browser | 图数据库浏览器 |
| `7687` | Neo4j Bolt | 后端连接 Neo4j 使用 |

如服务器已有端口占用，需要同步修改 `deploy/docker-compose.yml`、`deploy/nginx/conf.d/gopherpaper.conf` 和 `config/config.toml`。

## 5. 部署步骤

### 5.1 获取代码

```bash
git clone <项目仓库地址> GopherPaper
cd GopherPaper
```

如果已经有代码仓库，直接进入项目根目录：

```bash
cd /path/to/GopherPaper
```

### 5.2 准备配置文件

复制示例配置：

```bash
cp config/config.example.toml config/config.toml
```

重点修改以下配置：

```toml
[server]
addr = ":8080"
mode = "release"

[models.intent]
api_key = "替换为真实 key"

[models.chat]
api_key = "替换为真实 key"

[models.vlm]
api_key = "替换为真实 key"

[embedding]
api_key = "替换为真实 key"
dim = 1024

[rerank]
enabled = true
api_key = "替换为真实 key"

[parser]
token = "替换为 MinerU API token"

[jwt]
secret = "替换为强随机密钥"

[admin]
registration_code = "替换为管理员邀请码"

[mail]
server_mail = "发件邮箱"
smtp_host = "SMTP 服务地址"
smtp_port = 465
key = "SMTP 授权码"
```

基础设施若使用默认 Compose 配置，下面几项可以保持示例值：

```toml
[mysql]
dsn = "gopher:gopher@tcp(localhost:3306)/gopherpaper?charset=utf8mb4&parseTime=True&loc=Local"

[redis]
addr = "localhost:6379"

[mq]
url = "amqp://gopher:gopher@localhost:5772/"

[milvus]
address = "localhost:19530"

[neo4j]
uri = "bolt://localhost:7687"
username = "neo4j"
password = "gopherpaper"
```

注意：`config/config.toml` 已被 `.gitignore` 忽略，不要提交真实密钥。

### 5.3 启动基础设施

```bash
docker compose -f deploy/docker-compose.yml up -d
```

查看容器状态：

```bash
docker compose -f deploy/docker-compose.yml ps
```

建议等待 MySQL、Redis、RabbitMQ、Milvus、Neo4j 均为 healthy 后再启动后端。

### 5.4 启动后端

开发或演示环境可直接运行：

```bash
go run cmd/server/main.go -c config/config.toml
```

部署环境建议先构建二进制：

```bash
go build -o server ./cmd/server
./server -c config/config.toml
```

后端启动时会按固定顺序初始化：

```text
配置 -> 日志 -> MySQL -> AutoMigrate -> history
     -> Redis -> Neo4j -> RabbitMQ -> 模型工厂 -> 工具集
     -> Milvus -> MinerU Parser -> AI 编排器
     -> 论文解析 worker -> JWT/邮件 -> HTTP 路由
```

### 5.5 启动前端

进入前端目录：

```bash
cd frontend
npm ci
npm run build
npm run start -- -H 0.0.0.0 -p 3000
```

开发环境也可以使用：

```bash
cd frontend
npm install
npm run dev
```

前端默认通过 `GOPHERPAPER_API_ORIGIN` 将 `/api/v1/*` 代理到后端。后端使用默认端口时无需额外设置；如果后端地址变化，可按需指定：

```bash
GOPHERPAPER_API_ORIGIN=http://127.0.0.1:8080 npm run start -- -H 0.0.0.0 -p 3000
```

### 5.6 通过 Nginx 访问

Compose 中的 Nginx 默认监听 `8081`，并代理：

| 路径 | 转发目标 |
| --- | --- |
| `/` | Next.js 前端 `host.docker.internal:3000` |
| `/api/v1/*` | Go 后端 `host.docker.internal:8080` |
| `/swagger`、`/swagger/*` | Go 后端 |
| `/healthz` | Go 后端 |

浏览器访问：

```text
http://localhost:8081
```

Swagger 地址：

```text
http://localhost:8081/swagger
http://localhost:8080/swagger
```

如果要使用正式域名和 HTTPS，建议在生产 Nginx 或云负载均衡上终止 TLS，再转发到 `8081`，或直接改造 `deploy/nginx/conf.d/gopherpaper.conf` 增加证书配置。

## 6. 进程守护建议

当前仓库没有内置 systemd、pm2 或 Dockerfile。生产/演示服务器建议使用进程管理工具托管 Go 后端和 Next.js 前端。

### 6.1 后端 systemd 示例

示例文件：`/etc/systemd/system/gopherpaper-backend.service`

```ini
[Unit]
Description=GopherPaper Backend
After=network.target docker.service

[Service]
Type=simple
WorkingDirectory=/opt/GopherPaper
ExecStart=/opt/GopherPaper/server -c /opt/GopherPaper/config/config.toml
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

启用：

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now gopherpaper-backend
sudo systemctl status gopherpaper-backend
```

### 6.2 前端 systemd 示例

示例文件：`/etc/systemd/system/gopherpaper-frontend.service`

```ini
[Unit]
Description=GopherPaper Frontend
After=network.target

[Service]
Type=simple
WorkingDirectory=/opt/GopherPaper/frontend
Environment=GOPHERPAPER_API_ORIGIN=http://127.0.0.1:8080
ExecStart=/usr/bin/npm run start -- -H 0.0.0.0 -p 3000
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

启用：

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now gopherpaper-frontend
sudo systemctl status gopherpaper-frontend
```

## 7. 部署验证

### 7.1 基础设施验证

```bash
docker compose -f deploy/docker-compose.yml ps
```

检查关键服务：

```bash
curl http://127.0.0.1:8081/nginx-health
curl http://127.0.0.1:9091/healthz
```

可视化控制台：

| 地址 | 说明 |
| --- | --- |
| `http://localhost:15672` | RabbitMQ 管理台，默认 `gopher/gopher` |
| `http://localhost:8000` | Milvus Attu |
| `http://localhost:9001` | MinIO Console |
| `http://localhost:7474` | Neo4j Browser，默认 `neo4j/gopherpaper` |

### 7.2 后端验证

```bash
curl http://127.0.0.1:8080/healthz
curl http://127.0.0.1:8081/healthz
```

预期返回包含：

```json
{"data":{"status":"ok"}}
```

Swagger：

```text
http://127.0.0.1:8080/swagger
http://127.0.0.1:8081/swagger
```

### 7.3 前端验证

访问：

```text
http://127.0.0.1:3000
http://127.0.0.1:8081
```

验证流程：

1. 打开首页。
2. 注册或登录用户。
3. 上传 50MB 以内的 PDF。
4. 观察论文状态从 `uploaded`、`parsing`、`extracted`、`indexed` 到 `ready`。
5. 进入论文精读页，发起问答，确认回答包含出处和页码。
6. 生成研读报告，确认报告能保存并重复读取。

## 8. 数据目录与备份

需要关注的数据包括两类：Docker volume 和项目本地目录。

Docker volume：

| Volume | 内容 |
| --- | --- |
| `mysql_data` | MySQL 业务数据 |
| `redis_data` | Redis 数据 |
| `rabbitmq_data` | RabbitMQ 队列数据 |
| `etcd_data` | Milvus 元数据 |
| `minio_data` | Milvus 对象数据 |
| `milvus_data` | Milvus 本地数据 |
| `neo4j_data` | Neo4j 图谱数据 |

项目本地目录：

| 路径 | 内容 |
| --- | --- |
| `data/papers` | 上传 PDF、MinerU 归档、解析图片 |
| `data/avatars` | 用户头像 |
| `log/` | 后端日志 |
| `config/config.toml` | 私有配置和密钥 |

MySQL 备份示例：

```bash
mkdir -p backup
docker exec gopher-mysql sh -c 'mysqldump -uroot -proot gopherpaper' > backup/gopherpaper-mysql.sql
```

本地文件备份示例：

```bash
tar -czf backup/gopherpaper-data.tar.gz data config/config.toml
```

Milvus 和 Neo4j 数据建议在停机或低流量窗口备份对应 volume，避免运行中快照不一致。

## 9. 更新与回滚

### 9.1 更新代码

```bash
git pull
```

后端：

```bash
go build -o server ./cmd/server
sudo systemctl restart gopherpaper-backend
```

前端：

```bash
cd frontend
npm ci
npm run build
sudo systemctl restart gopherpaper-frontend
```

基础设施配置有变化时：

```bash
docker compose -f deploy/docker-compose.yml pull
docker compose -f deploy/docker-compose.yml up -d
```

### 9.2 回滚

1. 切回上一版本代码。
2. 重新构建后端和前端。
3. 重启后端、前端和必要的 Docker 服务。
4. 如果涉及数据库结构或 Milvus schema 变更，需要同时恢复对应备份。

## 10. 运行维护

常用命令：

```bash
docker compose -f deploy/docker-compose.yml ps
docker compose -f deploy/docker-compose.yml logs -f
docker compose -f deploy/docker-compose.yml restart
docker compose -f deploy/docker-compose.yml down
```

查看后端日志：

```bash
tail -f log/app-$(date +%F).log
```

如果使用 systemd：

```bash
journalctl -u gopherpaper-backend -f
journalctl -u gopherpaper-frontend -f
```

重置本地测试数据前务必确认环境。项目提供了重置工具，默认会清理 MySQL、Redis、Milvus、Neo4j 和本地论文文件：

```bash
go run cmd/resetdata/main.go -c config/config.toml
go run cmd/resetdata/main.go -c config/config.toml --yes
```

第一条为 dry run，第二条才会实际执行。

## 11. 安全配置

上线或对外演示前建议完成以下检查：

1. 替换 MySQL、RabbitMQ、Neo4j、MinIO 的默认密码，并同步修改 `config/config.toml`。
2. 替换 `[jwt].secret` 为强随机密钥。
3. 替换 `[admin].registration_code`，不要使用示例值。
4. 不提交 `config/config.toml`、日志、上传文件和任何 API Key。
5. 对外访问时启用 HTTPS。
6. 限制 MySQL、Redis、RabbitMQ、Milvus、MinIO、Neo4j 管理端口的公网访问。
7. 将 `[server].mode` 设置为 `release`。
8. 定期备份 `data/`、MySQL、Milvus 和 Neo4j。

## 12. 常见问题

### 12.1 后端启动失败，提示连接 MySQL/Redis/RabbitMQ 失败

先检查容器是否 healthy：

```bash
docker compose -f deploy/docker-compose.yml ps
```

再确认 `config/config.toml` 中端口、账号、密码与 Compose 配置一致。

### 12.2 Milvus 初始化失败或向量维度不一致

`[embedding].dim` 必须与 Milvus collection 维度一致。若已经创建过 collection 后又更换 embedding 模型或维度，需要删除旧 collection 或迁移数据后重新建库。

### 12.3 论文上传失败

系统上传 PDF 上限为 50MB，Nginx 当前 `client_max_body_size` 为 80MB。若文件过大，需要同时修改后端限制和 Nginx 限制。另需确认 `data/papers` 有写入权限。

### 12.4 论文长期停留在 parsing

重点检查：

1. MinerU token 是否有效。
2. 外网是否可以访问 MinerU API。
3. RabbitMQ 队列是否有堆积。
4. 后端 worker 是否仍在运行。
5. 后端日志中是否有 MinerU 轮询超时或解析失败信息。

### 12.5 问答没有引用来源

重点检查：

1. 论文状态是否已经到 `ready`。
2. Milvus 是否写入成功。
3. `[embedding]` 配置是否可用。
4. 当前会话是否绑定论文。
5. 检索过滤是否能命中当前用户可见的 private/public 数据。

### 12.6 SSE 进度或聊天流式输出不连续

项目 Nginx 已对 `/api/v1/events` 和 `/api/v1/sessions/:id/messages` 关闭代理缓冲。如果使用额外的反向代理或云网关，需要同样关闭 SSE 路径的响应缓冲，并延长读写超时时间。

### 12.7 Nginx 访问不到宿主机前后端

Compose 内 Nginx 通过 `host.docker.internal` 访问宿主机 `3000` 和 `8080`。Linux 环境需要 Docker 支持 `host-gateway`；如果不可用，可将 `deploy/nginx/conf.d/gopherpaper.conf` 中 upstream 地址改为宿主机实际 IP。

## 13. 停止服务

停止前端和后端进程后，再停止基础设施：

```bash
sudo systemctl stop gopherpaper-frontend
sudo systemctl stop gopherpaper-backend
docker compose -f deploy/docker-compose.yml down
```

如果不使用 systemd，直接终止对应 `go run`、`./server` 和 `npm run start` 进程即可。

注意：`docker compose down` 不会删除 named volume；如执行 `docker compose down -v` 会删除数据库、Milvus、Neo4j 等持久化数据，生产环境禁止随意使用。
