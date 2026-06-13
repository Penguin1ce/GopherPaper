package retrieval

import (
	"context"
	"errors"
	"os"
	"testing"

	trpcreranker "trpc.group/trpc-go/trpc-agent-go/knowledge/reranker"

	"GopherPaper/internal/aimodel"
	"GopherPaper/internal/config"
)

// stubReranker 以注入的 fn 模拟精排,验证 rerankDocs 的排序/截断/退化逻辑,不走网络。
type stubReranker struct {
	fn func([]*trpcreranker.Result) ([]*trpcreranker.Result, error)
}

func (s stubReranker) Rerank(_ context.Context, _ *trpcreranker.Query, results []*trpcreranker.Result) ([]*trpcreranker.Result, error) {
	return s.fn(results)
}

func ids(docs []*Doc) []string {
	out := make([]string, len(docs))
	for i, d := range docs {
		out[i] = d.ID
	}
	return out
}

func eq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestRerankDocs 验证三种路径:reranker 为 nil 退化截断、stub 精排重排后截断、精排失败退化原序截断。
func TestRerankDocs(t *testing.T) {
	orig := reranker
	defer func() { reranker = orig }()

	newDocs := func() []*Doc {
		return []*Doc{{ID: "a"}, {ID: "b"}, {ID: "c"}, {ID: "d"}}
	}
	ctx := context.Background()

	// 1 reranker 为 nil:不重排,仅截断到 topN
	reranker = nil
	got := rerankDocs(ctx, "q", newDocs(), 2)
	if want := []string{"a", "b"}; !eq(ids(got), want) {
		t.Fatalf("nil reranker: got %v want %v", ids(got), want)
	}

	// 2 stub 反转顺序后返回:输出须跟随精排序并截断
	reranker = stubReranker{fn: func(rs []*trpcreranker.Result) ([]*trpcreranker.Result, error) {
		out := make([]*trpcreranker.Result, len(rs))
		for i := range rs {
			out[i] = rs[len(rs)-1-i]
		}
		return out, nil
	}}
	got = rerankDocs(ctx, "q", newDocs(), 2)
	if want := []string{"d", "c"}; !eq(ids(got), want) {
		t.Fatalf("rerank: got %v want %v", ids(got), want)
	}

	// 3 精排失败:退化为向量原序并截断,不阻断
	reranker = stubReranker{fn: func(_ []*trpcreranker.Result) ([]*trpcreranker.Result, error) {
		return nil, errors.New("boom")
	}}
	got = rerankDocs(ctx, "q", newDocs(), 2)
	if want := []string{"a", "b"}; !eq(ids(got), want) {
		t.Fatalf("rerank err fallback: got %v want %v", ids(got), want)
	}

	// 4 候选不足 topN:原样返回
	reranker = nil
	got = rerankDocs(ctx, "q", []*Doc{{ID: "x"}}, 5)
	if want := []string{"x"}; !eq(ids(got), want) {
		t.Fatalf("undersize: got %v want %v", ids(got), want)
	}
}

// TestRerankE2E 实调硅基 rerank API,验证端点/模型/key 可用且重排合理:
// 与 query 语义最相关的文档须被精排到首位。需 RERANK_E2E=1 启用,key 从 config.toml 读不硬编码。
func TestRerankE2E(t *testing.T) {
	if os.Getenv("RERANK_E2E") == "" {
		t.Skip("设 RERANK_E2E=1 跑实调")
	}
	cfg, err := config.Load("../../../config/config.toml")
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	rr := aimodel.NewReranker(cfg.Rerank)
	if rr == nil {
		t.Fatal("NewReranker 返回 nil:检查 [rerank] enabled 与 base_url/model/api_key")
	}
	orig := reranker
	defer func() { reranker = orig }()
	reranker = rr

	docs := []*Doc{
		{ID: "cat", Content: "猫是一种小型哺乳动物，性格独立，常被作为宠物饲养。"},
		{ID: "photo", Content: "光合作用是绿色植物利用光能把二氧化碳和水转化为有机物的过程。"},
		{ID: "attn", Content: "Transformer 是一种基于自注意力机制的深度学习模型架构，广泛用于自然语言处理。"},
		{ID: "weather", Content: "明天多云转晴，最高气温 28 摄氏度，东南风三级。"},
	}
	query := "自注意力机制和神经网络模型架构"
	got := rerankDocs(context.Background(), query, docs, 4)
	if len(got) == 0 {
		t.Fatal("rerank 返回空")
	}
	for i, d := range got {
		t.Logf("#%d id=%s score=%.4f", i+1, d.ID, d.Score)
	}
	if got[0].ID != "attn" {
		t.Fatalf("期望最相关文档 attn 排首位,实得 %s", got[0].ID)
	}
}
