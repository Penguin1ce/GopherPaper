package ai

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
	hostma "github.com/cloudwego/eino/flow/agent/multiagent/host"
	"github.com/cloudwego/eino/schema"

	localagent "GopherCPP/internal/agent"
	"GopherCPP/internal/config"
	"GopherCPP/internal/factory"
	"GopherCPP/pkg/constant"
)

func TestHostRoutesToExpectedSpecialist(t *testing.T) {
	ctx := context.Background()
	ragRunner := &recordingRunner{}

	host, err := buildChat(ctx, &routingChatModel{}, ragRunner)
	if err != nil {
		t.Fatalf("build host: %v", err)
	}

	cases := []struct {
		name  string
		query string
		want  constant.IntentType
	}{
		{
			name:  "concept",
			query: "C++ 引用和指针有什么区别？",
			want:  constant.IntentConcept,
		},
		{
			name:  "debug",
			query: "这段代码运行时报 Segmentation fault，帮我定位一下",
			want:  constant.IntentDebug,
		},
		{
			name:  "review",
			query: "帮我评审并优化这段 C++ 排序代码",
			want:  constant.IntentReview,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			out, err := host.Generate(ctx, []*schema.Message{schema.UserMessage(tt.query)})
			if err != nil {
				t.Fatalf("generate: %v", err)
			}

			reply := messageToReply(out)
			if reply.Intent != tt.want {
				t.Fatalf("intent mismatch: want %q got %q content=%q", tt.want, reply.Intent, reply.Content)
			}
			if !strings.Contains(reply.Content, string(tt.want)) {
				t.Fatalf("reply content did not come from expected specialist: %q", reply.Content)
			}
		})
	}

	for _, want := range []constant.IntentType{
		constant.IntentConcept,
		constant.IntentDebug,
		constant.IntentReview,
	} {
		if got := totalCalls(want, ragRunner); got != 1 {
			t.Fatalf("specialist %q call count mismatch: want 1 got %d", want, got)
		}
	}
}

func TestHostClassifyWithAPIModel(t *testing.T) {
	//if os.Getenv("GOPHERCPP_RUN_API_TESTS") != "1" {
	//	t.Skip("跳过：设置 GOPHERCPP_RUN_API_TESTS=1 后才真实调用 API 模型测试 Host 分类")
	//}

	cfg, err := config.Load("../../config/config.toml")
	if err != nil {
		t.Skipf("跳过：读取配置失败 %v", err)
	}
	if cfg.Models.Chat.Provider != constant.ProviderOpenAI {
		t.Skipf("跳过：chat 模型不是 API provider，当前 %q", cfg.Models.Chat.Provider)
	}
	if cfg.Models.Chat.APIKey == "" {
		t.Skip("跳过：未配置 chat 模型 api_key")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	chatModel, err := factory.NewModelFactory(cfg).NewChatModel(ctx)
	if err != nil {
		t.Fatalf("创建 API chat 模型失败: %v", err)
	}

	ragRunner := &recordingRunner{}
	host, err := buildChat(ctx, chatModel, ragRunner)
	if err != nil {
		t.Fatalf("build host: %v", err)
	}

	cases := []struct {
		name  string
		query string
		want  constant.IntentType
	}{
		{
			name:  "concept",
			query: "C++ 里的引用和指针有什么区别？只需要选择合适专家处理。",
			want:  constant.IntentConcept,
		},
		{
			name:  "debug",
			query: "我的 C++ 程序运行时报 Segmentation fault，请选择合适专家处理。",
			want:  constant.IntentDebug,
		},
		{
			name:  "review",
			query: "请评审并优化这段 C++ 排序代码，只需要选择合适专家处理。",
			want:  constant.IntentReview,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			cb := &handoffRecorder{}
			out, err := host.Generate(ctx, []*schema.Message{schema.UserMessage(tt.query)}, hostma.WithAgentCallbacks(cb))
			if err != nil {
				t.Fatalf("generate: %v", err)
			}
			reply := messageToReply(out)
			if handoff := cb.last(); handoff != nil {
				t.Logf("Host 选择: specialist=%s argument=%s reply_intent=%q", handoff.ToAgentName, handoff.Argument, reply.Intent)
			} else {
				t.Logf("Host 选择: direct_answer reply_intent=%q", reply.Intent)
			}
			if reply.Intent != tt.want {
				t.Fatalf("intent mismatch: want %q got %q content=%q", tt.want, reply.Intent, reply.Content)
			}
		})
	}
}

type routingChatModel struct {
	tools []*schema.ToolInfo
}

func (m *routingChatModel) WithTools(tools []*schema.ToolInfo) (einomodel.ToolCallingChatModel, error) {
	return &routingChatModel{tools: tools}, nil
}

func (m *routingChatModel) Generate(_ context.Context, input []*schema.Message, _ ...einomodel.Option) (*schema.Message, error) {
	if len(m.tools) == 0 {
		return nil, errors.New("routing model must be used as host with tools")
	}

	query := lastUserContent(input)
	return schema.AssistantMessage("", []schema.ToolCall{{
		ID:   "call_route",
		Type: "function",
		Function: schema.FunctionCall{
			Name:      m.route(query),
			Arguments: `{"reason":"unit test route"}`,
		},
	}}), nil
}

func (m *routingChatModel) Stream(ctx context.Context, input []*schema.Message, opts ...einomodel.Option) (*schema.StreamReader[*schema.Message], error) {
	msg, err := m.Generate(ctx, input, opts...)
	if err != nil {
		return nil, err
	}
	return schema.StreamReaderFromArray([]*schema.Message{msg}), nil
}

func (m *routingChatModel) route(query string) string {
	switch {
	case strings.Contains(query, "评审") || strings.Contains(query, "优化"):
		return "code_reviewer"
	case strings.Contains(query, "Segmentation") || strings.Contains(query, "报错") || strings.Contains(query, "错误"):
		return "debug_tutor"
	default:
		return "concept_tutor"
	}
}

type recordingRunner struct {
	mu    sync.Mutex
	calls map[constant.IntentType]int
}

func (r *recordingRunner) Invoke(_ context.Context, in *localagent.AgentInput, _ ...compose.Option) (*localagent.Reply, error) {
	if in == nil {
		return nil, errors.New("nil agent input")
	}
	r.mu.Lock()
	if r.calls == nil {
		r.calls = map[constant.IntentType]int{}
	}
	r.calls[in.Intent.Type]++
	r.mu.Unlock()

	return &localagent.Reply{
		Intent:  in.Intent.Type,
		Content: fmt.Sprintf("%s handled: %s", in.Intent.Type, in.Query),
	}, nil
}

func (r *recordingRunner) Stream(context.Context, *localagent.AgentInput, ...compose.Option) (*schema.StreamReader[*localagent.Reply], error) {
	return nil, errors.New("not implemented")
}

func (r *recordingRunner) Collect(context.Context, *schema.StreamReader[*localagent.AgentInput], ...compose.Option) (*localagent.Reply, error) {
	return nil, errors.New("not implemented")
}

func (r *recordingRunner) Transform(context.Context, *schema.StreamReader[*localagent.AgentInput], ...compose.Option) (*schema.StreamReader[*localagent.Reply], error) {
	return nil, errors.New("not implemented")
}

func (r *recordingRunner) count(intent constant.IntentType) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls[intent]
}

func totalCalls(intent constant.IntentType, runners ...*recordingRunner) int {
	total := 0
	for _, runner := range runners {
		total += runner.count(intent)
	}
	return total
}

type handoffRecorder struct {
	mu    sync.Mutex
	infos []*hostma.HandOffInfo
}

func (r *handoffRecorder) OnHandOff(ctx context.Context, info *hostma.HandOffInfo) context.Context {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.infos = append(r.infos, info)
	return ctx
}

func (r *handoffRecorder) last() *hostma.HandOffInfo {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.infos) == 0 {
		return nil
	}
	return r.infos[len(r.infos)-1]
}
