# GopherCPP · 智能编程助教

基于 [eino](https://github.com/cloudwego/eino) 编排的多智能体编程助教

## 启动

```bash
# 1. 基础设施
docker compose -f deploy/docker-compose.yml up -d
#   Milvus 可视化 Attu: http://localhost:8000
#   RabbitMQ 管理台:    http://localhost:15672 (gopher/gopher)

# 2. 本地 ollama 拉模型
ollama pull qwen2.5-coder:1.5b   # 小模型
ollama pull bge-m3               # embedding

# 3. 配置：填入大模型 API key、SMTP 授权码等
cp config/config.example.toml config/config.toml
```
