// Package agentrt 是下游链路的统一生成入口。
// 按 userID 懒建并缓存一个带工具的 chat agent 与 runner,把 toolkit 的 mcp 工具与
// skills 挂上去,chat/extract/report 三条链路的生成段都经此跑一轮再聚合成文本。
//
// 每轮的 system prompt 与历史按请求注入,不依赖 runner 的会话持久化:
// system prompt 走 WithInstruction 覆盖,历史走 WithInjectedContextMessages 注入,
// 会话历史的 system-of-record 仍是 history 包,故 runner 配 noop session。
package agentrt

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/agent/llmagent"
	"trpc.group/trpc-go/trpc-agent-go/event"
	trpcmodel "trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/runner"
	sessnoop "trpc.group/trpc-go/trpc-agent-go/session/noop"

	"GopherPaper/internal/ai/toolkit"
	"GopherPaper/internal/aimodel"
	"GopherPaper/internal/config"
)

const (
	appName   = "gopherpaper"
	agentName = "paper-assistant"
	sessionID = "default" // noop session 不落库,固定即可
)

// runners 按 userID 缓存 runner,与 aimodel 的模型缓存一一对应。
var runners sync.Map // userID -> *runnerEntry

type runnerEntry struct {
	once sync.Once
	rt   runner.Runner
	err  error
}

// Generate 经该用户的带工具 chat agent 跑一轮并聚合成完整文本。
// instruction 为本轮 system prompt,history 为多轮上下文(注入不持久化),query 为当前输入。
func Generate(ctx context.Context, userID, instruction string, history []trpcmodel.Message, query string) (string, error) {
	rt, err := runnerForUser(userID)
	if err != nil {
		return "", err
	}
	ch, err := rt.Run(ctx, userID, sessionID, trpcmodel.NewUserMessage(query),
		agent.WithInstruction(instruction),
		agent.WithInjectedContextMessages(history),
	)
	if err != nil {
		return "", err
	}
	return collect(ch)
}

// runnerForUser 懒建该用户的 runner,模型取自 aimodel,工具取自 toolkit。
func runnerForUser(userID string) (runner.Runner, error) {
	if userID == "" {
		return nil, fmt.Errorf("agentrt: userID 不能为空")
	}
	models, err := aimodel.ModelsForUser(userID)
	if err != nil {
		return nil, err
	}
	e, _ := runners.LoadOrStore(userID, &runnerEntry{})
	ent := e.(*runnerEntry)
	ent.once.Do(func() {
		opts := []llmagent.Option{
			llmagent.WithModel(models.Chat),
			llmagent.WithGenerationConfig(genConfig(models.ChatMC)),
		}
		if sets := toolkit.ToolSets(); len(sets) > 0 {
			opts = append(opts, llmagent.WithToolSets(sets))
		}
		if repo := toolkit.SkillRepo(); repo != nil {
			opts = append(opts, llmagent.WithSkills(repo))
		}
		ent.rt = runner.NewRunner(appName, llmagent.New(agentName, opts...),
			runner.WithSessionService(sessnoop.NewService()))
	})
	return ent.rt, ent.err
}

// genConfig 把 ModelConfig 的生成参数映射到 trpc 的 GenerationConfig。
func genConfig(mc config.ModelConfig) trpcmodel.GenerationConfig {
	var gc trpcmodel.GenerationConfig
	if mc.MaxTokens > 0 {
		gc.MaxTokens = &mc.MaxTokens
	}
	if mc.ReasoningEffort != "" {
		gc.ReasoningEffort = &mc.ReasoningEffort
	}
	return gc
}

// collect 聚合事件流为完整答案,跳过工具结果与 runner 收尾事件,避免混入或重复计数。
func collect(ch <-chan *event.Event) (string, error) {
	var sb strings.Builder
	for ev := range ch {
		if ev.Error != nil {
			return "", fmt.Errorf("agentrt: %s", ev.Error.Message)
		}
		if ev.Object == trpcmodel.ObjectTypeToolResponse || ev.IsRunnerCompletion() {
			continue
		}
		for _, c := range ev.Choices {
			sb.WriteString(c.Message.Content)
		}
	}
	out := strings.TrimSpace(sb.String())
	if out == "" {
		return "", fmt.Errorf("agentrt: 模型返回空内容")
	}
	return out, nil
}
