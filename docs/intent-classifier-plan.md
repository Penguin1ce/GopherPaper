# 意图分类器优化计划：蒸馏微调 + 置信度级联路由

> 制定于 2026-07-15，目标两个月内完成，作为预推免面试的科研创新点。

## 背景

聊天框意图分类目前是 prompt 裸调 doubao-seed-mini 三分类（chitchat/summary/method，`internal/ai/chat/chat.go` 的 `ClassifyIntent`），准确率不足，代码里已积累两处硬规则补丁作为误判兜底：

- `methodIntentKeywordOverride`：「复现」关键词直走 method
- `paperResourceIntentOverride`：兜住「有 GitHub 仓库吗」这类资源短问句被误判成闲聊

规则补丁不可扩展，且本身就是分类器系统性误判的证据。此外三分类粒度太粗：RPC-Bench 显示 Why 类（设计动机/结果归因）是模型最弱项但被混在 summary 里，对比类问题按 Typed-RAG 需要拆解式检索却无对应类别，资源类问句只能靠硬规则兜。

目标：把分类体系按文献扩到 7 类，用「大模型蒸馏 + LoRA 微调开源小模型 + 置信度级联」替换 prompt 分类，每类路由到差异化的生成策略，产出可量化的准确率/延迟/成本对比。

约定：训练与评测代码放独立新 repo（建议名 `gopher-intent`），本仓只做 Go 侧级联集成与分类扩展；训练用本地 NVIDIA 卡；部署上线后置，不在本计划内。

## 分类体系（7 类）

设计原则：类目细到「下游处理真的不同」为止，每类必须对应独立的 prompt/检索策略/迭代预算，处理相同的类合并。体系参照 NF-CATS（SIGIR 2022）、RPC-Bench、QASPER 与 Typed-RAG 设计：

| 类别 | 语义 | 文献依据 | 下游路径 |
|---|---|---|---|
| chitchat | 与论文无关的闲聊/寒暄/身份提问 | 现有 | 直答不检索 |
| fact | 定位论文中的数值/定义/结论 | QASPER extractive（占其一半） | 单轮快路径（复用 `singleShotRAG`），省 agentic 开销 |
| summary | 概括/解释/综述 | 现有 | agentic RAG，概括 prompt |
| method | 方法/实验/技术流程（How） | RPC-Bench How | agentic RAG，宽迭代预算 |
| reason | 设计动机/合理性/结果归因（Why） | RPC-Bench Why、NF-CATS Reason | agentic RAG，新 prompt 侧重 intro/discussion 与动机论证 |
| comparison | 本文 vs 其他工作/方法变体对比 | NF-CATS Comparison、Typed-RAG | agentic RAG，对比 prompt；进阶做拆解式检索（两边分别召回再对比，可跨论文） |
| resource | GitHub/数据集/项目主页等资源定位 | 线上误判实证（现 `paperResourceIntentOverride` 硬规则） | 短检索定位链接，聚焦 prompt |

不设 unanswerable 类：分类器看不到库内内容，拒答由生成端在召回为空时处理。NF-CATS 的 Experience/Debate 类在论文问答场景无对应需求，不引入。

新旧映射：原 summary 拆出 fact/reason/comparison/resource；method 语义不变；chitchat 不变。7 分类的任务难度也让微调与级联的对比有区分度，避免三分类被 prompt 基线刷到顶。

## 交付物总览

1. 新 repo `gopher-intent`：评测集、蒸馏数据管线、训练脚本、评测框架、实验报告
2. 本仓：`IntentType` 扩到 7 类并接通各自下游路径；`ClassifyIntent` 支持置信度级联（配置驱动、未配置行为不变）
3. 一张核心对比表：zero-shot / few-shot / embedding+分类头 / LoRA 微调 / 微调+级联，各行 accuracy、per-class F1、P50/P99 延迟、单次成本

## 阶段一：评测集与基线（第 1–2 周）

新 repo 结构：

```
gopher-intent/
  data/eval.jsonl          # 人工校对评测集
  data/train.jsonl         # 蒸馏训练集
  scripts/synthesize.py    # teacher 合成数据
  scripts/dedup.py         # 去重与评测集防泄漏
  scripts/evaluate.py      # 统一评测入口
  train/                   # LLaMA-Factory 配置
  baselines/               # prompt 基线与 embedding 分类头
  report/                  # 实验记录与对比表
```

- 评测样本 schema（JSONL 一行一条）：`{"query", "history": [{"role","content"}], "label": "chitchat|fact|summary|method|reason|comparison|resource", "tags": ["指代","中英混杂","短问句","类别边界",...], "source": "manual|log|synthetic"}`。history 复刻线上格式：最近 2 条、每条截 360 字符（对齐 `constant.IntentContextMessages`）。
- 来源三路：手写难例（重点覆盖两处硬规则兜的场景与新类别间的边界，如 summary vs reason、method vs comparison）、`chat_test.go` 已有用例（按新体系重标）、teacher 合成难例后人工校对。目标 500–700 条（7 类比 3 类需要更多样本保证 per-class F1 稳定），类别均衡，难例占比 ≥ 40%。
- 基线脚本 `evaluate.py`：基线 prompt 在线上 `IntentPrompt`（`pkg/constant/constant.go`）基础上扩成 7 类版 + 同款 JSON 容错解析（复刻 `parseIntent` 逻辑）调 doubao-mini，报 accuracy / per-class F1 / P50 P99 延迟。分两个口径：纯模型、模型+规则 override（把两处硬规则移植成 Python 等价实现），量化规则补丁的实际贡献。同时在旧三分类口径下补测一版（7 类标签向旧类归并），保证与线上现状可比。

## 阶段二：蒸馏数据与训练（第 3–5 周）

- `synthesize.py`：teacher（doubao-pro，可换 DeepSeek 对比标注一致性）按 7 类别 × 难例维度矩阵生成 5k–8k 条带标签 query（含多轮 history 场景；类别边界样本刻意加密，如「为什么用这个方法」是 reason 而「这个方法怎么做」是 method），温度调高保多样性。
- `dedup.py`：MinHash/embedding 相似度去重；与评测集做泄漏过滤（相似度阈值剔除）。抽样 5–10% 人工复核标注质量。
- 训练：LLaMA-Factory LoRA 微调 **Qwen3-0.6B**（备选 1.7B 做规模消融），本地卡即可。输出格式训成与线上一致的 `{"type":"..."}` 单行 JSON，保证 Go 侧 `parseIntent` 零改动兼容。
- 基线补齐（都在 `baselines/`）：
  - few-shot prompt（doubao-mini，每类 3–5 例）
  - embedding+分类头：`Qwen/Qwen3-Embedding-0.6B`（与本仓知识库同款）取向量 + 逻辑回归，几乎零训练成本，是「为什么要 LoRA」的对照组
- 全部过 `evaluate.py` 出统一对比表。

## 阶段三：置信度级联（第 6 周）

- 微调模型推理时取标签 token 的 logprobs 归一为置信度；`evaluate.py` 扫描阈值画「准确率–升级率」曲线，选工作点。
- 低置信样本升级 teacher 二审，报级联后的 accuracy / 平均延迟 / 平均成本，与纯小模型、纯大模型两端对比——这是面试叙事的核心图表。

## 阶段四：Go 侧集成（第 7 周，本仓）

分类扩展：

- `pkg/constant/constant.go`：`IntentType` 增加 `IntentFact`/`IntentReason`/`IntentComparison`/`IntentResource`，`IsChat` 连带更新；`IntentPrompt` 换成 7 类版（与训练数据同源）。
- `internal/ai/chat/chat.go` `parseIntent`：识别新标签，非法仍兜底 summary；`ChatRAG` 分流扩展——fact 走 `singleShotRAG` 快路径（函数已有，恢复为一等路径），reason/comparison 走 agentic RAG 配新 prompt（`constant.AgenticRAGPromptFor` 扩支），resource 走短检索聚焦 prompt，`policyFor` 按类配迭代预算。
- comparison 的拆解式检索（Typed-RAG 思路：两边分别召回再对比）作为进阶项，首版先用 agentic 循环 + 对比 prompt（agent 本身可多轮检索，天然近似拆解）。

置信度级联：

- `internal/config/config.go`：`Models` 增加 `IntentFallback ModelConfig`（可选），intent 段增加 `ConfidenceThreshold float64`。
- `internal/aimodel/aimodel.go`：`ModelSet` 增加 fallback 模型槽位，`ModelsForUser` 按配置懒建（复用现有单例模式）。
- `internal/ai/chat/chat.go` `ClassifyIntent`：主模型返回后取置信度，低于阈值时调 fallback 模型重分类；未配置 fallback 或阈值为 0 时行为与现状完全一致。
- 实现前须验证：trpc-agent-go 的 openai model 是否透出 logprobs。若不透出，退路有二：训练时让模型输出 `{"type":"...","confidence":0.x}` 自报置信度（阶段三评测时同步验证其校准度，可用则 Go 侧零协议成本）；或在 `internal/ai/chat` 加一个直连 OpenAI 兼容端点带 `logprobs=true` 的轻量 HTTP 调用。
- 微调模型上线验证后，删除两处硬规则及其测试（评测集已覆盖这些场景，作为「规则被学习方法收编」的佐证）；集成阶段先保留，删除放到部署验证后。

## 阶段五：收尾与叙事（第 8 周）

- `report/` 汇总：对比表、准确率–升级率曲线、混淆矩阵（重点看 summary/reason、method/comparison 边界）、消融（0.6B vs 1.7B、有无 history、有无难例增广、7 类 vs 3 类归并口径）、误判案例分析。
- 面试一句话：发现 prompt 三分类在多轮指代与资源类短问句上系统性误判且粒度不足 → 参照 NF-CATS/RPC-Bench/QASPER 重设 7 类意图体系、每类路由差异化 RAG 策略（Typed-RAG 思路）→ 蒸馏 + LoRA 微调 0.6B 替换 prompt 分类 → 置信度级联兜底 → 准确率 X%→Y%、延迟/成本降 Z%、删除两处硬规则。

## 验证

- Python 侧：`evaluate.py` 对每个方案跑同一评测集，结果落 `report/`，可复现（固定 seed、记录模型版本与 prompt 版本）。
- Go 侧：`go test ./internal/ai/chat/... ./internal/router/...` 全绿；`chat_test.go` 用例按 7 类体系重标并补齐新类别与边界用例；未配置 fallback 时级联逻辑不生效、分类行为与仅换 prompt 等价；新增级联单测用 httptest mock 两级端点（高置信不升级、低置信升级、fallback 失败兜底主结果）。

## 风险

- teacher 标注噪声：7 类边界（summary/reason、method/comparison）teacher 自己也会标错，抽样人工复核 + 双 teacher 交叉标注不一致样本人工裁决；类别定义文档随数据集一起维护，标注口径漂移时先改定义再重标。
- 类别扩展导致下游工作量膨胀：reason/comparison/resource 的新 prompt 首版从现有 summary/method prompt 派生微调，不追求一步到位；comparison 拆解式检索明确列为进阶项，时间不够就砍。
- 评测集难例占比压到 40%+，区分度靠级联、混淆矩阵与消融而非裸准确率。
- logprobs 通路不确定：阶段四开工前先验证，两条退路已列。

## 参考文献

- Bolotova et al. A Non-Factoid Question-Answering Taxonomy. SIGIR 2022 Best Paper. https://dl.acm.org/doi/10.1145/3477495.3531926 （NF-CATS 数据集 https://github.com/Lurunchik/NF-CATS）
- Typed-RAG: Type-Aware Decomposition of Non-Factoid Questions for RAG. https://arxiv.org/abs/2503.15879
- Dasigi et al. QASPER: A Dataset of Information-Seeking Questions and Answers Anchored in Research Papers. NAACL 2021. https://arxiv.org/abs/2105.03011
- RPC-Bench: A Fine-grained Benchmark for Research Paper Comprehension. https://arxiv.org/abs/2601.14289
- REIC: RAG-Enhanced Intent Classification at Scale. EMNLP 2025 Industry. https://aclanthology.org/2025.emnlp-industry.74.pdf
