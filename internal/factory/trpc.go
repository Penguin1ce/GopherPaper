// Package factory 按配置创建 trpc-agent-go 的 model 与 embedder。
// 对话/意图模型按 userID 维护单例(同用户复用、不同用户隔离),embedder 与用户无关。
//
// 生成参数(MaxTokens/ReasoningEffort)随 ModelConfig 带出,在 trpc 走调用时的
// model.Request.GenerationConfig 注入,不在建模时配。
package factory

import (
	"fmt"
	"sync"

	trpcembedder "trpc.group/trpc-go/trpc-agent-go/knowledge/embedder/openai"
	trpcopenai "trpc.group/trpc-go/trpc-agent-go/model/openai"

	"GopherPaper/internal/config"
)

// cfg 保存全局配置,须在使用 TRPCModelsForUser / NewTRPCEmbedder 前由 Init 注入。
var cfg *config.Config

// Init 保存全局配置。
func Init(c *config.Config) {
	cfg = c
}

// NewTRPCChatModel 用 trpc 的 openai 兼容 model 建对话/意图模型,走自定义网关。
func NewTRPCChatModel(mc config.ModelConfig) *trpcopenai.Model {
	return trpcopenai.New(mc.Model,
		trpcopenai.WithBaseURL(mc.BaseURL),
		trpcopenai.WithAPIKey(mc.APIKey),
	)
}

// NewTRPCEmbedder 用 trpc 的 openai 兼容 embedder 建向量化器。
// trpc v1.10.0 无独立 ollama embedder,bge-m3 经 Ollama 的 OpenAI 兼容端点(/v1)接入,
// 故 ec.BaseURL 须指向 /v1 兼容地址。
func NewTRPCEmbedder(ec config.ModelConfig) *trpcembedder.Embedder {
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

// TRPCUserModels 是单用户的 trpc 模型集合 + 对应生成参数配置。
// 生成参数(MaxTokens/ReasoningEffort)随 ModelConfig 带出,调用时注入 Request.GenerationConfig。
type TRPCUserModels struct {
	Intent   *trpcopenai.Model  // 意图分类小模型
	Chat     *trpcopenai.Model  // 下游 RAG/抽取/报告主力模型
	IntentMC config.ModelConfig // 意图模型生成参数
	ChatMC   config.ModelConfig // 对话模型生成参数
}

// trpcUserModels 按 userID 缓存,保留每用户隔离的口子(模型本身并发安全)。
var trpcUserModels sync.Map // userID -> *trpcUserEntry

type trpcUserEntry struct {
	once   sync.Once
	models *TRPCUserModels
}

// TRPCModelsForUser 返回该用户的 trpc 模型集合,未命中则建并缓存。
func TRPCModelsForUser(userID string) (*TRPCUserModels, error) {
	if cfg == nil {
		return nil, fmt.Errorf("factory: 未初始化")
	}
	if userID == "" {
		return nil, fmt.Errorf("factory: userID 不能为空")
	}
	e, _ := trpcUserModels.LoadOrStore(userID, &trpcUserEntry{})
	ent := e.(*trpcUserEntry)
	ent.once.Do(func() {
		ent.models = &TRPCUserModels{
			Intent:   NewTRPCChatModel(cfg.Models.Intent),
			Chat:     NewTRPCChatModel(cfg.Models.Chat),
			IntentMC: cfg.Models.Intent,
			ChatMC:   cfg.Models.Chat,
		}
	})
	return ent.models, nil
}
