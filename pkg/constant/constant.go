// Package constant 集中定义全项目共享的常量与枚举。
package constant

// Provider 标识模型组件来源。
type Provider string

const (
	ProviderOllama Provider = "ollama"
	ProviderOpenAI Provider = "openai"
)

// IntentType 是意图分类结果，也用作主图 Branch 的路由键。
type IntentType string

const (
	IntentRAG   IntentType = "rag"   // 答疑
	IntentExam  IntentType = "exam"  // 出题
	IntentGrade IntentType = "grade" // 批改
)

// Valid 判断意图是否为已知类型。
func (t IntentType) Valid() bool {
	switch t {
	case IntentRAG, IntentExam, IntentGrade:
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
	StudentPartitionPrefix = "s_" // 学生私有 partition 名前缀
	TopKPerSource          = 5    // 每个检索来源带回的片段数
)

// IntentPrompt 是意图识别的 system prompt。
const IntentPrompt = `你是编程助教的意图分类器。判断学生输入属于以下哪一类，并抽取相关参数。
- rag:   学生在问知识/概念/语法/报错原因，需要查资料解释
- exam:  学生想出题/生成练习/测验/模拟卷
- grade: 学生想批改作业/评估代码/打分

只输出一个 JSON，禁止任何多余文字或解释。格式：
{"type":"rag|exam|grade","slots":{"topic":"涉及知识点","count":"题目数量","difficulty":"难度"}}`

// RAGPrompt 是答疑 agent 的 system prompt。
const RAGPrompt = `你是一位严谨的编程助教。请仅依据下面提供的「参考资料」回答学生问题；
若资料不足以回答，明确说明并给出基于通用知识的提示。回答要简洁、可操作，必要时给代码示例。

参考资料：
{context}`

// ExamPrompt 是出卷 agent 的 system prompt。
const ExamPrompt = `你是编程课出题老师。根据要求生成练习题，覆盖知识点并控制难度。
每题包含：题干、考察点、参考答案、评分要点。用清晰的 Markdown 输出。
知识点：{topic}；题目数量：{count}；难度：{difficulty}`

// GradePrompt 是批改 agent 的 system prompt。
const GradePrompt = `你是编程作业批改老师。请阅读学生提交的内容，按正确性、可读性、复杂度三个维度评分，
指出问题与改进建议，最后给出 0-100 的总分。用 Markdown 输出，先给分数再给评语。`
