package factory

import (
	"context"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"

	"GopherPaper/internal/config"
	"GopherPaper/pkg/constant"
)

// configPath 测试相对仓库根的配置路径。
const configPath = "../../config/config.toml"

// loadCfg 加载测试配置并初始化工厂，缺失时跳过。
func loadCfg(t *testing.T) *config.Config {
	t.Helper()
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Skipf("跳过：读取配置失败 %v", err)
	}
	Init(cfg)
	return cfg
}

// TestModelsForUser_Singleton 验证同 userID 返回同实例、不同 userID 隔离。
func TestModelsForUser_Singleton(t *testing.T) {
	loadCfg(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	a1, err := ModelsForUser(ctx, "u1")
	if err != nil {
		t.Fatalf("创建用户模型失败 %v", err)
	}
	a2, err := ModelsForUser(ctx, "u1")
	if err != nil {
		t.Fatalf("二次获取失败 %v", err)
	}
	if a1 != a2 {
		t.Fatal("同 userID 应返回同一实例")
	}
	b1, err := ModelsForUser(ctx, "u2")
	if err != nil {
		t.Fatalf("创建另一用户模型失败 %v", err)
	}
	if a1 == b1 {
		t.Fatal("不同 userID 应隔离为不同实例")
	}
	if a1.Intent == nil || a1.Chat == nil {
		t.Fatal("模型集合不应有 nil 成员")
	}
}

// TestUserChatModel_API 验证下游大模型走 API 真实可用。
// 需配置有效的 chat 模型与网络，缺配置或调用失败则跳过。
func TestUserChatModel_API(t *testing.T) {
	cfg := loadCfg(t)
	if cfg.Models.Chat.Provider == constant.ProviderOllama {
		t.Skipf("跳过：chat 模型为本地 ollama，非 API 模型")
	}
	if cfg.Models.Chat.APIKey == "" {
		t.Skip("跳过：未配置 chat 模型 api_key")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	um, err := ModelsForUser(ctx, "api-test")
	if err != nil {
		t.Fatalf("创建用户模型失败 %v", err)
	}

	out, err := um.Chat.Generate(ctx, []*schema.Message{
		schema.UserMessage("我是用户测试，你只需要说芝麻开门即可"),
	})
	if err != nil {
		t.Skipf("跳过：调用 API 模型失败，确认网关与密钥可用 %v", err)
	}
	if out == nil || out.Content == "" {
		t.Fatal("API 模型返回内容为空")
	}
	t.Logf("API 模型响应 %q", out.Content)
}
