# Neo4j 知识图谱设计

本文描述 GopherPaper 中 Neo4j 的节点、关系、约束与写入边界。Neo4j 用于承载论文知识图谱，支撑论文关系发现、研究趋势分析、相似论文推荐与个人论文库知识管理。

## 一、设计边界

GopherPaper 同时使用 MySQL、Milvus、Redis 与 Neo4j，各自职责如下：

| 存储 | 职责 |
| --- | --- |
| MySQL | 用户、论文记录、解析状态、结构化元信息、报告、会话元数据 |
| Milvus | 论文 chunk 向量、RAG 检索召回 |
| Redis | 验证码、登录态、Agent 工作记忆等临时数据 |
| Neo4j | 论文、作者、关键词、机构、研究问题、方法、实验、结果、创新点、引用与相似关系 |

当前 Neo4j 图谱按用户隔离。所有图谱节点都带 `owner` 字段，值为用户学号 `student_id`。查询、写入、删除均必须带 `owner` 条件。

## 二、通用字段约定

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `owner` | string | 图谱归属用户，对应 `users.student_id` |
| `name` | string | 实体原始名称或展示名称 |
| `norm` | string | 归一化名称，用于去重，通常为小写、去符号、压缩空格后的文本 |
| `updated_at` | integer | 更新时间戳，单位为毫秒 |

实体节点通常通过 `(owner, norm)` 保证用户维度下唯一。论文节点通过 `(owner, id)` 保证唯一。

## 三、节点定义

### 1. `Paper` 论文节点

论文图谱的中心节点，对应 MySQL 中的 `papers.id`。

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `owner` | string | 是 | 论文归属用户，对应学号 |
| `id` | string | 是 | 论文 UUID，对应 MySQL `papers.id` |
| `title` | string | 是 | 论文标题 |
| `norm_title` | string | 否 | 归一化标题，用于引用匹配 |
| `year` | integer | 否 | 发表年份 |
| `venue` | string | 否 | 发表会议、期刊或来源 |
| `embedding` | list<float> | 否 | 论文级向量，用于粗粒度相似论文发现 |
| `semantic_profile` | string | 否 | 论文语义画像文本 |
| `semantic_embedding` | list<float> | 否 | 语义画像向量，用于语义相似召回 |
| `updated_at` | integer | 是 | 更新时间戳 |

唯一键：

```cypher
(:Paper {owner, id})
```

### 2. `Author` 作者节点

表示论文作者。

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `owner` | string | 是 | 图谱归属用户 |
| `name` | string | 是 | 作者名称 |
| `norm` | string | 是 | 归一化作者名称 |

唯一键：

```cypher
(:Author {owner, norm})
```

### 3. `Keyword` 关键词节点

表示论文关键词、主题词或核心概念。

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `owner` | string | 是 | 图谱归属用户 |
| `name` | string | 是 | 关键词名称 |
| `norm` | string | 是 | 归一化关键词 |

唯一键：

```cypher
(:Keyword {owner, norm})
```

### 4. `Affiliation` 机构节点

表示作者单位、学校、研究机构或公司。

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `owner` | string | 是 | 图谱归属用户 |
| `name` | string | 是 | 机构名称 |
| `norm` | string | 是 | 归一化机构名称 |

唯一键：

```cypher
(:Affiliation {owner, norm})
```

### 5. `Venue` 发表来源节点

表示会议、期刊、预印本平台或其他发表来源。

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `owner` | string | 是 | 图谱归属用户 |
| `name` | string | 是 | 来源名称 |
| `norm` | string | 是 | 归一化来源名称 |

唯一键：

```cypher
(:Venue {owner, norm})
```

### 6. `ResearchQuestion` 研究问题节点

表示论文关注或试图回答的研究问题。

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `owner` | string | 是 | 图谱归属用户 |
| `name` | string | 是 | 研究问题文本 |
| `norm` | string | 是 | 归一化研究问题文本 |

唯一键：

```cypher
(:ResearchQuestion {owner, norm})
```

### 7. `Method` 方法节点

表示论文提出、使用或对比的方法、模型、算法、框架或流程。

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `owner` | string | 是 | 图谱归属用户 |
| `name` | string | 是 | 方法名称或方法概括 |
| `norm` | string | 是 | 归一化方法文本 |

唯一键：

```cypher
(:Method {owner, norm})
```

### 8. `Experiment` 实验节点

表示论文的实验设置、数据集、实验流程或评测方案概括。

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `owner` | string | 是 | 图谱归属用户 |
| `name` | string | 是 | 实验内容概括 |
| `norm` | string | 是 | 归一化实验文本 |

唯一键：

```cypher
(:Experiment {owner, norm})
```

### 9. `Result` 结果节点

表示论文的主要实验结果、发现或结论概括。

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `owner` | string | 是 | 图谱归属用户 |
| `name` | string | 是 | 结果内容概括 |
| `norm` | string | 是 | 归一化结果文本 |

唯一键：

```cypher
(:Result {owner, norm})
```

### 10. `Innovation` 创新点节点

表示论文的贡献点、创新点或新颖性描述。

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `owner` | string | 是 | 图谱归属用户 |
| `name` | string | 是 | 创新点文本 |
| `norm` | string | 是 | 归一化创新点文本 |

唯一键：

```cypher
(:Innovation {owner, norm})
```

### 11. `Limitation` 局限性节点

表示论文中声明或抽取出的局限性。

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `owner` | string | 是 | 图谱归属用户 |
| `name` | string | 是 | 局限性文本 |
| `norm` | string | 是 | 归一化局限性文本 |

唯一键：

```cypher
(:Limitation {owner, norm})
```

### 12. `FutureWork` 未来工作节点

表示论文提出的后续研究方向。

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `owner` | string | 是 | 图谱归属用户 |
| `name` | string | 是 | 未来工作文本 |
| `norm` | string | 是 | 归一化未来工作文本 |

唯一键：

```cypher
(:FutureWork {owner, norm})
```

### 13. `Reference` 参考文献节点

表示尚未匹配为系统内论文节点的参考文献条目。

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `owner` | string | 是 | 图谱归属用户 |
| `key` | string | 是 | 归一化参考文献 key |
| `raw` | string | 是 | 原始参考文献文本 |

唯一键：

```cypher
(:Reference {owner, key})
```

## 四、关系定义

### 1. `AUTHORED_BY`

论文由作者撰写。

```cypher
(Paper)-[:AUTHORED_BY]->(Author)
```

| 起点 | 关系 | 终点 | 说明 |
| --- | --- | --- | --- |
| `Paper` | `AUTHORED_BY` | `Author` | 一篇论文可以关联多个作者 |

### 2. `HAS_KEYWORD`

论文包含关键词或主题概念。

```cypher
(Paper)-[:HAS_KEYWORD]->(Keyword)
```

| 起点 | 关系 | 终点 | 说明 |
| --- | --- | --- | --- |
| `Paper` | `HAS_KEYWORD` | `Keyword` | 用于主题聚类、关键词趋势与相关论文发现 |

### 3. `FROM_AFFILIATION`

论文关联作者机构。

```cypher
(Paper)-[:FROM_AFFILIATION]->(Affiliation)
```

| 起点 | 关系 | 终点 | 说明 |
| --- | --- | --- | --- |
| `Paper` | `FROM_AFFILIATION` | `Affiliation` | 表示论文涉及的作者单位集合 |

### 4. `PUBLISHED_IN`

论文发表在某个来源。

```cypher
(Paper)-[:PUBLISHED_IN]->(Venue)
```

| 起点 | 关系 | 终点 | 说明 |
| --- | --- | --- | --- |
| `Paper` | `PUBLISHED_IN` | `Venue` | 来源可以是会议、期刊或预印本平台 |

### 5. `HAS_RESEARCH_QUESTION`

论文关注某个研究问题。

```cypher
(Paper)-[:HAS_RESEARCH_QUESTION]->(ResearchQuestion)
```

| 起点 | 关系 | 终点 | 说明 |
| --- | --- | --- | --- |
| `Paper` | `HAS_RESEARCH_QUESTION` | `ResearchQuestion` | 用于按问题组织论文与发现研究脉络 |

### 6. `USES_METHOD`

论文使用、提出或讨论某种方法。

```cypher
(Paper)-[:USES_METHOD]->(Method)
```

| 起点 | 关系 | 终点 | 说明 |
| --- | --- | --- | --- |
| `Paper` | `USES_METHOD` | `Method` | 用于方法维度的论文关联和对比 |

### 7. `HAS_EXPERIMENT`

论文包含某类实验设置或评测方案。

```cypher
(Paper)-[:HAS_EXPERIMENT]->(Experiment)
```

| 起点 | 关系 | 终点 | 说明 |
| --- | --- | --- | --- |
| `Paper` | `HAS_EXPERIMENT` | `Experiment` | 用于比较论文实验设计、数据集和评测过程 |

### 8. `HAS_RESULT`

论文包含某个结果或结论。

```cypher
(Paper)-[:HAS_RESULT]->(Result)
```

| 起点 | 关系 | 终点 | 说明 |
| --- | --- | --- | --- |
| `Paper` | `HAS_RESULT` | `Result` | 用于结果对比、结论归纳和报告生成 |

### 9. `HAS_INNOVATION`

论文包含某个创新点。

```cypher
(Paper)-[:HAS_INNOVATION]->(Innovation)
```

| 起点 | 关系 | 终点 | 说明 |
| --- | --- | --- | --- |
| `Paper` | `HAS_INNOVATION` | `Innovation` | 用于创新点分析和论文贡献对比 |

### 10. `HAS_LIMITATION`

论文包含某个局限性。

```cypher
(Paper)-[:HAS_LIMITATION]->(Limitation)
```

| 起点 | 关系 | 终点 | 说明 |
| --- | --- | --- | --- |
| `Paper` | `HAS_LIMITATION` | `Limitation` | 用于局限性归纳和未来研究建议 |

### 11. `HAS_FUTURE_WORK`

论文提出某个未来工作方向。

```cypher
(Paper)-[:HAS_FUTURE_WORK]->(FutureWork)
```

| 起点 | 关系 | 终点 | 说明 |
| --- | --- | --- | --- |
| `Paper` | `HAS_FUTURE_WORK` | `FutureWork` | 用于挖掘后续研究方向 |

### 12. `CITES`

论文引用参考文献或系统内已有论文。

```cypher
(Paper)-[:CITES]->(Reference)
(Paper)-[:CITES]->(Paper)
```

| 起点 | 关系 | 终点 | 说明 |
| --- | --- | --- | --- |
| `Paper` | `CITES` | `Reference` | 引用原始参考文献条目 |
| `Paper` | `CITES` | `Paper` | 当参考文献标题能匹配系统内论文时，直接连接到论文节点 |

当前实现中，引用关系由参考文献文本归一化后写入。若参考文献文本中包含某篇已入库论文的归一化标题，则建立 `Paper -> Paper` 的直接引用边。

### 13. `SIMILAR_TO`

论文与论文之间的向量相似关系。

```cypher
(Paper)-[:SIMILAR_TO {
  score: float,
  updated_at: integer
}]->(Paper)
```

| 属性 | 类型 | 说明 |
| --- | --- | --- |
| `score` | float | 基于 `embedding` 的 cosine 相似度 |
| `updated_at` | integer | 更新时间戳 |

说明：

- 写入论文后，系统会基于论文级 `embedding` 重新计算该论文指向其他论文的 `SIMILAR_TO` 边。
- 只保留超过阈值的 Top K 相似论文。
- 当前实现删除并重建当前论文的出边，避免旧相似关系残留。

### 14. `SEMANTIC_SIMILAR`

论文与论文之间的语义相似关系。

```cypher
(Paper)-[:SEMANTIC_SIMILAR {
  score: float,
  matched_fields: list<string>,
  summary: string,
  keyword_similarity: string,
  research_question_similarity: string,
  method_similarity: string,
  experiment_similarity: string,
  innovation_similarity: string,
  model: string,
  updated_at: integer
}]->(Paper)
```

| 属性 | 类型 | 说明 |
| --- | --- | --- |
| `score` | float | 语义相似度分数 |
| `matched_fields` | list<string> | 命中的相似字段，如关键词、研究问题、方法、实验、创新点 |
| `summary` | string | 相似性摘要 |
| `keyword_similarity` | string | 关键词相似性说明 |
| `research_question_similarity` | string | 研究问题相似性说明 |
| `method_similarity` | string | 方法相似性说明 |
| `experiment_similarity` | string | 实验或数据集相似性说明 |
| `innovation_similarity` | string | 创新点相似性说明 |
| `model` | string | 判定相似关系的模型 |
| `updated_at` | integer | 更新时间戳 |

说明：

- `SEMANTIC_SIMILAR` 是比 `SIMILAR_TO` 更可解释的相似论文关系。
- 它不仅保存分数，还保存相似原因，前端可用于展示论文之间为什么相关。

## 五、关系总览

```text
Paper
 ├─[:AUTHORED_BY]──────────────> Author
 ├─[:HAS_KEYWORD]──────────────> Keyword
 ├─[:FROM_AFFILIATION]─────────> Affiliation
 ├─[:PUBLISHED_IN]─────────────> Venue
 ├─[:HAS_RESEARCH_QUESTION]───> ResearchQuestion
 ├─[:USES_METHOD]─────────────> Method
 ├─[:HAS_EXPERIMENT]──────────> Experiment
 ├─[:HAS_RESULT]──────────────> Result
 ├─[:HAS_INNOVATION]──────────> Innovation
 ├─[:HAS_LIMITATION]──────────> Limitation
 ├─[:HAS_FUTURE_WORK]─────────> FutureWork
 ├─[:CITES]───────────────────> Reference
 ├─[:CITES]───────────────────> Paper
 ├─[:SIMILAR_TO]──────────────> Paper
 └─[:SEMANTIC_SIMILAR]────────> Paper
```

## 六、约束与索引

Neo4j 初始化时应幂等创建以下约束与索引。

```cypher
CREATE CONSTRAINT paper_owner_id IF NOT EXISTS
FOR (p:Paper) REQUIRE (p.owner, p.id) IS UNIQUE;

CREATE CONSTRAINT author_owner_norm IF NOT EXISTS
FOR (a:Author) REQUIRE (a.owner, a.norm) IS UNIQUE;

CREATE CONSTRAINT keyword_owner_norm IF NOT EXISTS
FOR (k:Keyword) REQUIRE (k.owner, k.norm) IS UNIQUE;

CREATE CONSTRAINT affiliation_owner_norm IF NOT EXISTS
FOR (a:Affiliation) REQUIRE (a.owner, a.norm) IS UNIQUE;

CREATE CONSTRAINT venue_owner_norm IF NOT EXISTS
FOR (v:Venue) REQUIRE (v.owner, v.norm) IS UNIQUE;

CREATE CONSTRAINT research_question_owner_norm IF NOT EXISTS
FOR (q:ResearchQuestion) REQUIRE (q.owner, q.norm) IS UNIQUE;

CREATE CONSTRAINT method_owner_norm IF NOT EXISTS
FOR (m:Method) REQUIRE (m.owner, m.norm) IS UNIQUE;

CREATE CONSTRAINT experiment_owner_norm IF NOT EXISTS
FOR (e:Experiment) REQUIRE (e.owner, e.norm) IS UNIQUE;

CREATE CONSTRAINT result_owner_norm IF NOT EXISTS
FOR (r:Result) REQUIRE (r.owner, r.norm) IS UNIQUE;

CREATE CONSTRAINT innovation_owner_norm IF NOT EXISTS
FOR (i:Innovation) REQUIRE (i.owner, i.norm) IS UNIQUE;

CREATE CONSTRAINT limitation_owner_norm IF NOT EXISTS
FOR (l:Limitation) REQUIRE (l.owner, l.norm) IS UNIQUE;

CREATE CONSTRAINT future_work_owner_norm IF NOT EXISTS
FOR (f:FutureWork) REQUIRE (f.owner, f.norm) IS UNIQUE;

CREATE CONSTRAINT reference_owner_key IF NOT EXISTS
FOR (r:Reference) REQUIRE (r.owner, r.key) IS UNIQUE;

CREATE INDEX paper_owner IF NOT EXISTS
FOR (p:Paper) ON (p.owner);

CREATE INDEX paper_year IF NOT EXISTS
FOR (p:Paper) ON (p.year);
```

## 七、写入流程

论文解析完成后，Neo4j 图谱写入建议发生在结构化抽取之后。

```text
PDF 上传
  -> MinerU 解析
  -> 结构化抽取 paper_metas
  -> 构建 Milvus chunk 向量
  -> 构建 PaperGraph
  -> 写入 Neo4j 节点与基础关系
  -> 计算 SIMILAR_TO / SEMANTIC_SIMILAR
  -> 论文状态进入 ready
```

写入策略：

1. `MERGE (p:Paper {owner, id})` 创建或更新论文节点。
2. 删除当前论文旧的元信息关系，避免重复或脏边。
3. 对作者、关键词、机构、来源、研究问题、方法、实验、结果、创新点、局限性、未来工作做归一化去重。
4. 通过 `(owner, norm)` 或 `(owner, key)` 合并实体节点。
5. 重建当前论文指向实体节点的关系。
6. 如参考文献已抽取，重建 `CITES` 关系。
7. 清理孤立实体节点。
8. 根据向量与语义画像刷新论文间相似关系。

## 八、查询隔离原则

所有查询必须带 `owner` 条件。

```cypher
MATCH (p:Paper {owner:$owner})
RETURN p;
```

单篇论文图谱查询：

```cypher
MATCH (p:Paper {owner:$owner, id:$paper_id})
OPTIONAL MATCH (p)-[r]->(n)
RETURN p, r, n;
```

相似论文查询：

```cypher
MATCH (p:Paper {owner:$owner, id:$paper_id})-[s:SEMANTIC_SIMILAR|SIMILAR_TO]-(q:Paper {owner:$owner})
WHERE q.id <> p.id
RETURN q.id AS id, q.title AS title, type(s) AS relation, s.score AS score
ORDER BY score DESC
LIMIT 10;
```

关键词趋势查询：

```cypher
MATCH (p:Paper {owner:$owner})-[:HAS_KEYWORD]->(k:Keyword)
WHERE p.year > 0
RETURN k.name AS keyword, p.year AS year, count(*) AS count
ORDER BY keyword, year;
```

## 九、当前不进入 Neo4j 的数据

以下数据当前不作为 Neo4j 节点保存：

| 数据 | 当前存储 | 原因 |
| --- | --- | --- |
| PDF 原文 chunk | Milvus | 用于向量检索和 RAG 召回 |
| chunk 页码、章节、图片引用元数据 | Milvus metadata / MySQL | 用于问答出处与前端定位 |
| 论文解析状态 | MySQL `papers` | 属于业务流程状态 |
| 研读报告正文 | MySQL `paper_reports` | 属于可缓存业务产物 |
| 对话历史 | trpc-agent-go MySQL Session 表 | 属于会话框架数据 |

如后续需要在图谱中支持页码级证据链，可以新增 `Chunk`、`Figure`、`Table`、`Finding` 等节点，并通过 `Finding -> Chunk/Figure/Table` 建立可追溯证据关系。
