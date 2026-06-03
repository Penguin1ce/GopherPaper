// Package constant 集中定义全项目共享的常量与枚举。
package constant

import "time"

// Provider 标识模型组件来源。
type Provider string

const (
	ProviderOllama Provider = "ollama"
	ProviderOpenAI Provider = "openai"
)

// IntentType 标识一次请求的处理路径。
// concept/debug/review 是聊天框的答疑子类，由 Host 调度；
// exam/grade 是显式动作，由前端按钮触发，不经分类器。
type IntentType string

const (
	IntentConcept IntentType = "concept" // 概念/语法讲解
	IntentDebug   IntentType = "debug"   // 报错调试
	IntentReview  IntentType = "review"  // 代码评审

	IntentExam  IntentType = "exam"  // 出题
	IntentGrade IntentType = "grade" // 批改
)

// IsChat 判断是否为聊天框可调度的答疑子类。
func (t IntentType) IsChat() bool {
	switch t {
	case IntentConcept, IntentDebug, IntentReview:
		return true
	default:
		return false
	}
}

// 主图节点名。
const (
	NodePrepare     = "prepare"
	NodeIntentTpl   = "intent_tpl"
	NodeIntentModel = "intent_model"
	NodeParseIntent = "parse_intent"
)

// HTTP 鉴权相关。
const (
	HeaderAuthorization = "Authorization"
	BearerPrefix        = "bearer "
)

// 知识库相关。
const (
	DefaultKnowledgeCollection = "knowledge_chunks"
	TopKKnowledge              = 8
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
)

// Redis 键前缀与时效。
const (
	RedisKeyVerifyCode = "verify_code:" // 邮箱验证码，键拼接邮箱
	RedisKeyUserToken  = "jwt:"         // 登录 token，键拼接邮箱前缀
)

// VerifyCodeTTL 邮箱验证码有效期。
const VerifyCodeTTL = 5 * time.Minute

// IntentPrompt 是意图识别的 system prompt，只在聊天框答疑子类间分类。
const IntentPrompt = `你是编程助教的意图分类器，判断学生提问属于以下哪一类答疑：
- concept: 询问概念、语法、原理、用法等知识性问题
- debug:   贴出报错或异常，想定位并修复 bug
- review:  贴出可运行代码，想要评审、优化、改进建议

只输出一个 JSON，禁止任何多余文字。格式：
{{"type":"concept|debug|review"}}
无法判断时输出 {{"type":"concept"}}。`

// 三类答疑 agent 的 system prompt，均带 {context} 检索占位符。
const (
	ConceptPrompt = `你是一位严谨的编程助教，负责讲解概念与语法。请优先依据下面的「参考资料」作答，
资料不足时基于通用知识补充并说明。讲清原理，配最小可运行示例，避免空泛。

参考资料：
{context}`

	DebugPrompt = `你是一位编程助教，负责帮学生定位并修复报错。请结合下面的「参考资料」，
先指出错误原因，再给出修正后的代码与验证方法，必要时点明易错点。

参考资料：
{context}`

	ReviewPrompt = `你是一位资深代码评审者。请结合下面的「参考资料」，从正确性、可读性、
性能、风格四方面点评学生代码，给出可直接采纳的改进建议与示例。

参考资料：
{context}`

	HostPrompt = `你是 GopherCPP 编程助教的 Host 调度器。你的唯一任务是判断学生请求应该交给哪个专家处理。

必须遵守：
- 必须且只能调用一个最合适的工具，不要直接回答用户。
- 不要把同一个请求拆给多个专家。
- 学生想理解概念、语法、原理、用法时，调用 concept_tutor。
- 学生贴出报错、异常、崩溃、编译失败、运行结果不对时，调用 debug_tutor。
- 学生要求评审、优化、重构、改进代码时，调用 code_reviewer。`
)

// RAGPromptFor 按答疑子类返回 system prompt，未知子类回退到概念讲解。
func RAGPromptFor(t IntentType) string {
	switch t {
	case IntentDebug:
		return DebugPrompt
	case IntentReview:
		return ReviewPrompt
	default:
		return ConceptPrompt
	}
}

// ExamPrompt 是出卷 agent 的 system prompt。
const ExamPrompt = `你是编程课出题老师。根据要求生成练习题，覆盖知识点并控制难度。
每题包含：题干、考察点、参考答案、评分要点。用清晰的 Markdown 输出。
知识点：{topic}；题目数量：{count}；难度：{difficulty}`

// GradePrompt 是批改 agent 的 system prompt。
const GradePrompt = `你是编程作业批改老师。请阅读学生提交的内容，按正确性、可读性、复杂度三个维度评分，
指出问题与改进建议，最后给出 0-100 的总分。用 Markdown 输出，先给分数再给评语。`
