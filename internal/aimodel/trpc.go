// Package aimodel 管理 trpc-agent-go 模型资源。
// 对话/意图模型按 userID 维护单例,embedder 与用户无关。
//
// 生成参数随 ModelConfig 带出,在 trpc 走调用时的
// model.Request.GenerationConfig 注入,不在建模时配。
package aimodel

import (
	"fmt"
	"sync"

	trpcembedder "trpc.group/trpc-go/trpc-agent-go/knowledge/embedder/openai"
	trpcopenai "trpc.group/trpc-go/trpc-agent-go/model/openai"

	"GopherPaper/internal/config"
)

// cfg 保存全局配置,须在使用 ModelsForUser / NewEmbedder 前由 Init 注入。
var cfg *config.Config

// Init 保存全局配置。
func Init(c *config.Config) {
	cfg = c
}

// NewChatModel 用 trpc 的 openai 兼容 model 建对话/意图模型。
func NewChatModel(mc config.ModelConfig) *trpcopenai.Model {
	return trpcopenai.New(mc.Model,
		trpcopenai.WithBaseURL(mc.BaseURL),
		trpcopenai.WithAPIKey(mc.APIKey),
	)
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

// ModelSet 是单用户的 trpc 模型集合和对应生成参数配置。
type ModelSet struct {
	Intent   *trpcopenai.Model  // 意图分类小模型
	Chat     *trpcopenai.Model  // 下游 RAG/抽取/报告主力模型
	IntentMC config.ModelConfig // 意图模型生成参数
	ChatMC   config.ModelConfig // 对话模型生成参数
}

// modelSets 按 userID 缓存,保留每用户隔离的口子。
var modelSets sync.Map // userID -> *modelSetEntry

type modelSetEntry struct {
	once   sync.Once
	models *ModelSet
}

// ModelsForUser 返回该用户的 trpc 模型集合,未命中则建并缓存。
func ModelsForUser(userID string) (*ModelSet, error) {
	if cfg == nil {
		return nil, fmt.Errorf("aimodel: 未初始化")
	}
	if userID == "" {
		return nil, fmt.Errorf("aimodel: userID 不能为空")
	}
	e, _ := modelSets.LoadOrStore(userID, &modelSetEntry{})
	ent := e.(*modelSetEntry)
	ent.once.Do(func() {
		ent.models = &ModelSet{
			Intent:   NewChatModel(cfg.Models.Intent),
			Chat:     NewChatModel(cfg.Models.Chat),
			IntentMC: cfg.Models.Intent,
			ChatMC:   cfg.Models.Chat,
		}
	})
	return ent.models, nil
}
