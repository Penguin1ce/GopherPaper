// toolfailmemory_test.go 验证小云雀的核心问题:工具调用失败后,失败结果是否会进入
// 工作记忆并影响后续轮次。
//
// 不连真实模型与 Redis:用脚本化 fake model 驱动 ReAct 循环,用 inmemory session 承载
// 工作记忆。持久化逻辑与线上 Redis session 同一套,只换存储后端,记忆行为一致。
// 一个注定失败的 function tool 模拟凭据失效 401。断言两件事:
//  1. 形成错误记忆:失败被框架包成 role=tool 消息喂回同轮模型
//  2. 影响后续:换一轮对话同 session 时上轮的失败消息仍在喂给模型的历史里
package pioneer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"trpc.group/trpc-go/trpc-agent-go/agent/llmagent"
	"trpc.group/trpc-go/trpc-agent-go/event"
	trpcmodel "trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/runner"
	"trpc.group/trpc-go/trpc-agent-go/session/inmemory"
	"trpc.group/trpc-go/trpc-agent-go/tool"
	"trpc.group/trpc-go/trpc-agent-go/tool/function"
)

// scriptedModel 是按调用序返回预设响应的 fake model,同时记录每次收到的对话历史,
// 用来观测上一轮的工具失败是否出现在后续请求的 messages 里。
type scriptedModel struct {
	mu      sync.Mutex
	replies []trpcmodel.Message   // 第 i 次 GenerateContent 返回的 assistant 消息
	seen    [][]trpcmodel.Message // 第 i 次收到的 request.Messages 快照
}

func (m *scriptedModel) Info() trpcmodel.Info { return trpcmodel.Info{Name: "scripted"} }

func (m *scriptedModel) GenerateContent(ctx context.Context, req *trpcmodel.Request) (<-chan *trpcmodel.Response, error) {
	m.mu.Lock()
	idx := len(m.seen)
	snap := make([]trpcmodel.Message, len(req.Messages))
	copy(snap, req.Messages)
	m.seen = append(m.seen, snap)
	var reply trpcmodel.Message
	if idx < len(m.replies) {
		reply = m.replies[idx]
	} else {
		reply = trpcmodel.Message{Role: trpcmodel.RoleAssistant, Content: "脚本已耗尽"}
	}
	m.mu.Unlock()

	finish := "stop"
	if len(reply.ToolCalls) > 0 {
		finish = "tool_calls"
	}
	ch := make(chan *trpcmodel.Response, 1)
	ch <- &trpcmodel.Response{
		ID:      fmt.Sprintf("resp-%d", idx),
		Object:  trpcmodel.ObjectTypeChatCompletion,
		Created: time.Now().Unix(),
		Done:    true,
		Choices: []trpcmodel.Choice{{
			Index:        0,
			Message:      reply,
			FinishReason: &finish,
		}},
	}
	close(ch)
	return ch, nil
}

// seenAt 返回第 i 次模型调用收到的对话历史,越界返回 nil。
func (m *scriptedModel) seenAt(i int) []trpcmodel.Message {
	m.mu.Lock()
	defer m.mu.Unlock()
	if i < 0 || i >= len(m.seen) {
		return nil
	}
	return m.seen[i]
}

func (m *scriptedModel) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.seen)
}

type coffeeArgs struct {
	Item string `json:"item"`
}
type coffeeOut struct {
	OK bool `json:"ok"`
}

// failErrText 是工具失败抛出的错误文本,断言时按它判定错误记忆是否存在。
const failErrText = "瑞幸下单失败: 凭据失效 401 Unauthorized"

// newFlakyTool 造一个注定失败的下单工具,模拟凭据型 mcp 工具的 401。
func newFlakyTool(calls *int) tool.Tool {
	return function.NewFunctionTool(
		func(ctx context.Context, a coffeeArgs) (coffeeOut, error) {
			*calls++
			return coffeeOut{}, errors.New(failErrText)
		},
		function.WithName("order_coffee"),
		function.WithDescription("给用户点一杯咖啡"),
	)
}

// toolCallReply 造一条让模型去调 order_coffee 的 assistant 消息。
func toolCallReply() trpcmodel.Message {
	return trpcmodel.Message{
		Role: trpcmodel.RoleAssistant,
		ToolCalls: []trpcmodel.ToolCall{{
			Type: "function",
			ID:   "call-1",
			Function: trpcmodel.FunctionDefinitionParam{
				Name:      "order_coffee",
				Arguments: json.RawMessage(`{"item":"latte"}`),
			},
		}},
	}
}

// drain 读完一轮 runner 事件流,把工具失败的 critical error 透出来辅助定位。
func drain(t *testing.T, ch <-chan *event.Event) {
	t.Helper()
	for ev := range ch {
		if ev.Error != nil {
			t.Logf("runner 事件错误 critical: %s", ev.Error.Message)
		}
	}
}

// hasToolError 判断一段对话历史里是否有携带失败文本的 tool 消息。
func hasToolError(msgs []trpcmodel.Message) bool {
	for _, m := range msgs {
		if m.Role == trpcmodel.RoleTool && strings.Contains(m.Content, "401") {
			return true
		}
	}
	return false
}

// TestToolFailureFormsErrorMemory 复现并固化小云雀的工作记忆行为:
// 工具失败会形成错误记忆并跨轮影响后续。结论记录在 doc/pioneer-tool-failure-memory.md。
func TestToolFailureFormsErrorMemory(t *testing.T) {
	var toolCalls int
	flaky := newFlakyTool(&toolCalls)

	// 脚本:第 1 次调工具,第 2 次第一轮收尾,第 3 次第二轮直接回答不再调工具。
	sm := &scriptedModel{replies: []trpcmodel.Message{
		toolCallReply(),
		{Role: trpcmodel.RoleAssistant, Content: "抱歉,咖啡没点成,凭据好像失效了。"},
		{Role: trpcmodel.RoleAssistant, Content: "现在大约中午。"},
	}}

	ag := llmagent.New("pioneer-test",
		llmagent.WithModel(sm),
		llmagent.WithTools([]tool.Tool{flaky}),
		llmagent.WithMaxHistoryRuns(40),
	)
	rt := runner.NewRunner("gopherpaper-test", ag,
		runner.WithSessionService(inmemory.NewSessionService()))

	ctx := context.Background()
	const userID, sessionID = "stu-1", "sess-1"

	// 第一轮:用户点咖啡,模型调工具,工具 401 失败。
	ch, err := rt.Run(ctx, userID, sessionID, trpcmodel.NewUserMessage("帮我点一杯拿铁"))
	if err != nil {
		t.Fatalf("第一轮 Run 失败: %v", err)
	}
	drain(t, ch)

	if toolCalls == 0 {
		t.Fatal("工具未被调用,脚本或工具挂载有误")
	}
	if sm.callCount() < 2 {
		t.Fatalf("模型应至少被调用两次（工具失败后需再问一次）, 实际 %d", sm.callCount())
	}

	// 断言一:形成错误记忆 —— 工具失败后第二次模型调用的历史里带 role=tool 的失败消息。
	secondCall := sm.seenAt(1)
	if !hasToolError(secondCall) {
		t.Errorf("断言一失败:工具失败结果未作为 tool 消息喂回同轮模型 messages=%+v", secondCall)
	} else {
		t.Logf("断言一通过:失败被包成 tool 消息喂回同轮模型,共 %d 条历史", len(secondCall))
	}

	// 第二轮:换个话题,同一 session 同一 user。
	ch2, err := rt.Run(ctx, userID, sessionID, trpcmodel.NewUserMessage("现在几点了"))
	if err != nil {
		t.Fatalf("第二轮 Run 失败: %v", err)
	}
	drain(t, ch2)

	// 断言二:影响后续 —— 第二轮的首个模型调用历史里仍残留上一轮的工具失败消息。
	lastFirstCall := sm.seenAt(2) // 全局第 3 次调用即第二轮首次
	if !hasToolError(lastFirstCall) {
		t.Errorf("断言二失败:第二轮未携带上一轮工具失败记忆 messages=%+v", lastFirstCall)
	} else {
		t.Logf("断言二通过:上一轮工具失败仍在第二轮喂给模型的历史里,共 %d 条", len(lastFirstCall))
	}
}
