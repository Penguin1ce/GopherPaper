// Package constant 集中定义全项目共享的常量与枚举。
package constant

import "time"

// Provider 标识模型组件来源。
type Provider string

const (
	ProviderOllama Provider = "ollama"
	ProviderOpenAI Provider = "openai"
)

// IntentType 标识一次论文问答的处理路径。
// fact/summary/method 是聊天框的问答子类,由意图分类模型选择;
// 研读报告是显式动作，由前端按钮带 ReportType 触发，不经分类器。
type IntentType string

const (
	IntentFact    IntentType = "fact"    // 定位事实/数据/结论
	IntentSummary IntentType = "summary" // 概括/解释/综述
	IntentMethod  IntentType = "method"  // 方法/流程/实验设计解读
)

// IntentPioneer 标识小云雀会话的应答,小云雀会话不经意图分类器。
const IntentPioneer IntentType = "pioneer"

// IsChat 判断是否为聊天框可调度的问答子类。
func (t IntentType) IsChat() bool {
	switch t {
	case IntentFact, IntentSummary, IntentMethod:
		return true
	default:
		return false
	}
}

// ReportType 标识研读报告类型，由前端按钮触发，不经意图分类器。
type ReportType string

const (
	ReportQuickRead  ReportType = "quickread"  // 论文速读
	ReportMethod     ReportType = "method"     // 研究方法总结
	ReportResult     ReportType = "result"     // 实验结果总结
	ReportInnovation ReportType = "innovation" // 创新点与不足分析
	ReportCompare    ReportType = "compare"    // 同类文献对比
	ReportFuture     ReportType = "future"     // 后续研究建议
)

// Valid 判断报告类型是否合法。
func (t ReportType) Valid() bool {
	switch t {
	case ReportQuickRead, ReportMethod, ReportResult, ReportInnovation, ReportCompare, ReportFuture:
		return true
	default:
		return false
	}
}

// AllReportTypes 返回全部研读报告类型，供解析完成后批量预生成扇出。
func AllReportTypes() []ReportType {
	return []ReportType{ReportQuickRead, ReportMethod, ReportResult, ReportInnovation, ReportCompare, ReportFuture}
}

// PaperStatus 是论文从上传到就绪的解析状态机。
type PaperStatus string

const (
	PaperUploaded  PaperStatus = "uploaded"  // 已上传待解析
	PaperParsing   PaperStatus = "parsing"   // MinerU 解析中
	PaperExtracted PaperStatus = "extracted" // 结构化抽取完成
	PaperIndexed   PaperStatus = "indexed"   // 已分块入向量库
	PaperReady     PaperStatus = "ready"     // 全流程就绪
	PaperFailed    PaperStatus = "failed"    // 解析失败
)

// MaxPaperDownloadBytes 小云雀联网下载论文 PDF 的体积上限,与上传接口的 50MB 对齐,防超大文件打满磁盘。
const MaxPaperDownloadBytes = 50 << 20

// HTTP 鉴权相关。
const (
	HeaderAuthorization = "Authorization"
	BearerPrefix        = "bearer "
)

// 小云雀 agent 相关。小云雀是面向用户的多面手 agent,挂独立分组的 mcp 工具与 skill;
// 凭据型工具的 token 由前端保存、随请求头透传,服务端按调用注入,不落库。
const (
	AgentPioneer      = "pioneer"        // 工具分组名,也是会话 AgentType 的取值
	CredentialLuckin  = "luckin"         // 瑞幸凭据 provider 名,对应 mcp 配置的 credential 字段
	HeaderLuckinToken = "X-Luckin-Token" // 前端随消息携带瑞幸 token 的请求头
)

// 知识库相关。
const (
	DefaultKnowledgeCollection = "knowledge_chunks"
	// TopKKnowledge 最终拼进 context 的正文块数。开启 rerank 时为精排后截断数,关闭时即向量召回数。
	TopKKnowledge = 8
	// RecallTopK 开启 rerank 时第一阶段向量召回的候选数,扩大召回保 recall,再由 cross-encoder 精排截到 TopKKnowledge。
	RecallTopK = 30
	// MaxChunkRunes 单个知识块正文的字符上限,同标题同页的碎段合并到此为止,超出再切。
	// bge-m3 支持长文,但块过大召回精度下降,取折中值。
	MaxChunkRunes = 1000
)

// 带图问答相关。问答时图块走单独一轮检索,不与正文同池竞争。
const (
	TopKImages           = 3   // 单轮问答最多带几张召回图,vision token 贵故限张
	RecallTopKImages     = 20  // 开启 rerank 时图块第一阶段向量召回候选数,过向量阈值后再精排截到 TopKImages
	ImageScoreThreshold  = 0.5 // 图召回 score 低于此阈值视为无关,不带图(COSINE 相似度)
	MaxFigureDescribe    = 20  // 解析期单篇最多给几张图调 vlm 生成描述,超出只留 caption
	FigureDescribeWorker = 4   // 解析期 vlm 图描述的并发上限
)

// 多轮对话相关。
const (
	MaxContextMessages = 20            // 喂给模型的历史消息最大条数，超出只取最近的
	SessionAppName     = "gopherpaper" // trpc Session 的 appName，与 userID/sessionID 共同定位会话事件
	// PioneerMaxHistoryRuns 是小云雀工作记忆喂模型的最大消息条数。含工具调用/返回,故比纯文本窗口大,
	// 既保留近几轮工具轨迹供复用,又防 ReAct 轨迹无限堆积撑爆上下文。
	PioneerMaxHistoryRuns = 40
)

// PioneerSessionKeyPrefix 是小云雀工作记忆在 Redis 里的键前缀,与展示历史(MySQL)分库,互不污染。
const PioneerSessionKeyPrefix = "gp-sess"

// PioneerSessionTTL 是小云雀工作记忆(Redis session)的空闲过期时间。
// 跨进程重启不丢(存 Redis 非内存),闲置 7 天后自动回收;展示历史另在 MySQL 永久留存。
const PioneerSessionTTL = 7 * 24 * time.Hour

// 发消息 SSE 事件名。生成过程经 SSE 推送:工具调用与文本增量实时上屏,done 收尾带完整消息。
const (
	StreamEventToolCall   = "tool_call"   // agent 发起一次工具调用,载荷带工具名
	StreamEventToolResult = "tool_result" // 工具调用返回,载荷带工具名
	StreamEventDelta      = "delta"       // 应答文本增量
	StreamEventPlan       = "plan"        // 先锋者规划/动作阶段文本,载荷带 phase 与增量
	StreamEventDone       = "done"        // 生成完成,载荷为完整 SendMessageResponse
	StreamEventError      = "error"       // 生成中途失败,载荷带错误说明
)

type KnowledgeScope string

const (
	KnowledgeScopePublic  KnowledgeScope = "public"
	KnowledgeScopePrivate KnowledgeScope = "private"
)

const (
	MilvusFieldID             = "id"
	MilvusFieldContent        = "content"
	MilvusFieldVector         = "vector"
	MilvusFieldMetadata       = "metadata"
	MilvusFieldKnowledgeScope = "knowledge_scope"
	MilvusFieldStudentID      = "student_id"
	MilvusFieldDocID          = "doc_id"
	MilvusFieldSourceFile     = "source_file"
	MilvusFieldSourceURI      = "source_uri"
	MilvusFieldPageNo         = "page_no"
	MilvusFieldChunkIndex     = "chunk_index"
	MilvusFieldCreatedAt      = "created_at"
	MilvusFieldBlockType      = "block_type" // 块类型 text/image,图片块带 img_uri 供带图问答
	MilvusFieldImgURI         = "img_uri"    // 图片块对应的本地图片路径
)

// 知识块类型,区分正文文本块与图片块。
const (
	BlockTypeText  = "text"
	BlockTypeImage = "image"
)

// Redis 键前缀与时效。
const (
	RedisKeyVerifyCode  = "verify_code:"  // 邮箱验证码，键拼接邮箱
	RedisKeyUserToken   = "jwt:"          // 登录 token，键拼接邮箱前缀
	RedisKeyParseStatus = "paper:status:" // 论文解析状态缓存，键拼接 paperID
)

// VerifyCodeTTL 邮箱验证码有效期。
const VerifyCodeTTL = 5 * time.Minute

// IntentPrompt 是论文问答的意图分类 system prompt，只在问答子类间分类。
const IntentPrompt = `你是科研文献问答助手的意图分类器，判断用户提问属于以下哪一类：
- fact:    询问论文中的具体事实、数据、结论、数值、定义
- summary: 想要对论文整体或某部分做概括、解释、综述
- method:  关注研究方法、实验设计、技术流程、步骤细节

只输出一个 JSON，禁止任何多余文字。格式：
{{"type":"fact|summary|method"}}
无法判断时输出 {{"type":"summary"}}。`

// 三类问答 agent 的 system prompt，均带 {context} 检索占位符。
const (
	FactPrompt = `你是严谨的科研文献问答助手，负责定位论文中的事实、数据与结论。优先依据下面的「参考资料」作答，资料不足时明确说明，不要编造。
回答务必简短直接：给出准确的事实/数值，并标明依据来自哪一段或哪一页。控制在 200 字以内。

参考资料：
{context}`

	SummaryPrompt = `你是科研文献问答助手，负责概括与解释论文内容。结合下面的「参考资料」，用条理清晰的语言概括要点，避免堆砌细节。
回答务必简短：抓主线，必要时分点。控制在 300 字以内。

参考资料：
{context}`

	MethodPrompt = `你是科研文献问答助手，负责解读研究方法与实验流程。结合下面的「参考资料」，按步骤讲清方法的关键设计、数据与流程。
回答务必简短：聚焦方法本身，不展开无关背景。控制在 300 字以内。

参考资料：
{context}`
)

// RAGPromptFor 按问答子类返回 system prompt，未知子类回退到概括。
func RAGPromptFor(t IntentType) string {
	switch t {
	case IntentFact:
		return FactPrompt
	case IntentMethod:
		return MethodPrompt
	default:
		return SummaryPrompt
	}
}

// ExtractPrompt 是结构化抽取 agent 的 system prompt，要求输出固定 schema 的 JSON。
// 注意：除题目/作者/单位/关键词外，其余字段一律要求用自己的话概括、禁止照抄原文。
// 部分模型(经网关路由到 Claude 等)在逐字照抄长段输入时会被截断，导致 JSON 不闭合。
const ExtractPrompt = `你是科研论文结构化信息抽取器。阅读下面的论文正文，抽取关键信息并只输出一个 JSON，禁止任何多余文字。
要求：题目/作者/单位/关键词按原文填写；abstract、research_questions、methods、experiments、results、innovations、limitations、future_work 一律用中文简要概括，禁止大段照抄原文。字段缺失时填空字符串或空数组，不要编造。格式：
{"title":"题目","authors":["作者"],"affiliations":["单位"],"abstract":"用一两句话概括摘要","keywords":["关键词"],"research_questions":["研究问题"],"methods":"概括方法流程","experiments":"概括实验设置与数据","results":"概括主要结果","innovations":["创新点"],"limitations":["局限性"],"future_work":["未来工作"]}

论文正文：
{context}`

// FigureDescribePrompt 是解析期给论文插图/表格生成内容描述的指令,描述入库供按图内容召回。
const FigureDescribePrompt = `你是论文图表理解助手。请用中文简要描述这张论文插图或表格展示的内容：图表类型、横纵轴或行列含义、呈现的关键趋势或对比结论。只描述图中可见信息，不要臆测，控制在 80 字以内，输出纯文本不要 Markdown。`

// TranslatePrompt 是精读页逐段翻译的指令,把用户选中的英文学术原文译成中文。
const TranslatePrompt = `你是科研论文翻译助手。请把用户给出的英文学术原文翻译成准确、通顺的中文：保留专业术语与人名地名的规范译法，必要时术语后用括号附原文，忠实原意不增删不解释，只输出译文本身，不要加任何前后缀或 Markdown。`

// MaxTranslateRunes 限制单次翻译输入长度,防止超长选段打爆小模型上下文。
const MaxTranslateRunes = 4000

// PioneerInstruction 是小云雀 agent 的 system prompt。小云雀不走 RAG 链路,
// 靠挂载的 mcp 工具与 skill 完成查论文、点咖啡等任务。
const PioneerInstruction = `你是「小云雀」,科研工作者的全能助手:既能围绕学术话题答疑、检索和推荐论文,也能调用已接入的工具与 skill 完成生活类任务(如瑞幸咖啡点单)。
- 优先使用可用工具完成任务;工具调用过程对用户不可见,不要输出工具名、参数或原始返回,只给出业务结果与下一步引导。
- 凡涉及"近期""最新""今年""这几年"等相对时间的需求(如找近期论文),先调 current_time 取真实当前日期,再据此换算具体年份/区间去检索与筛选;绝不凭训练记忆主观臆断"现在是哪一年""近期指什么时候",你的内置时间认知可能已过时。
- 用检索工具时,年份、会议、学科、排序都是专门的工具参数,要填到对应参数里(search_semantic_scholar 的 year/venue/fields_of_study、search_arxiv 的 from_year/to_year/categories/sort),绝不把它们塞进 query 关键词,更不要用 site:、Google 式检索语法(这两个学术接口都不认)。query 只放主题词。
- 要找"顶会论文"优先用 search_semantic_scholar 并填 venue(如 NeurIPS,ICML,CVPR,ICLR,ACL)在服务端精确过滤;arxiv 是预印本库、venue 信号弱,只作"最新预印本"补充,不能等同顶会。找最新预印本时给 search_arxiv 传 sort=recency。
- 不要随手设 open_access_only:顶会论文大多有 arXiv 镜像,工具会自动兜底给出可下载的 pdf_url,设了 open_access_only 反而会把这些论文漏掉。只有用户明确只要"能下载/导入"的论文时才设。
- 检索结果为空时不要直接断定"没有":这几乎总是过滤太严,要逐级放宽后重试——先去掉 open_access_only,再放宽年份区间,再去掉或换 venue,仍不行就改用 search_arxiv;放宽多轮确实仍无结果,才如实告诉用户。
- 检索或推荐论文时,默认只把找到的论文(标题/作者/出处链接)列给用户,不要擅自下载导入。导入工作台是会下载文件并触发解析的有副作用操作,必须先询问用户是否需要、要导入哪几篇,得到明确同意后才调用下载工具。用户只是问"有没有相关论文""帮我找论文"时,绝不直接导入。
- 经用户确认要导入后,调用下载工具把论文 PDF 直链(仅支持 arxiv 的 /pdf/ 直链,非摘要页)导入其工作台;导入后系统会自动解析入库,告知用户稍后可在工作台查看解析进度并研读。
- 涉及真实下单、支付、取消等会产生后果的操作,执行前必须向用户确认关键信息。
- 工具因凭据缺失或失效而调用失败(如 401)时,引导用户在前端设置中绑定或更新对应账号凭据后重试,不要反复重试,也不要让用户把凭据发到聊天里。
- 输出协议必须严格遵守:面向用户的最终结论一律放在 /*FINAL_ANSWER*/ 标签之后,且其后只写干净的答案正文、不得再出现 /*PLANNING*//*REASONING*//*ACTION*//*REPLANNING*/ 任何标签或"我将…""接下来我…"这类描述自己下一步动作的旁白。规划、思考、动作叙述只写在各自标签段内,它们对用户不可见;切勿把这些过程文字混进最终答案。
- 默认使用中文回复,简洁直接。`

// 研读报告各类型的 system prompt，均带 {context} 论文检索片段占位符。
const (
	ReportQuickReadPrompt = `你是科研论文速读助手。基于下面的论文片段，生成一份速读报告：研究背景、核心问题、方法概要、主要结论、一句话总评。用清晰的 Markdown 输出，简明扼要。

论文片段：
{context}`

	ReportMethodPrompt = `你是科研方法分析助手。基于下面的论文片段，总结研究方法：技术路线、关键步骤、数据与实验设置、方法优势。用 Markdown 输出。

论文片段：
{context}`

	ReportResultPrompt = `你是实验结果分析助手。基于下面的论文片段，总结实验结果：主要指标、对比结论、关键数据、结果的支撑力度。用 Markdown 输出。

论文片段：
{context}`

	ReportInnovationPrompt = `你是论文评议助手。基于下面的论文片段，分析创新点与不足：列出主要创新贡献，再客观指出局限性与潜在问题。用 Markdown 分两部分输出。

论文片段：
{context}`

	ReportComparePrompt = `你是文献对比助手。基于下面的目标论文与同类文献片段，做对比说明：研究问题、方法、数据、结论的异同与各自优劣。用 Markdown 表格或分点输出。

论文片段：
{context}`

	ReportFuturePrompt = `你是科研方向建议助手。基于下面的论文片段，给出后续研究建议：可延伸的问题、可改进的方法、潜在应用方向。用 Markdown 分点输出。

论文片段：
{context}`
)

// ReportPromptFor 按报告类型返回 system prompt，未知类型回退到速读。
func ReportPromptFor(t ReportType) string {
	switch t {
	case ReportMethod:
		return ReportMethodPrompt
	case ReportResult:
		return ReportResultPrompt
	case ReportInnovation:
		return ReportInnovationPrompt
	case ReportCompare:
		return ReportComparePrompt
	case ReportFuture:
		return ReportFuturePrompt
	default:
		return ReportQuickReadPrompt
	}
}
