# TODO

这里是 GopherPaper 的待完成清单

## Tools 和 MCP 待实现的功能

- 小云雀 Agent 的联网搜索能力
- 小云雀 Agent 的ArXiv论文查找能力
- 小云雀 Agent 的论文和工作台 Agent 的联动能力
- 小云雀 Agent 的我的论文库增删改查的能力

## tRPC-Agent-Go 框架

- Agent 的长时记忆和短时记忆的能力接入
- 小云雀 Agent 目前使用的是tRPC-Agent-Go的默认配置即ReAct模式，我们可以改为Plan-Execute-Replan

## 安全加固

> `download_paper` 工具(小云雀联网下载论文 PDF)是目前唯一接受任意 URL 并由服务端发起请求的工具,先验证可行性,以下加固后续补上。

- **SSRF 防护(已收敛/中)**:`fetchPDF` 现已限定 host 白名单只允许 `arxiv.org`(含子域),并给 `downloadClient` 设 `CheckRedirect` 对每一跳重定向目标重跑白名单校验,SSRF 面已收敛到 arxiv 单域。后续若要放开到更多论文站点(Semantic Scholar、bioRxiv 等),白名单每加一个域都要重新评估;若改为开放任意域,则必须补「解析 host→IP 拒绝私有/环回/链路本地段(`169.254.0.0/16`、`127.0.0.0/8`、`10/172.16/192.168`、`::1`)」。
- **下载配额/限速(中)**:LLM 可被诱导反复调用,每次落盘最多 50MB,无每用户配额,能打满磁盘。加每用户冷却或配额。
- **title 入库转义(低)**:`title` 原样存进 `Paper.Title`,前端渲染需转义防 XSS。

## 增强功能

- RAG 优化
  - 文本检索
    - 启用 hybrid 检索：Milvus schema 已有 BM25 sparse 字段，当前只用 dense vector，优先改为 dense + BM25 候选再 rerank。
    - 加 query rewrite：结合最近几轮 history，把“这个方法”“上面那张图”等指代问题改写成独立检索 query，生成仍使用用户原问题。
    - 加相邻块扩展：rerank 命中正文块后补同 doc、同 section 的 chunk_index 前后邻居，提升方法解释和跨段总结完整性。
    - 按 intent 调整召回策略：fact 偏 keyword/hybrid 和小上下文，summary/method 提高候选数并补邻近块。
    - 单篇问答补基础库：绑定 paperID 时当前只查本篇私有块，可考虑本篇为主、public 基础库少量补充，用于概念解释。
    - 统一 chunk 表征格式：正文用标题、页码、正文等显式标签，提升 embedding 和 rerank 可读性。
    - 修正 UpsertChunks 语义：当前底层 Add 是 insert，不是真 upsert，重复解析可能主键冲突。
  - 图片 / 图表 RAG
    - 图块内容结构化：把 caption、VLM 描述、页码、图表类型、所属章节、相邻正文用字段标签拼接，避免现在纯拼接语义过薄。
    - 表格优先结构化：MinerU 表格若能拿到 table_body，转成 Markdown 入库，比截图式 VLM 描述更适合问数据、指标和对比。
    - 图块召回改 hybrid：图名、Figure 3、数据集名、指标名更依赖关键词，图块也应走 dense + BM25 后 rerank。
    - 图块阈值动态化：当前 ImageScoreThreshold 固定 0.5，建议按 query 是否显式提图、候选分数分布和 rerank 分数自适应。
    - 附图前做去重与限流：按 img_uri 去重，限制总图片字节和像素，避免多张大图打爆 vision token 或请求体。
    - 增加多模态降级开关：模型或网关不支持图片时，只用图块文本作答，不应让问答链路失败。
    - 图引用更精确：图块 metadata 补 caption、figure_label、section，前端出处展示“图 3 / 表 2”而不只缩略图和页码。
    - 图描述质量评估：为“问图内容、问曲线趋势、问表格指标”准备小样本回归集，评估 topK 命中和答案是否插入正确 figure。
- 通过上一个主题的短时记忆，小云雀 Agent 能记住用户的近期研究方向，同时在上下问对话中，能减少工具的多次调用，例如记住用户的地理位置。
- 持续优化系统架构
- 通过当前用户的论文库和已有的论文关键词提取，是否能够搭建`知识图谱`。
- 继续优化我们的工作台 Agent 的意图识别。当前意图识别分类主要为：
  - fact 论文的事实 定位事实 / 数据 / 结论
  - summery 论文的总结 概括 / 解释 / 综述
  - method 论文的方法  方法 / 流程 / 实验设计解读
