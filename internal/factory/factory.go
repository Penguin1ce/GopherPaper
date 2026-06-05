// Package factory 用工厂模式按配置创建 eino 的模型与向量化组件。
// 对话模型按 userID 维护全局单例：同用户复用同一实例、不同用户隔离，
// 为后续每用户模型配置 / 限流留口子。embedder 与用户无关，保持单全局实例。
// 上层只依赖 eino 标准接口，新增 provider 只改这里。
package factory

import (
	"context"
	"fmt"
	"sync"
	"time"

	embollama "github.com/cloudwego/eino-ext/components/embedding/ollama"
	mdollama "github.com/cloudwego/eino-ext/components/model/ollama"
	mdopenai "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/model"

	"GopherPaper/internal/config"
	"GopherPaper/pkg/constant"
)

// UserModels 是单个用户的模型实例集合：意图路由小模型 + 下游主力大模型。
type UserModels struct {
	Intent model.ToolCallingChatModel
	Chat   model.ToolCallingChatModel
}

var (
	cfg *config.Config

	// userModels 按 userID 缓存模型集合，per-key once 保证只构建一次。
	userModels sync.Map // userID -> *userEntry
)

type userEntry struct {
	once   sync.Once
	models *UserModels
	err    error
}

// Init 保存全局配置，须在使用 ModelsForUser / NewEmbedder 前调用。
func Init(c *config.Config) {
	cfg = c
}

// ModelsForUser 返回该用户的模型单例，未命中则用工厂构建并缓存。
func ModelsForUser(ctx context.Context, userID string) (*UserModels, error) {
	if cfg == nil {
		return nil, fmt.Errorf("factory: 未初始化")
	}
	if userID == "" {
		return nil, fmt.Errorf("factory: userID 不能为空")
	}
	e, _ := userModels.LoadOrStore(userID, &userEntry{})
	ent := e.(*userEntry)
	ent.once.Do(func() {
		ent.models, ent.err = buildUserModels(ctx, userID)
	})
	if ent.err != nil {
		userModels.Delete(userID) // 构建失败不缓存，允许下次重试
		return nil, ent.err
	}
	return ent.models, nil
}

func buildUserModels(ctx context.Context, userID string) (*UserModels, error) {
	intent, err := newToolCallingModel(ctx, cfg.Models.Intent)
	if err != nil {
		return nil, fmt.Errorf("factory: 创建意图模型失败(user=%s): %w", userID, err)
	}
	chat, err := newToolCallingModel(ctx, cfg.Models.Chat)
	if err != nil {
		return nil, fmt.Errorf("factory: 创建对话模型失败(user=%s): %w", userID, err)
	}
	return &UserModels{Intent: intent, Chat: chat}, nil
}

// NewEmbedder 创建 RAG 向量化模型，与用户无关，启动期建一次供 knowledge 注入。
func NewEmbedder(ctx context.Context) (embedding.Embedder, error) {
	if cfg == nil {
		return nil, fmt.Errorf("factory: 未初始化")
	}
	ec := cfg.Embedding
	switch ec.Provider {
	case constant.ProviderOllama:
		return embollama.NewEmbedder(ctx, &embollama.EmbeddingConfig{
			BaseURL: ec.BaseURL,
			Model:   ec.Model,
			Timeout: 60 * time.Second,
		})
	default:
		return nil, fmt.Errorf("factory: 不支持的 embedding provider %q", ec.Provider)
	}
}

// newToolCallingModel 按 provider 创建支持工具调用的对话模型，Host 路由与下游专家共用。
func newToolCallingModel(ctx context.Context, mc config.ModelConfig) (model.ToolCallingChatModel, error) {
	switch mc.Provider {
	case constant.ProviderOpenAI:
		oc := &mdopenai.ChatModelConfig{
			APIKey:  mc.APIKey,
			BaseURL: mc.BaseURL,
			Model:   mc.Model,
			Timeout: 60 * time.Second,
		}
		// 推理模型(gpt-5/o 系列)不认 max_tokens,须用 max_completion_tokens
		// (含 reasoning 与可见输出),否则走网关默认小上限会把输出截断。
		if mc.MaxTokens > 0 {
			oc.MaxCompletionTokens = &mc.MaxTokens
		}
		if mc.ReasoningEffort != "" {
			oc.ReasoningEffort = mdopenai.ReasoningEffortLevel(mc.ReasoningEffort)
		}
		return mdopenai.NewChatModel(ctx, oc)
	case constant.ProviderOllama:
		return mdollama.NewChatModel(ctx, &mdollama.ChatModelConfig{
			BaseURL: mc.BaseURL,
			Model:   mc.Model,
			Timeout: 60 * time.Second,
		})
	default:
		return nil, fmt.Errorf("factory: 不支持的对话模型 provider %q", mc.Provider)
	}
}
