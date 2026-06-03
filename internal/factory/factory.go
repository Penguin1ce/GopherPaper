// Package factory 用工厂模式按配置创建 eino 的模型与向量化组件。
// 上层只依赖 eino 标准接口，新增 provider 只改这里。
package factory

import (
	"context"
	"fmt"
	"time"

	embollama "github.com/cloudwego/eino-ext/components/embedding/ollama"
	mdollama "github.com/cloudwego/eino-ext/components/model/ollama"
	mdopenai "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/model"

	"GopherCPP/internal/config"
	"GopherCPP/pkg/constant"
)

// ModelFactory 按配置创建对话模型与向量化模型。
type ModelFactory struct {
	cfg *config.Config
}

func NewModelFactory(cfg *config.Config) *ModelFactory {
	return &ModelFactory{cfg: cfg}
}

// NewIntentModel 创建意图识别小模型。
func (f *ModelFactory) NewIntentModel(ctx context.Context) (model.BaseChatModel, error) {
	return f.newChatModel(ctx, f.cfg.Models.Intent)
}

// NewChatModel 创建下游 agent 使用的主力大模型，支持工具调用。
func (f *ModelFactory) NewChatModel(ctx context.Context) (model.ToolCallingChatModel, error) {
	mc := f.cfg.Models.Chat
	switch mc.Provider {
	case constant.ProviderOpenAI:
		return mdopenai.NewChatModel(ctx, &mdopenai.ChatModelConfig{
			APIKey:  mc.APIKey,
			BaseURL: mc.BaseURL,
			Model:   mc.Model,
			Timeout: 60 * time.Second,
		})
	case constant.ProviderOllama:
		return mdollama.NewChatModel(ctx, &mdollama.ChatModelConfig{
			BaseURL: mc.BaseURL,
			Model:   mc.Model,
			Timeout: 60 * time.Second,
		})
	default:
		return nil, fmt.Errorf("factory: 不支持的 chat 模型 provider %q", mc.Provider)
	}
}

// NewEmbedder 创建 RAG 向量化模型。
func (f *ModelFactory) NewEmbedder(ctx context.Context) (embedding.Embedder, error) {
	ec := f.cfg.Embedding
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

// newChatModel 创建 BaseChatModel 的内部通用分发。
func (f *ModelFactory) newChatModel(ctx context.Context, mc config.ModelConfig) (model.BaseChatModel, error) {
	switch mc.Provider {
	case constant.ProviderOllama:
		return mdollama.NewChatModel(ctx, &mdollama.ChatModelConfig{
			BaseURL: mc.BaseURL,
			Model:   mc.Model,
			Timeout: 30 * time.Second,
		})
	case constant.ProviderOpenAI:
		return mdopenai.NewChatModel(ctx, &mdopenai.ChatModelConfig{
			APIKey:  mc.APIKey,
			BaseURL: mc.BaseURL,
			Model:   mc.Model,
			Timeout: 30 * time.Second,
		})
	default:
		return nil, fmt.Errorf("factory: 不支持的模型 provider %q", mc.Provider)
	}
}
