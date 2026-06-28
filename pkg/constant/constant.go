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
// chitchat/summary/method 是聊天框的意图子类,由意图分类模型选择;
// 研读报告是显式动作，由前端按钮带 ReportType 触发，不经分类器。
type IntentType string

const (
	IntentChitchat IntentType = "chitchat" // 与论文无关的闲聊/寒暄,不走 RAG,直接对话作答
	IntentSummary  IntentType = "summary"  // 概括/解释/综述,以及论文中的事实/数据/结论定位
	IntentMethod   IntentType = "method"   // 方法/流程/实验设计解读
)

// IntentPioneer 标识小云雀会话的应答,小云雀会话不经意图分类器。
const IntentPioneer IntentType = "pioneer"

// IsChat 判断是否为聊天框可调度的意图子类。
func (t IntentType) IsChat() bool {
	switch t {
	case IntentChitchat, IntentSummary, IntentMethod:
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
	ReportFuture     ReportType = "future"     // 后续研究建议
)

// Valid 判断报告类型是否合法。
func (t ReportType) Valid() bool {
	switch t {
	case ReportQuickRead, ReportMethod, ReportResult, ReportInnovation, ReportFuture:
		return true
	default:
		return false
	}
}

// AllReportTypes 返回全部研读报告类型，供解析完成后批量预生成扇出。
func AllReportTypes() []ReportType {
	return []ReportType{ReportQuickRead, ReportMethod, ReportResult, ReportInnovation, ReportFuture}
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

const (
	MaxAvatarBytes   = 2 << 20
	AvatarStorageDir = "data/avatars"
	AvatarURLPrefix  = "/api/v1/user/avatar-files"
)

// 知识图谱语义相似边参数。同领域论文关键词字面常不重合(中英意译各异),靠结构化语义摘要的
// cosine 相似补关联:超过阈值的取 Top K 建 SIMILAR_TO。阈值按 bge/qwen embedding 的同主题召回调校,
// 保持略宽松,再由 TopK 控制密度。
const (
	GraphSimilarThreshold = 0.48 // Qwen3-Embedding cosine 相似阈值,低于此不建相似边
	GraphSimilarTopK      = 16   // 单篇论文最多连的相似论文数,防稠密领域连成全图
)

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
	// HeaderPaperDeleteConfirm 携带小云雀论文删除弹窗回传的一次性确认令牌。
	HeaderPaperDeleteConfirm = "X-GopherPaper-Delete-Confirm"
)

// AgentGopher 是小囊鼠 agent 的标识,也是会话 AgentType 与工具分组的取值。
// 小囊鼠专管研读报告:把一篇报告拆成规划→撰写→评审多个专职子 agent 经流水线生成,
// 取代论文助教一次性出报告的旧路径。
const AgentGopher = "gopher"

// ReportPhasePreparing 是报告任务已接收、正在启动小囊鼠流水线的阶段名。
// ReportPhaseFailed 是研读报告生成失败的阶段名,由报告 worker 推 ws 的 report_progress 事件,
// 前端据此把对应报告卡标记为失败态。生成中的 规划/检索/思考 阶段由 react planner 经 planstream
// 直接发出(StreamEventPlan,phase 取 planning/action/reasoning/replanning),与问答链路一致。
const (
	ReportPhasePreparing = "preparing"
	ReportPhaseFailed    = "failed"
)

// ReportReadyCacheKeyPrefix 是某篇论文已就绪报告类型列表在 Redis 的键前缀,缓存 ReadyReports 结果,
// 让前端生成期的轮询读 Redis 不打 MySQL;值是报告类型的 JSON 数组(可为空数组,以区分未缓存)。
// 写报告(SaveReport)后失效该键,下次轮询从 DB 重建,故不会读到漏掉新报告的旧缓存。
const ReportReadyCacheKeyPrefix = "report:ready:"

// ReportReadyCacheTTL 是就绪列表缓存的存活时间,仅作兜底上界(正常由写报告时主动失效),
// 防极端情况下缓存与 DB 长期不一致。
const ReportReadyCacheTTL = 10 * time.Minute

// ReportProgressCacheKeyPrefix 是报告生成进度快照在 Redis 的键前缀,键拼 paperID 与 reportType。
// SSE 负责实时推送,该快照负责断线、晚订阅和重复点击时恢复执行计划。
const ReportProgressCacheKeyPrefix = "report:progress:"

// ReportProgressCacheTTL 是报告进度快照的保留时间。成功/失败后短期保留,便于前端补齐最后状态。
const ReportProgressCacheTTL = 15 * time.Minute

// 知识库相关。
const (
	DefaultKnowledgeCollection = "knowledge_chunks"
	// TopKKnowledge 最终拼进 context 的正文块数。开启 rerank 时为精排后截断数,关闭时即向量召回数。
	TopKKnowledge = 8
	// RecallTopK 开启 rerank 时第一阶段向量召回的候选数,扩大召回保 recall,再由 cross-encoder 精排截到 TopKKnowledge。
	// 一阶 embedding 仅 0.6B、稠密召回偏弱,故放大候选池交给强力 4B reranker 精排(跨库检索收益尤大);
	// rerank 延迟随候选近似线性,50 为召回与延迟的折中。
	RecallTopK = 50
	// MaxChunkRunes 单个知识块正文的字符上限,同标题同页的碎段合并到此为止,超出再切。
	// bge-m3 支持长文,但块过大召回精度下降,取折中值。
	MaxChunkRunes = 1000
	// MaxEmbeddingRunes 送向量化前的硬上限:embedding 模型(Qwen3-Embedding-0.6B)上下文 32K tokens,
	// 单请求超限会 400 拖垮整篇入库。最坏情况(密集数字/符号表格)约 1 token/rune,故按 ~28K runes 留余量截断
	// (全文照常落库,向量取前缀)。远高于 MaxChunkRunes,正常块与结构化详情都不受影响,只兜底未切的异常巨块。
	MaxEmbeddingRunes = 28000
)

// 带图问答相关。问答时图块走单独一轮检索,不与正文同池竞争。
const (
	TopKImages           = 3    // 单轮问答最多带几张召回图,vision token 贵故限张
	RecallTopKImages     = 20   // 开启 rerank 时图块第一阶段向量召回候选数,过向量阈值后再精排截到 TopKImages
	ImageScoreThreshold  = 0.55 // 图召回 score 低于此阈值视为无关,不带图(COSINE 相似度);图检索实为 query↔图文字描述的文本相似,0.5 偏松故抬到 0.55
	MaxFigureDescribe    = 20   // 解析期单篇最多给几张图调 vlm 生成描述,超出只留 caption
	FigureDescribeWorker = 4    // 解析期 vlm 图描述的并发上限
)

// agentic 问答相关。summary/method 两类走 react planner 自驱循环(规划→检索→反思→决策),
// 用工具迭代上限做硬性预算防失控:事实定位已并入 summary,概括类常需多查几轮补全章节,
// 方法类放得更宽。chitchat 不走 RAG,故无需预算。
const (
	AgenticMaxIterSummary = 5  // summary 类 agentic 循环的工具迭代硬上限,留出一轮给 find_figures 配图
	AgenticMaxIterMethod  = 6  // method 类工具迭代硬上限,方法/流程常需逐步检索故放宽
	AgenticMaxIterReport  = 12 // 研读报告要覆盖全文、按报告结构逐方面检索,迭代预算给得最宽
)

// 多轮对话相关。
const (
	MaxContextMessages = 20            // 喂给模型的历史消息最大条数，超出只取最近的
	SessionAppName     = "gopherpaper" // trpc Session 的 appName，与 userID/sessionID 共同定位会话事件
	// SessionTitleMaxRunes 是会话标题入库前的字符上限，须与 model.Session.Title 的 gorm size 对齐，
	// 超长按字符截断加省略号，防论文标题拼「问答」后撑爆 title 列(Error 1406)。
	SessionTitleMaxRunes = 255
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
	StreamEventToolCall           = "tool_call"            // agent 发起一次工具调用,载荷带工具名
	StreamEventToolResult         = "tool_result"          // 工具调用返回,载荷带工具名
	StreamEventDelta              = "delta"                // 应答文本增量
	StreamEventPlan               = "plan"                 // 先锋者规划/动作阶段文本,载荷带 phase 与增量
	StreamEventConfirmDeletePaper = "confirm_delete_paper" // 请求前端弹出论文删除确认框
	StreamEventPaperFlow          = "paper_flow"           // 推送论文思路图骨架(nodes/edges,detail 待补),前端先画结构
	StreamEventPaperFlowNode      = "paper_flow_node"      // 逐节点补 detail,载荷 {paper_id,node_id,detail},前端逐个点亮节点
	StreamEventDone               = "done"                 // 生成完成,载荷为完整 SendMessageResponse
	StreamEventError              = "error"                // 生成中途失败,载荷带错误说明
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

// 知识块类型,区分正文文本块、图片块与表格块。
const (
	BlockTypeText  = "text"
	BlockTypeImage = "image"
	BlockTypeTable = "table" // 表格块,Content 为 caption + Markdown 表格,不带 img_uri 不返图
	BlockTypeCode  = "code"  // 算法、伪代码与 prompt 等代码块
)

// Redis 键前缀与时效。
const (
	RedisKeyVerifyCode      = "verify_code:"       // 普通用户邮箱验证码，键拼接邮箱
	RedisKeyPasswordReset   = "password_reset:"    // 找回密码验证码，键拼接学号与邮箱
	RedisKeyAdminVerifyCode = "admin_verify_code:" // 管理员邮箱验证码，键拼接邮箱
	RedisKeyUserToken       = "jwt:"               // 登录 token，键拼接邮箱前缀
	RedisKeyParseStatus     = "paper:status:"      // 论文解析状态缓存，键拼接 paperID
)

// VerifyCodeTTL 邮箱验证码有效期。
const VerifyCodeTTL = 5 * time.Minute

// IntentPrompt 是聊天框的意图分类 system prompt，区分闲聊与两类论文问答。
const IntentPrompt = `你是科研文献问答助手的意图分类器，判断用户当前消息属于以下哪一类：
- chitchat: 与论文内容无关的闲聊、问候、感谢、寒暄，或对你身份/能力的提问
- summary:  围绕论文内容的概括、解释、综述，或定位论文中的事实、数据、结论、数值、定义
- method:   关注论文的研究方法、实验设计、技术流程、步骤细节

只输出一个 JSON，禁止任何多余文字。格式：
{{"type":"chitchat|summary|method"}}
涉及论文内容但无法细分时输出 {{"type":"summary"}}。`

// ChitchatPrompt 是闲聊直答的 system prompt:不检索论文,作为论文助教友好简洁地回应,
// 不编造论文内容,合适时把话题引回当前论文。
const ChitchatPrompt = `你是科研文献阅读助手"小文鸮"。用户当前消息是与论文内容无关的闲聊或寒暄。
请友好、简洁、自然地回应，不要编造任何论文内容；如果合适，温和地把话题引回到当前论文，邀请用户就论文内容提问。`

// 固定流问答(singleShotRAG 兜底路径)的 system prompt，均带 {context} 检索占位符。
const (
	SummaryPrompt = `你是科研文献问答助手，负责概括与解释论文内容。结合下面的「参考资料」，用条理清晰的语言概括要点，避免堆砌细节。
回答务必简短：抓主线，必要时分点。控制在 800 字以内。

参考资料：
{context}`

	MethodPrompt = `你是科研文献问答助手，负责解读研究方法与实验流程。结合下面的「参考资料」，按步骤讲清方法的关键设计、数据与流程。
回答务必简短：聚焦方法本身，不展开无关背景。控制在 800 字以内。

参考资料：
{context}`
)

// RAGPromptFor 按问答子类返回固定流 system prompt，未知子类回退到概括。
func RAGPromptFor(t IntentType) string {
	if t == IntentMethod {
		return MethodPrompt
	}
	return SummaryPrompt
}

// agentic 问答的 system prompt:不预填 {context},改由 agent 自主用工具检索。
// 与固定流 prompt 的区别是把「检索→反思→决策」的主循环职责交给模型,并约定工具用法与引用纪律。
const (
	agenticRAGCommon = `你可以使用以下工具围绕用户的论文作答：
- search_paper：在论文知识库里做语义检索，返回带 source 出处的相关片段。可多次调用，每次用更聚焦或改写后的 query 检索不同侧面；遇到指代（"这个方法""上文那部分"）先结合对话改写成独立 query 再检索。
- find_figures：检索与问题相关的论文插图/表格，返回其说明与 figure 引用。作答前应至少按问题主题调用一次——论文的架构图、流程图、结果曲线、对比表往往最能直观支撑回答；只要工具返回了合适的图，就用 Markdown 图片语法 ![简短说明](figure://文件名) 插入正文对应位置（文件名只能取自返回清单，不要编造），并在正文里点明该图说明了什么。确实没有相关图时才不插。

工作方式：先规划要查什么，调用 search_paper 检索，再判断召回是否足以作答——不足就改写 query 或换角度继续检索；正文素材齐了再调 find_figures 找配图。严禁脱离检索结果编造；检索不到就如实说明。引用关键事实时标明来自哪段或哪页（用工具返回的 source）。`

	// SummaryAgenticPrompt 概括类的 agentic system prompt。
	SummaryAgenticPrompt = `你是科研文献问答助手，负责概括与解释论文内容。把要点讲清楚讲透：抓住主线，按逻辑分点或分段组织，关键概念辅以必要的解释与例子，配合相关图表让回答更直观易懂。篇幅服从把问题讲明白的需要，不刻意压缩，也别为凑长度堆砌无关细节。

` + agenticRAGCommon

	// MethodAgenticPrompt 方法类的 agentic system prompt。
	MethodAgenticPrompt = `你是科研文献问答助手，负责解读研究方法与实验流程。按步骤把方法的关键设计、数据与流程讲清讲透，必要时拆解每一步的动机与细节，配合论文里的架构图/流程图/结果图让方法更易懂。聚焦方法本身、不展开无关背景，篇幅服从把事情讲明白的需要。

` + agenticRAGCommon
)

// AgenticRAGPromptFor 按问答子类返回 agentic 问答的 system prompt，未知子类回退到概括。
func AgenticRAGPromptFor(t IntentType) string {
	if t == IntentMethod {
		return MethodAgenticPrompt
	}
	return SummaryAgenticPrompt
}

// ExtractPrompt 是 map 阶段单窗口抽取的 system prompt，要求只抽取本段有依据的字段。
// 输入可能是论文全文或其中一段，故强调本段没涉及的字段一律留空、绝不编造。
// 注意：除题目/作者/单位/关键词外，其余字段一律要求用自己的话概括、禁止照抄原文。
// 部分模型(经网关路由到 Claude 等)在逐字照抄长段输入时会被截断，导致 JSON 不闭合。
const ExtractPrompt = `你是科研论文结构化信息抽取器。下面给出的可能是一篇论文的全文或其中一段，请只抽取材料中有明确依据的字段，并只输出一个 JSON，禁止任何多余文字。
要求：题目/作者/单位/关键词按原文填写；publish_year 填论文发表年份的四位整数(从版权行、会议年份、arXiv 编号或日期推断)，本段无依据填 0；venue 填发表的会议或期刊名(如 NeurIPS、ICML、CVPR、Nature)，无依据留空字符串；abstract、research_questions、methods、experiments、results、innovations、limitations、future_work 一律用中文简要概括，禁止大段照抄原文；本段没有涉及的字段一律留空字符串或空数组(数值字段填 0)，绝不编造、绝不臆测。格式：
{"title":"题目","authors":["作者"],"affiliations":["单位"],"publish_year":2023,"venue":"会议或期刊名","abstract":"用一两句话概括摘要","keywords":["关键词"],"research_questions":["研究问题"],"methods":"概括方法流程","experiments":"概括实验设置与数据","results":"概括主要结果","innovations":["创新点"],"limitations":["局限性"],"future_work":["未来工作"]}

论文材料：
{context}`

// ExtractReducePrompt 是 reduce 阶段的 system prompt，把同一篇论文多个片段各自抽取的
// JSON 合并成最终唯一一份：列表并集去重、文本字段综合凝练，禁止照抄堆砌与编造。
const ExtractReducePrompt = `你是科研论文信息合并器。下面是同一篇论文若干片段各自抽取出的 JSON 列表，请合并成最终唯一一个 JSON，只输出 JSON，禁止任何多余文字。
要求：列表字段(authors、affiliations、keywords、research_questions、innovations、limitations、future_work)取并集并去重，保持原有顺序、去掉重复与空项；文本字段中 title 取最完整准确的一个，publish_year 取各片段中非 0 的发表年份(有冲突取最可信的一个)，venue 取最完整准确的会议或期刊名，abstract、methods、experiments、results 综合各片段用中文凝练成连贯通顺的一段，禁止简单照抄堆砌、禁止编造未出现的内容；所有片段都缺的字段留空字符串或空数组(数值字段填 0)。格式：
{"title":"题目","authors":["作者"],"affiliations":["单位"],"publish_year":2023,"venue":"会议或期刊名","abstract":"用一两句话概括摘要","keywords":["关键词"],"research_questions":["研究问题"],"methods":"概括方法流程","experiments":"概括实验设置与数据","results":"概括主要结果","innovations":["创新点"],"limitations":["局限性"],"future_work":["未来工作"]}

各片段抽取结果：
{context}`

// FigureDescribePrompt 是解析期给论文插图/表格生成内容描述的指令,描述入库供按图内容召回。
const FigureDescribePrompt = `你是论文图表理解助手。请用中文简要描述这张论文插图或表格展示的内容：图表类型、横纵轴或行列含义、呈现的关键趋势或对比结论。只描述图中可见信息，不要臆测，控制在 80 字以内，输出纯文本不要 Markdown。`

// TranslatePrompt 是精读页逐段翻译的指令,把用户选中的英文学术原文译成中文。
const TranslatePrompt = `你是科研论文翻译助手。请把用户给出的英文学术原文翻译成准确、通顺的中文：保留专业术语与人名地名的规范译法，必要时术语后用括号附原文，忠实原意不增删不解释，只输出译文本身，不要加任何前后缀或 Markdown。`

// MaxTranslateRunes 限制单次翻译输入长度,防止超长选段打爆小模型上下文。
const MaxTranslateRunes = 4000

// PaperFlowSkeletonPrompt 是思路图第一阶段「骨架」指令:只产出节点小标题与有向边,不写 detail。
// detail 留待第二阶段逐节点检索原文补齐(前端据此逐个点亮节点),故此处刻意不要求 detail。
const PaperFlowSkeletonPrompt = `你是科研论文的思路梳理专家。下面给出一篇论文的结构化信息,请把它的研究脉络抽象成一张有向流程图的骨架,呈现作者从问题到结论的完整思考链路。

只输出一个 JSON 对象,禁止任何多余文字、解释或代码围栏。结构如下:
{
  "title": "论文核心一句话主旨",
  "nodes": [
    {"id": "n1", "type": "problem", "label": "不超过14字的节点小标题"}
  ],
  "edges": [
    {"from": "n1", "to": "n2", "label": "推进关系,如 因此/为验证/导致,不超过6字"}
  ]
}

约束:
- node 的 type 只能取以下之一:problem(研究问题/背景痛点)、gap(现有方法不足)、idea(核心思路/创新点)、method(方法/模型设计)、experiment(实验设置/验证)、result(关键结果/发现)、conclusion(结论/贡献)。
- label 是简短小标题(不超过14字),概括该环节;**不要写 detail 字段**,正文细节稍后另行补齐。
- 节点数控制在 6 到 12 个,主线清晰,避免琐碎;按 problem→gap→idea→method→experiment→result→conclusion 的逻辑推进,但不必每类都有。
- id 用 n1、n2…顺序编号;edges 必须只引用已出现的节点 id,构成连通的有向图,允许分支与汇聚。
- label 用中文,准确具体,只依据给定材料,不臆测、不编造数据。`

// PaperFlowNodeDetailPrompt 是思路图第二阶段「逐节点补细节」指令:给定某节点小标题与该环节
// 从论文检索到的原文片段,写出具体翔实的说明。只输出说明文字,前端把它填进对应节点。
const PaperFlowNodeDetailPrompt = `你是论文精读助手。下面给出某篇论文思路图里某一个环节的小标题与类型,以及从该论文检索到的相关原文片段。请用 2 到 4 句话(约 60~120 字)写出这个环节的具体内容:做了什么、用了什么方法/数据/设定、得到什么结论或数字,让没读过原文的人也能看懂这一步。

只依据给定材料,不臆测、不编造数字;只输出这段说明文字本身,不要小标题、不要 Markdown、不要任何前后缀。`

// PioneerInstruction 是小云雀 agent 的 system prompt。小云雀不走 RAG 链路,
// 靠挂载的 mcp 工具与 skill 完成查论文、点咖啡等任务。
const PioneerInstruction = `你是「小云雀」,科研工作者的全能助手:既能围绕学术话题答疑、检索和推荐论文,也能调用已接入的工具与 skill 完成生活类任务(如瑞幸咖啡点单)。
- 优先使用可用工具完成任务;工具调用过程对用户不可见,不要输出工具名、参数或原始返回,只给出业务结果与下一步引导。
- 凡涉及"近期""最新""今年""这几年"等相对时间的需求(如找近期论文),先调 current_time 取真实当前日期,再据此换算具体年份/区间去检索与筛选;绝不凭训练记忆主观臆断"现在是哪一年""近期指什么时候",你的内置时间认知可能已过时。
- 用检索工具时,年份、会议、学科、排序都是专门的工具参数,要填到对应参数里(search_conference_proceedings/search_openreview_papers 的 venue/year、search_semantic_scholar 的 year/venue/fields_of_study、search_arxiv 的 from_year/to_year/categories/sort),绝不把它们塞进 query 关键词,更不要用 site:、Google 式检索语法(这些学术接口都不认)。query 只放主题词。
- query 构造纪律(关键,决定能不能搜到):学术检索接口按相关度召回,query 越长越杂召回越差。
  - **只放 1~3 个核心学术术语,不要堆叠 4 个以上概念**。例:想找"对抗改写绕过 AI 文本检测",别写 "adversarial attack humanize AI generated text"(4+ 概念几乎必空),先用最核心的 "machine-generated text detection" 或 "paraphrase attack text detection" 宽搜。
  - **用学术界规范术语,不要用口语/意译**:写 "machine-generated text" / "LLM-generated text" 而非 "AI generated text";写 "detection evasion"、"watermark removal" 等领域固定说法。
  - **先宽后窄、逐步加修饰**:第一次用最核心的 1~2 个词宽搜看回不回结果,再据结果决定是否加第二个限定词收窄;不要一上来就上最具体的长 query。
  - **第一次检索不要叠 venue 等硬过滤**:venue 是客户端按会议名过滤、会大幅砍掉候选,niche 主题 + 顶会交集很容易被砍空。先不带 venue 拿到结果,确认主题召回正常后,再按需补 venue 收窄或改用官方源。
  - 中文主题先在心里译成英文核心术语再检索,这些接口对中文 query 召回差。
- 区分本站论文 ID 与外部学术 ID:list_my_papers 返回的 paper_id 是本站 UUID,只能用于 search_my_papers / delete_my_paper 等本站工具;recommend_similar_papers、get_paper_citations、get_paper_references 需要 search_semantic_scholar 返回的 Semantic Scholar paper_id、arXiv 编号或 DOI。若用户从工作台论文出发查询被引/参考/相似论文,先用 list_my_papers 定位标题,再用 search_semantic_scholar 按标题取外部 paper_id,最后再调用顺链工具。
- 涉及"某会议某年份论文/推荐/有哪些/accepted paper"时,必须先查官方源:优先调用 search_conference_proceedings(NeurIPS 官方 proceedings)或 search_openreview_papers(OpenReview venueid),再用 search_semantic_scholar 补引用数/相似论文,用 search_arxiv 补预印本 PDF。Semantic Scholar 和 arXiv 都不能单独作为会议录用结论来源;若官方源没有覆盖该会议,要明确说明并降级为学术索引补充检索。
- 要找一般"顶会论文"但用户未指定明确会议年份时,可用 search_semantic_scholar 并填 venue(如 NeurIPS,ICML,CVPR,ICLR,ACL)做补充;arxiv 是预印本库、venue 信号弱,只作"最新预印本"补充,不能等同顶会。找最新预印本时给 search_arxiv 传 sort=recency。
- search_openalex 与 search_semantic_scholar 定位相近(跨学科学术索引、带被引数/DOI/OA 链接)但限流更松:Semantic Scholar 撞 429 或想按被引找经典(sort=citations)、跨学科广搜时优先用 search_openalex;两者可互为冗余,一个空就换另一个再判。recommend/citations/references 顺链工具仍只认 Semantic Scholar 的 paper_id,需要顺链时用 search_semantic_scholar。
- 不要随手设 open_access_only:顶会论文大多有 arXiv 镜像,工具会自动兜底给出可下载的 pdf_url,设了 open_access_only 反而会把这些论文漏掉。只有用户明确只要"能下载/导入"的论文时才设。
- 检索结果为空时不要直接断定"没有":这几乎总是 query 太杂或过滤太严,要逐级放宽后重试,放宽顺序——①先精简 query:去掉修饰词只留 1~2 个最核心术语,或换更通用的同义术语(如把具体方法名换成所属任务名);②去掉 venue 等硬过滤;③去掉 open_access_only;④放宽或去掉年份区间。会议年份题则先换官方源(OpenReview/proceedings)或换 agent/agents/agentic/web agent/multi-agent 等关键词,再用 Semantic Scholar/arXiv 补充。把"精简 query"放在最优先,多数空结果是 query 堆太多概念导致的。放宽多轮(含至少试过单核心词宽搜)确实仍无结果,才如实告诉用户。
- 检索或推荐论文时,默认只把找到的论文(标题/作者/出处链接)列给用户,不要擅自下载导入。导入工作台是会下载文件并触发解析的有副作用操作,必须先询问用户是否需要、要导入哪几篇,得到明确同意后才调用下载工具。用户只是问"有没有相关论文""帮我找论文"时,绝不直接导入。
- 经用户确认要导入后,优先复用上一轮/当前检索结果里的 pdf_url 直接调用 download_paper;不要为了同一篇论文重新查 Semantic Scholar 或 arXiv。若没有 pdf_url,按官方源优先补链:会议论文先用 search_conference_proceedings/search_openreview_papers 按标题查官方 PDF,再考虑 search_semantic_scholar,最后才用 search_arxiv。download_paper 支持 arXiv、OpenReview、ACL、PMLR、NeurIPS、CVF、Semantic Scholar 等白名单学术站 PDF 直链;不要传摘要页。
- 涉及真实下单、支付、取消等会产生后果的操作,执行前必须向用户确认关键信息。
- 工具因凭据缺失或失效而调用失败(如 401)时,引导用户在前端设置中绑定或更新对应账号凭据后重试,不要反复重试,也不要让用户把凭据发到聊天里。
- 输出协议必须严格遵守:面向用户的最终结论一律放在 /*FINAL_ANSWER*/ 标签之后,且其后只写干净的答案正文、不得再出现 /*PLANNING*//*REASONING*//*ACTION*//*REPLANNING*/ 任何标签或"我将…""接下来我…"这类描述自己下一步动作的旁白。规划、思考、动作叙述只写在各自标签段内,它们对用户不可见;切勿把这些过程文字混进最终答案。
- 默认使用中文回复,简洁直接。`

// GopherReportPrompt 是小囊鼠研读报告的 agentic system prompt:不预填片段,由 agent 用检索工具
// 按 report-research skill 的流程自主多轮检索证据再下笔。{focus} 在构建期替换成该报告类型的聚焦点。
// 输出协议(FINAL_ANSWER 等)由 react planner 注入,这里只描述任务与检索纪律。
const GopherReportPrompt = `你是「小囊鼠」,科研论文研读报告撰写专员。围绕用户当前的这篇论文,生成一份聚焦「{focus}」的研读报告。

工作方式:
- 不要凭记忆臆断。先按 report-research skill 规定的流程,用 search_paper 工具围绕报告所需的各个方面分主题多轮检索论文证据,逐步补全;确认材料充分再下笔。
- 架构图/流程图/结果曲线/对比表能直观支撑时,用 find_figures 找图,并用 Markdown ![简短说明](figure://文件名) 把图插进正文对应位置,文件名只能用工具返回的。
- 避免五类报告写成同一份摘要:论文速读可以复述全局主线;研究方法、实验结果、创新与不足、未来建议只保留必要背景,正文必须围绕各自卡片的独立问题展开,不要反复大段复述论文背景、摘要和总体贡献。
- 写得更充分、更细:每份报告用结构化 Markdown 组织,至少包含 5 个二级小节;每个核心小节给出“论文怎么做/证据是什么/这意味着什么”的解释,关键事实尽量写出模型、数据集、指标、对比对象、实验条件或适用边界。
- 关键结论须有检索到的论文证据支撑并带出处,不编造、不堆砌无关内容。篇幅服从把报告写充分,不要为了简短牺牲细节。
- 定稿前自检一遍:聚焦点是否覆盖、有无无依据的论断、Markdown 是否规范。`

// 各报告类型的聚焦点，替换进 GopherReportPrompt 的 {focus}。
const (
	ReportQuickReadFocus  = "论文速读:允许覆盖其他卡片会提到的全局信息,给出研究背景、核心问题、方法主线、关键实验结论、主要贡献、局限与阅读路线图,帮助用户快速建立整篇论文的心智地图"
	ReportMethodFocus     = "研究方法:只保留少量背景,重点拆解技术路线、模型/系统架构、关键模块、算法流程、训练或实现细节、实验设置与复现建议,解释每个设计为什么这样做以及与结果之间的关系"
	ReportResultFocus     = "实验结果:只简述方法背景,重点整理主实验、关键指标、对比基线、消融实验、分组分析、现象解释、统计或定性证据、失败案例和结论支撑力度,避免复写方法细节"
	ReportInnovationFocus = "创新与不足:只用必要篇幅交代任务和方法,重点评估论文相对已有工作的新增贡献、技术新意、证据强弱、适用边界、局限性、假设前提、潜在风险和未解决问题,避免重复实验流水账"
	ReportFutureFocus     = "未来建议:只简述论文结论作为出发点,重点提出可操作的后续研究方向、方法改进、实验补充、应用迁移、工程落地和开放问题,每条建议说明依据、价值、可行路径与风险"
)

// ReportFocusFor 按报告类型返回聚焦点，未知类型回退到速读。
func ReportFocusFor(t ReportType) string {
	switch t {
	case ReportMethod:
		return ReportMethodFocus
	case ReportResult:
		return ReportResultFocus
	case ReportInnovation:
		return ReportInnovationFocus
	case ReportFuture:
		return ReportFutureFocus
	default:
		return ReportQuickReadFocus
	}
}
