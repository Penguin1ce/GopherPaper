// toolkit tavily.go 是 Tavily 联网搜索 function tool:把模型知识截止之外的实时信息查回来,
// 供小云雀回答"今天发生了什么""最新版本是多少"这类离线知识答不了的问题。
package toolkit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"trpc.group/trpc-go/trpc-agent-go/tool"
	"trpc.group/trpc-go/trpc-agent-go/tool/function"
)

// tavilyBaseURL Tavily 搜索端点,测试时可替换。
var tavilyBaseURL = "https://api.tavily.com/search"

var tavilyClient = &http.Client{Timeout: 20 * time.Second}

type tavilyInput struct {
	Query string `json:"query" jsonschema:"description=搜索关键词或问题,用自然语言描述,如:2026 年诺贝尔物理学奖得主,required"`
	Deep  bool   `json:"deep,omitempty" jsonschema:"description=是否深度搜索,需要更全面准确时置 true(更慢更耗额度),一般留空走快速搜索"`
}

type tavilyResult struct {
	Title   string  `json:"title" jsonschema:"description=结果标题"`
	URL     string  `json:"url" jsonschema:"description=来源链接"`
	Content string  `json:"content" jsonschema:"description=正文摘要"`
	Score   float64 `json:"score,omitempty" jsonschema:"description=相关度,越大越相关"`
}

type tavilyOutput struct {
	Answer  string         `json:"answer,omitempty" jsonschema:"description=Tavily 综合各来源给出的直接回答,可作答案基础但需结合 results 出处核对"`
	Results []tavilyResult `json:"results" jsonschema:"description=检索到的网页结果列表,引用时附上 url 出处"`
}

// tavilyReq 是 Tavily search 的请求体,只暴露我们用到的字段。
type tavilyReq struct {
	Query         string `json:"query"`
	SearchDepth   string `json:"search_depth"`
	IncludeAnswer bool   `json:"include_answer"`
	MaxResults    int    `json:"max_results"`
}

// tavilyResp 是 Tavily search 的响应,失败时 detail 带错误文案。
type tavilyResp struct {
	Answer  string          `json:"answer"`
	Results []tavilyResult  `json:"results"`
	Detail  json.RawMessage `json:"detail"`
}

// newTavilyTool 构建 Tavily 联网搜索工具。默认走 basic 深度(1 额度、更快),
// 模型显式要深度时切 advanced;带 include_answer 让 Tavily 先给一句综合回答省一轮推理。
func newTavilyTool(apiKey string) tool.Tool {
	fn := func(ctx context.Context, in tavilyInput) (tavilyOutput, error) {
		return tavilySearch(ctx, apiKey, in)
	}
	return function.NewFunctionTool(fn,
		function.WithName("web_search"),
		function.WithDescription("联网搜索实时信息。凡涉及近期事件、最新数据、版本号、价格行情等模型知识截止后可能变化的内容,先调本工具再回答,并在答案里附上来源链接。"),
	)
}

func tavilySearch(ctx context.Context, apiKey string, in tavilyInput) (tavilyOutput, error) {
	depth := "basic"
	if in.Deep {
		depth = "advanced"
	}
	body, err := json.Marshal(tavilyReq{
		Query:         in.Query,
		SearchDepth:   depth,
		IncludeAnswer: true,
		MaxResults:    5,
	})
	if err != nil {
		return tavilyOutput{}, fmt.Errorf("web_search: 序列化请求失败: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tavilyBaseURL, bytes.NewReader(body))
	if err != nil {
		return tavilyOutput{}, fmt.Errorf("web_search: 构建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := tavilyClient.Do(req)
	if err != nil {
		return tavilyOutput{}, fmt.Errorf("web_search: 请求 Tavily 失败: %w", err)
	}
	defer resp.Body.Close()

	var r tavilyResp
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return tavilyOutput{}, fmt.Errorf("web_search: 解析响应失败: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return tavilyOutput{}, fmt.Errorf("web_search: Tavily 返回 %d: %s", resp.StatusCode, r.Detail)
	}
	return tavilyOutput{Answer: r.Answer, Results: r.Results}, nil
}
