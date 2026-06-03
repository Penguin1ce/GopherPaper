package factory

import (
	"context"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"

	"GopherCPP/internal/agent/intent"
	"GopherCPP/internal/config"
	"GopherCPP/pkg/constant"
)

// configPath 测试相对仓库根的配置路径。
const configPath = "../../config/config.toml"

// loadCfg 加载测试配置，缺失时跳过。
func loadCfg(t *testing.T) *config.Config {
	t.Helper()
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Skipf("跳过：读取配置失败 %v", err)
	}
	return cfg
}

// loadFactory 加载配置并构造工厂，要求意图模型为 ollama，否则跳过。
func loadFactory(t *testing.T) *ModelFactory {
	t.Helper()
	cfg := loadCfg(t)
	if cfg.Models.Intent.Provider != constant.ProviderOllama {
		t.Skipf("跳过：意图模型 provider 非 ollama，当前 %q", cfg.Models.Intent.Provider)
	}
	return NewModelFactory(cfg)
}

// TestNewIntentModel_FromFactory 验证能从工厂拿到 ollama 意图模型。
func TestNewIntentModel_FromFactory(t *testing.T) {
	f := loadFactory(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	m, err := f.NewIntentModel(ctx)
	if err != nil {
		t.Fatalf("创建意图模型失败 %v", err)
	}
	if m == nil {
		t.Fatal("意图模型为 nil")
	}
}

// classifyCase 一条意图分类样本，want 为期望子类。
type classifyCase struct {
	name  string
	query string
	want  constant.IntentType
}

// TestIntentClassify 跑通模板到模型到 Parse 的完整链路，检查分类是否有效。
// 需本地 ollama 运行，连不上则跳过。
func TestIntentClassify(t *testing.T) {
	f := loadFactory(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	m, err := f.NewIntentModel(ctx)
	if err != nil {
		t.Fatalf("创建意图模型失败 %v", err)
	}

	tpl := intent.Template()

	cases := []classifyCase{
		{
			"概念讲解",
			"C++ 里的引用和指针有什么区别？分别适合在什么场景使用？",
			constant.IntentConcept,
		},
		{
			"报错调试",
			"我的 C++ 程序运行时报 Segmentation fault，代码里用了指针和数组，应该怎么定位这个问题？",
			constant.IntentDebug,
		},
		{
			"代码评审",
			`这段 C++ 排序函数可以运行，帮我看看有没有可以优化和改进的地方：
#include <vector>
using namespace std;
void bubbleSort(vector<int>& arr) {
    int n = arr.size();
    for (int i = 0; i < n; i++) {
        bool swapped = false;
        for (int j = 0; j < n - i - 1; j++) {
            if (arr[j] > arr[j + 1]) {
                int temp = arr[j];
                arr[j] = arr[j + 1];
                arr[j + 1] = temp;
                swapped = true;
            }
        }
        if (!swapped) {
            break;
        }
    }
}`,
			constant.IntentReview,
		},
	}

	hit := 0
	for _, c := range cases {
		msgs, err := tpl.Format(ctx, map[string]any{"query": c.query})
		if err != nil {
			t.Fatalf("[%s] 渲染模板失败 %v", c.name, err)
		}

		out, err := m.Generate(ctx, msgs)
		if err != nil {
			t.Skipf("跳过：调用 ollama 失败，确认本地服务已启动 %v", err)
		}

		got := intent.Parse(out)
		if !got.Type.IsChat() {
			t.Errorf("[%s] Parse 结果非答疑子类 type=%q raw=%q", c.name, got.Type, out.Content)
			continue
		}
		if got.Type == c.want {
			hit++
		} else {
			// 小模型可能误判，记录但不直接判失败，用命中率衡量有效性。
			t.Logf("[%s] 期望 %q 实际 %q raw=%q", c.name, c.want, got.Type, out.Content)
		}
	}

	t.Logf("意图分类命中 %d/%d", hit, len(cases))
	if hit == 0 {
		t.Errorf("意图分类全部未命中，分类器疑似失效")
	}
}

// TestNewChatModel_API 验证能从工厂拿到走 API 的下游大模型并真实可用。
// 需配置有效的 chat 模型与网络，缺配置或调用失败则跳过。
func TestNewChatModel_API(t *testing.T) {
	cfg := loadCfg(t)
	if cfg.Models.Chat.Provider == constant.ProviderOllama {
		t.Skipf("跳过：chat 模型为本地 ollama，非 API 模型")
	}
	if cfg.Models.Chat.APIKey == "" {
		t.Skip("跳过：未配置 chat 模型 api_key")
	}
	f := NewModelFactory(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	m, err := f.NewChatModel(ctx)
	if err != nil {
		t.Fatalf("创建 API chat 模型失败 %v", err)
	}
	if m == nil {
		t.Fatal("chat 模型为 nil")
	}

	out, err := m.Generate(ctx, []*schema.Message{
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
