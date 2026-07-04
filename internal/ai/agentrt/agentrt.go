// Package agentrt 是下游链路的统一生成入口。
// 按 userID 懒建并缓存一个带工具的 chat agent 与 runner,把 toolkit 的 mcp 工具与
// skills 挂上去,chat/extract/report 三条链路的生成段都经此跑一轮再聚合成文本。
// 用户身份从 ctx 的 tenant 取,调用方不传 userID。
//
// 每轮的 system prompt 与历史按请求注入,不依赖 runner 的会话持久化:
// system prompt 走 WithInstruction 覆盖,历史走 WithInjectedContextMessages 注入,
// 会话历史的 system-of-record 仍是 history 包,故 runner 配 noop session。
package agentrt

import (
	"context"
	"fmt"
	"sync"

	"trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/agent/llmagent"
	trpcmodel "trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/runner"
	sessnoop "trpc.group/trpc-go/trpc-agent-go/session/noop"

	"GopherPaper/internal/ai/core"
	"GopherPaper/internal/ai/toolkit"
	"GopherPaper/internal/aimodel"
	"GopherPaper/internal/tenant"
)

const (
	appName   = "gopherpaper"
	agentName = "paper-assistant"
	sessionID = "default" // noop session 不落库,固定即可
)

// runners 按用户和 agent 分组缓存 runner。默认论文助教与小囊鼠等分组使用同一模型缓存,
// 但各自挂载隔离的工具和 skill 仓库。
var runners sync.Map // runnerKey -> *runnerEntry

type runnerKey struct {
	userID string
	agent  string
}

type runnerEntry struct {
	once sync.Once
	rt   runner.Runner
	err  error
}

// Generate 经该用户的带工具 chat agent 跑一轮并聚合成完整文本。
// instruction 为本轮 system prompt,history 为多轮上下文(注入不持久化),query 为当前输入。
func Generate(ctx context.Context, instruction string, history []trpcmodel.Message, query string) (string, error) {
	return GenerateWithImages(ctx, instruction, history, query, nil)
}

// GenerateFor 使用指定 agent 分组的工具和 skill 生成文本。
// 适合需要复用统一运行时、但必须隔离能力集合的下游链路。
func GenerateFor(
	ctx context.Context,
	agentGroup string,
	instruction string,
	history []trpcmodel.Message,
	query string,
) (string, error) {
	return generateWithImagesFor(ctx, agentGroup, instruction, history, query, nil)
}

// Image 是带图问答的一张图片,Data 为原始字节,Format 为不带点的扩展名(png/jpeg/...)。
type Image struct {
	Data   []byte
	Format string
}

// GenerateWithImages 同 Generate,额外把 images 作为当前 user 轮次的图片随 query 一起发给模型。
// 多张图塞进同一条 user message 做一次综合推理;chat 模型须支持视觉(本项目 doubao-seed-2-0-pro-260215 多模态)。
func GenerateWithImages(ctx context.Context, instruction string, history []trpcmodel.Message, query string, images []Image) (string, error) {
	return generateWithImagesFor(ctx, "", instruction, history, query, images)
}

func generateWithImagesFor(
	ctx context.Context,
	agentGroup string,
	instruction string,
	history []trpcmodel.Message,
	query string,
	images []Image,
) (string, error) {
	userID := tenant.MustStudentID(ctx)
	rt, err := runnerForUser(ctx, userID, agentGroup)
	if err != nil {
		return "", err
	}
	ch, err := rt.Run(ctx, userID, sessionID, userMessage(query, images),
		agent.WithInstruction(instruction),
		agent.WithInjectedContextMessages(history),
	)
	if err != nil {
		return "", err
	}
	return core.CollectEvents(ctx, ch)
}

// userMessage 构造当前 user 轮次:无图时退化为纯文本,有图时文本与图片同放 ContentParts。
func userMessage(query string, images []Image) trpcmodel.Message {
	if len(images) == 0 {
		return trpcmodel.NewUserMessage(query)
	}
	msg := trpcmodel.Message{Role: trpcmodel.RoleUser}
	q := query
	msg.ContentParts = append(msg.ContentParts, trpcmodel.ContentPart{Type: trpcmodel.ContentTypeText, Text: &q})
	for _, img := range images {
		msg.AddImageData(img.Data, "auto", img.Format)
	}
	return msg
}

// runnerForUser 懒建该用户在指定 agent 分组下的 runner。
func runnerForUser(ctx context.Context, userID, agentGroup string) (runner.Runner, error) {
	if userID == "" {
		return nil, fmt.Errorf("agentrt: userID 不能为空")
	}
	models, err := aimodel.ModelsForUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	key := runnerKey{userID: userID, agent: agentGroup}
	e, _ := runners.LoadOrStore(key, &runnerEntry{})
	ent := e.(*runnerEntry)
	ent.once.Do(func() {
		gc := core.GenConfig(models.ChatMC)
		// 流式拉取模型输出,问答链路经 ctx 的 StreamHandler 把增量推给前端;
		// 无 handler 的链路(抽取/报告)仍由 CollectEvents 聚合,行为不变。
		gc.Stream = true
		sets := toolkit.ToolSets()
		repo := toolkit.SkillRepo()
		if agentGroup != "" {
			sets = toolkit.ToolSetsFor(agentGroup)
			repo = toolkit.SkillRepoFor(agentGroup)
		}
		// 兼容兜底:部分 openai 兼容端点在 chat/completions 下 function tools 与
		// reasoning_effort 不能同用(400),配了工具就剥离推理强度走端点默认。
		if len(sets) > 0 {
			gc.ReasoningEffort = nil
		}
		opts := []llmagent.Option{
			llmagent.WithModel(models.Chat),
			llmagent.WithGenerationConfig(gc),
		}
		if len(sets) > 0 {
			opts = append(opts, llmagent.WithToolSets(sets))
		}
		if repo != nil {
			opts = append(opts, llmagent.WithSkills(repo))
		}
		name := agentName
		if agentGroup != "" {
			name += "-" + agentGroup
		}
		ent.rt = runner.NewRunner(appName, llmagent.New(name, opts...),
			runner.WithSessionService(sessnoop.NewService()))
	})
	return ent.rt, ent.err
}

// EvictUser 清除该用户缓存的 runner,登出时调用。下次访问 runnerForUser 重建。
// runner 持有的 toolkit 工具集是包级共享引用,不随条目删除关闭;noop session 无持久态。
func EvictUser(userID string) {
	runners.Range(func(key, _ any) bool {
		if key.(runnerKey).userID == userID {
			runners.Delete(key)
		}
		return true
	})
}

func EvictAll() {
	runners.Range(func(k, _ any) bool {
		runners.Delete(k)
		return true
	})
}
