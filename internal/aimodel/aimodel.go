// Package aimodel 管理 trpc-agent-go 模型资源。
// 对话/意图模型按 userID 维护单例,embedder 与用户无关。
//
// 生成参数随 ModelConfig 带出,在 trpc 走调用时的
// model.Request.GenerationConfig 注入,不在建模时配。
package aimodel

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	trpcembedder "trpc.group/trpc-go/trpc-agent-go/knowledge/embedder/openai"
	trpcreranker "trpc.group/trpc-go/trpc-agent-go/knowledge/reranker"
	trpcinfinity "trpc.group/trpc-go/trpc-agent-go/knowledge/reranker/infinity"
	trpcopenai "trpc.group/trpc-go/trpc-agent-go/model/openai"

	"GopherPaper/internal/config"
	"GopherPaper/internal/zlog"
	"GopherPaper/pkg/constant"
)

// cfg 保存全局配置,须在使用 ModelsForUser / NewEmbedder 前由 Init 注入。
var cfg *config.Config

// UserConfigResolver 在构建某个用户的模型缓存前叠加该用户自己的模型配置。
// resolver 只能向下依赖 dao/model 等底层包,避免 aimodel 反向 import service 造成循环。
type UserConfigResolver func(ctx context.Context, userID string, base *config.Config) (*config.Config, error)

var (
	resolverMu         sync.RWMutex
	userConfigResolver UserConfigResolver
)

// Init 保存全局配置。
func Init(c *config.Config) {
	cfg = c
}

func SetUserConfigResolver(r UserConfigResolver) {
	resolverMu.Lock()
	defer resolverMu.Unlock()
	userConfigResolver = r
}

// NewChatModel 用 trpc 的 openai 兼容 model 建对话/意图模型。
// 空 key 用占位:ollama 等本地 openai 兼容端点不校验密钥,但 sdk 要求非空。
// thinking 开关经模型级 extra fields 随每次请求注入,火山方舟格式 thinking.type。
func NewChatModel(mc config.ModelConfig) *trpcopenai.Model {
	apiKey := mc.APIKey
	if apiKey == "" {
		apiKey = "ollama"
	}
	opts := []trpcopenai.Option{
		trpcopenai.WithBaseURL(mc.BaseURL),
		trpcopenai.WithAPIKey(apiKey),
	}
	if mc.Thinking != "" {
		opts = append(opts, trpcopenai.WithExtraFields(map[string]any{
			"thinking": map[string]any{"type": mc.Thinking},
		}))
	}
	return trpcopenai.New(mc.Model, opts...)
}

// NewEmbedder 用 trpc 的 openai 兼容 embedder 建向量化器。
// bge-m3 经 Ollama 的 OpenAI 兼容端点接入,ec.BaseURL 须指向 /v1 兼容地址。
func NewEmbedder(ec config.ModelConfig) *trpcembedder.Embedder {
	apiKey := ec.APIKey
	if apiKey == "" {
		apiKey = "ollama" // Ollama 兼容端点不校验密钥,占位避免 sdk 报错
	}
	return trpcembedder.New(
		trpcembedder.WithBaseURL(ec.BaseURL),
		trpcembedder.WithModel(ec.Model),
		trpcembedder.WithDimensions(ec.Dim),
		trpcembedder.WithAPIKey(apiKey),
	)
}

// NewReranker 用 trpc 的 infinity reranker 接 OpenAI/Infinity 兼容的 /rerank 端点(硅基流动等)。
// 与用户无关,启动期建一次交给 chat 检索层。enabled 关闭或建失败返回 nil,RAG 退化为纯向量召回。
// WithTopN 设为候选上界 RecallTopK:等效全量重排返回(候选数 <= 此值),再由调用方按正文/图块各自截断。
// 必须显式设:组件默认 top_n=-1,而硅基等服务要求 top_n>=1,否则 400 拒绝。
func NewReranker(rc config.RerankConfig) trpcreranker.Reranker {
	if !rc.Enabled {
		return nil
	}
	r, err := trpcinfinity.New(
		trpcinfinity.WithEndpoint(rc.BaseURL),
		trpcinfinity.WithModel(rc.Model),
		trpcinfinity.WithAPIKey(rc.APIKey),
		trpcinfinity.WithTopN(constant.RecallTopK),
		// 自定义超时:组件默认 30s,精排卡顿会拖满整轮问答。精排 best-effort,超时即退化为向量序,宜短失败快回退。
		trpcinfinity.WithHTTPClient(&http.Client{Timeout: time.Duration(rc.Timeout) * time.Second}),
	)
	if err != nil {
		zlog.Error("rerank 初始化失败,退化为纯向量召回", "err", err)
		return nil
	}
	return r
}

// ModelSet 是单用户的 trpc 模型集合和对应生成参数配置。
type ModelSet struct {
	Intent      *trpcopenai.Model  // 意图分类小模型
	Chat        *trpcopenai.Model  // 下游 RAG/抽取/报告主力模型
	Vlm         *trpcopenai.Model  // 带图推理视觉模型:图描述生成与带图问答
	Translate   *trpcopenai.Model  // 精读页逐段翻译小模型
	Pioneer     *trpcopenai.Model  // 小云雀工具循环模型,未配置时即 Chat
	Maodie      *trpcopenai.Model  // 小耄耋局部问答模型,未配置时即 Chat;带图时调用方仍可选择 Chat
	IntentMC    config.ModelConfig // 意图模型生成参数
	ChatMC      config.ModelConfig // 对话模型生成参数
	VlmMC       config.ModelConfig // 视觉模型生成参数
	TranslateMC config.ModelConfig // 翻译模型生成参数
	PioneerMC   config.ModelConfig // 小云雀模型生成参数
	MaodieMC    config.ModelConfig // 小耄耋模型生成参数
}

// modelSets 按 userID 缓存,保留每用户隔离的口子。
var modelSets sync.Map // userID -> *modelSetEntry

type modelSetEntry struct {
	once   sync.Once
	models *ModelSet
	err    error
}

// ModelsForUser 返回该用户的 trpc 模型集合,未命中则建并缓存。
func ModelsForUser(ctx context.Context, userID string) (*ModelSet, error) {
	if cfg == nil {
		return nil, fmt.Errorf("aimodel: 未初始化")
	}
	if userID == "" {
		return nil, fmt.Errorf("aimodel: userID 不能为空")
	}
	e, _ := modelSets.LoadOrStore(userID, &modelSetEntry{})
	ent := e.(*modelSetEntry)
	ent.once.Do(func() {
		effectiveCfg := cfg
		resolverMu.RLock()
		resolver := userConfigResolver
		resolverMu.RUnlock()
		if resolver != nil {
			base := *cfg
			resolved, err := resolver(ctx, userID, &base)
			if err != nil {
				ent.err = err
				return
			}
			if resolved != nil {
				effectiveCfg = resolved
			}
		}
		ent.models = &ModelSet{
			Intent:      NewChatModel(effectiveCfg.Models.Intent),
			Chat:        NewChatModel(effectiveCfg.Models.Chat),
			Vlm:         NewChatModel(effectiveCfg.Models.Vlm),
			Translate:   NewChatModel(effectiveCfg.Models.Translate),
			IntentMC:    effectiveCfg.Models.Intent,
			ChatMC:      effectiveCfg.Models.Chat,
			VlmMC:       effectiveCfg.Models.Vlm,
			TranslateMC: effectiveCfg.Models.Translate,
		}
		// 小云雀模型未配置时回退 Chat,复用同一实例。
		if effectiveCfg.Models.Pioneer.Model != "" {
			ent.models.Pioneer = NewChatModel(effectiveCfg.Models.Pioneer)
			ent.models.PioneerMC = effectiveCfg.Models.Pioneer
		} else {
			ent.models.Pioneer = ent.models.Chat
			ent.models.PioneerMC = effectiveCfg.Models.Chat
		}
		// 小耄耋模型未配置时回退 Chat,避免缺配置影响精读页。
		if effectiveCfg.Models.Maodie.Model != "" {
			ent.models.Maodie = NewChatModel(effectiveCfg.Models.Maodie)
			ent.models.MaodieMC = effectiveCfg.Models.Maodie
		} else {
			ent.models.Maodie = ent.models.Chat
			ent.models.MaodieMC = effectiveCfg.Models.Chat
		}
	})
	if ent.err != nil {
		modelSets.Delete(userID)
		return nil, ent.err
	}
	return ent.models, nil
}

// EvictUser 清除该用户缓存的模型集合,登出时调用释放常驻内存。
// 模型只是 http client 句柄无需显式关闭,删 map 条目即由 GC 回收;
// 下次该用户访问 ModelsForUser 会按需重建。
func EvictUser(userID string) {
	modelSets.Delete(userID)
}

func EvictAll() {
	modelSets.Range(func(k, _ any) bool {
		modelSets.Delete(k)
		return true
	})
}
