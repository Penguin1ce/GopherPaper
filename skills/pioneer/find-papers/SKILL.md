---
name: find-papers
description: 当用户要找论文、查文献、调研某个研究方向有哪些工作、找某主题的经典/最新论文、查某篇论文被谁引用/参考了谁/相似论文、要一份阅读清单或参考文献列表,且目标是输出文献清单而不是连续正文时使用。涉及 找论文、查文献、被引论文、参考文献、相似论文、阅读清单、经典论文、最新进展、有哪些工作 时遵循本流程。若用户明确要求写正文,或要围绕自己的论文贡献定位前人工作,不要使用本 skill。
---

# 文献检索调研流程(小云雀)

用户想找某个主题的论文。你的任务是**把相关文献找全、找准,整理成一份可直接用的阅读清单**,而不是写成综述。核心纪律:多源检索、按价值筛选、带真实出处、不编造。

## 可用工具

- `search_conference_proceedings`:查会议官方 proceedings。用户问"某会议某年份论文/推荐/有哪些/accepted paper"时优先调用,当前主要覆盖 NeurIPS 官方论文集。
- `search_openreview_papers`:查 OpenReview 上某会议某年份的 accepted/public papers。支持 NeurIPS、ICLR、ICML 的 `venue/year` 映射,或直接传 `venue_id`。
- `search_semantic_scholar`:返回**引用数**,用来找该方向的经典/高被引奠基工作。
- `search_arxiv`:找最新预印本,补近一两年的新工作。
- `recommend_similar_papers`:从一篇核心论文扩展相似工作,补关键词检索漏掉的邻近方向。
- `get_paper_citations`:从一篇核心论文追后续引用,找跟进工作和最新改进。
- `get_paper_references`:从一篇核心论文回溯参考文献,找奠基工作和上游方法。
- `web_search`:学术库没覆盖到的信息(如某方法的博客、官方主页),只作补充。
- `search_my_papers` / `list_my_papers`:用户自己的论文库,相关就并入清单。
- `download_paper`:用户明确要把某篇 PDF 存进工作台时才用。若要导入的是刚才结果里的论文,优先直接使用该条结果已有的 `pdf_url`;没有 `pdf_url` 时,会议论文先回官方 proceedings/OpenReview 按标题补链,再考虑 Semantic Scholar,最后才用 arXiv。

检索工具返回结果自带 BibTeX,直接用于清单的引用,不要自己编造引用格式。

## ID 纪律

- `list_my_papers` 返回的 `paper_id` 是本站 UUID,只能传给 `search_my_papers` 等本站工具。
- `recommend_similar_papers`、`get_paper_citations`、`get_paper_references` 的 `paper_id` 必须来自 `search_semantic_scholar` 返回的 Semantic Scholar `paper_id`,或使用 arXiv 编号/DOI。
- 用户指定工作台里的某篇论文来查被引/参考/相似论文时,流程是:`list_my_papers` 定位标题 → `search_semantic_scholar` 按标题取外部 `paper_id` → 调用顺链工具。不要把本站 UUID 直接传给顺链工具。

## 检索流程

1. **先明确检索意图**:用户要的是"该方向最重要的几篇经典",还是"最新进展",还是"全面调研"?不清楚就问一句,再决定工具配比。
2. **会议年份先查官方源**:用户指定了会议+年份(如"2025 NeurIPS Agent 论文")时,先调用 `search_conference_proceedings` 或 `search_openreview_papers`,以官方 proceedings / OpenReview 为录用依据;不要只靠 `search_semantic_scholar` 或 `search_arxiv` 下结论。
3. **分主题多轮检索**:把主题拆成几个聚焦子方向,逐个检索,不要用一个宽词查一次了事。query 用自然语言主题词,含指代先改写独立。会议年份题尤其要扩展同义词,如 `agent`、`agents`、`agentic`、`LLM agent`、`web agent`、`multi-agent`、`tool use`。
4. **经典与最新结合**:`search_semantic_scholar` 抓高引骨架和引用数,`search_arxiv` 补最新预印本/PDF;用户库相关就 `search_my_papers`。但会议录用身份以官方源为准。
5. **顺链扩展核心论文**:每个子方向找到 1-2 篇核心种子后,先确认种子有外部学术标识,再用相似推荐补邻近论文,用引用找后续工作,用参考文献回溯经典。不要无限顺链,只保留和主题直接相关的结果。
6. **去重与筛选**:不同工具会返回重叠论文,合并去重;按**官方录用证据 + 相关性 + 引用数 + 年份 + 证据链位置**取舍,宁缺毋滥,不堆砌弱相关文献。
7. **召回不足就再查**:某子方向返回太少或不对题,换更聚焦或更宽的词再查一轮。官方源为空时只能说"官方源当前未召回",不能说该会议一定没有。

## 交付:阅读清单

把筛选后的文献整理成清单,每篇给出:

- 标题、作者、年份(必要时会议/期刊)
- 引用数(若工具返回)
- **一句话评述**:这篇解决了什么、为什么值得读、在该方向里的位置
- 工具返回的 BibTeX

建议按子方向或"经典 / 最新"分组,让用户一眼看清结构。最后可主动问:要不要把其中某几篇 `download_paper` 存进工作台,或基于这份清单写综述(可转 literature-review)。若清单里的工具结果带 `pdf_url`,后续用户同意导入时直接复用该链接。

## 强约束

- **只列检索真实返回的论文**,不得虚构标题/作者/年份/引用数。
- **会议录用身份必须来自官方源**。Semantic Scholar/arXiv 结果可做补充,但不能单独证明某论文属于某会议某年份。
- **导入不重复检索**。上一轮或当前结果已有 `pdf_url` 时,不要再为了同一篇论文调用 Semantic Scholar/arXiv;直接用 `download_paper`。
- 检索不到代表性文献时如实说明,不要脑补凑数。
- 不评判用户读不到的细节(没下载的论文只依据检索返回的摘要信息评述)。
