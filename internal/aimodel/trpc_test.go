package aimodel

import (
	"context"
	"strings"
	"testing"
	"time"

	trpcmodel "trpc.group/trpc-go/trpc-agent-go/model"

	"GopherPaper/internal/config"
)

// loadTestConfig 读项目真实配置驱动连通性验证,缺失则跳过(免无网络/无配置环境挂)。
func loadTestConfig(t *testing.T) *config.Config {
	t.Helper()
	cfg, err := config.Load("../../config/config.toml")
	if err != nil {
		t.Skipf("跳过:未找到可用 config.toml: %v", err)
	}
	return cfg
}

// TestTRPCChatModelConnectivity 验证 trpc openai 兼容 model 能连网关并产出非空回答。
func TestTRPCChatModelConnectivity(t *testing.T) {
	cfg := loadTestConfig(t)
	mc := cfg.Models.Chat
	if mc.APIKey == "" || mc.BaseURL == "" {
		t.Skip("跳过:chat 模型未配置网关或密钥")
	}
	m := NewChatModel(mc)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	ch, err := m.GenerateContent(ctx, &trpcmodel.Request{
		Messages: []trpcmodel.Message{trpcmodel.NewUserMessage("用一句话回答:你好")},
	})
	if err != nil {
		t.Fatalf("GenerateContent 失败: %v", err)
	}

	var sb strings.Builder
	for rsp := range ch {
		if rsp.Error != nil {
			t.Fatalf("响应错误: %s", rsp.Error.Message)
		}
		for _, c := range rsp.Choices {
			sb.WriteString(c.Message.Content)
			sb.WriteString(c.Delta.Content)
		}
	}
	if strings.TrimSpace(sb.String()) == "" {
		t.Fatal("模型返回空内容")
	}
	t.Logf("trpc chat 模型连通,返回: %s", strings.TrimSpace(sb.String()))
}

// TestTRPCEmbedderConnectivity 验证 bge-m3 经 Ollama OpenAI 兼容端点能向量化且维度对齐。
func TestTRPCEmbedderConnectivity(t *testing.T) {
	cfg := loadTestConfig(t)
	ec := cfg.Embedding
	if ec.BaseURL == "" {
		t.Skip("跳过:embedding 未配置")
	}
	// Ollama 走 OpenAI 兼容端点须补 /v1;兼容端点不校验密钥,占位即可。
	ec.BaseURL = strings.TrimRight(ec.BaseURL, "/") + "/v1"
	if ec.APIKey == "" {
		ec.APIKey = "ollama"
	}
	emb := NewEmbedder(ec)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	vec, err := emb.GetEmbedding(ctx, "科研文献智能解析与知识服务")
	if err != nil {
		t.Skipf("跳过:embedder 连通失败(Ollama 未启动?): %v", err)
	}
	if len(vec) != ec.Dim {
		t.Fatalf("向量维度不符,期望 %d 实际 %d", ec.Dim, len(vec))
	}
	t.Logf("trpc embedder 连通,bge-m3 维度 %d", len(vec))
}
