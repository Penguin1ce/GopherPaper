package knowledge

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/milvus-io/milvus/client/v2/milvusclient"
	"trpc.group/trpc-go/trpc-agent-go/knowledge/vectorstore"

	"GopherPaper/internal/aimodel"
	"GopherPaper/internal/config"
	"GopherPaper/pkg/constant"
)

// TestTRPCStoreEndToEnd 在真实 Milvus 上验证 collection 初始化、写入和多租户检索过滤。
// 同时校准 searchfilter 字段前缀。缺 Milvus 或 Ollama 则 skip。
func TestTRPCStoreEndToEnd(t *testing.T) {
	cfg, err := config.Load("../../config/config.toml")
	if err != nil {
		t.Skipf("跳过:未找到 config.toml: %v", err)
	}
	ctx := context.Background()

	ec := cfg.Embedding // config 的 base_url 已含 /v1,apikey 占位由 NewEmbedder 处理
	emb := aimodel.NewEmbedder(ec)
	if _, e := emb.GetEmbedding(ctx, "ping"); e != nil {
		t.Skipf("跳过:Ollama embedder 不可用: %v", e)
	}

	const collection = "knowledge_trpc_e2e"
	dropTestCollection(ctx, cfg.Milvus, collection)
	if err := InitTRPCStore(ctx, cfg.Milvus, collection, emb, ec.Dim); err != nil {
		t.Fatalf("InitTRPCStore 失败(可能 Milvus 版本不支持 BM25): %v", err)
	}
	defer dropTestCollection(ctx, cfg.Milvus, collection)

	mustAdd := func(scope constant.KnowledgeScope, owner, doc, content string) {
		if err := AddChunkTRPC(ctx, Chunk{Scope: scope, OwnerID: owner, DocID: doc, Content: content, SourceFile: "t.pdf"}); err != nil {
			t.Fatalf("AddChunkTRPC(%s/%s): %v", scope, owner, err)
		}
	}
	mustAdd(constant.KnowledgeScopePublic, "", "pubdoc", "深度学习中的注意力机制是一种重要的建模方法")
	mustAdd(constant.KnowledgeScopePrivate, "userA", "docA", "用户A的私有论文研究图神经网络与引用预测方法")
	mustAdd(constant.KnowledgeScopePrivate, "userB", "docB", "用户B的私有论文研究强化学习的奖励建模")

	// 写入后数据可见有延迟,重试等到 public+A 两条都可见(B 应被过滤,故 userA 最多 2 条)。
	var res *vectorstore.SearchResult
	for range 20 {
		r, e := SearchTRPC(ctx, "论文的研究方法", "userA", "", 10)
		if e != nil {
			t.Fatalf("SearchTRPC 失败: %v", e)
		}
		res = r
		if len(r.Results) >= 2 {
			break
		}
		time.Sleep(time.Second)
	}
	if res == nil || len(res.Results) == 0 {
		t.Fatal("SearchTRPC 未召回任何片段(写入可见性或过滤异常)")
	}

	sawPublic, sawA := false, false
	for _, r := range res.Results {
		scope := fmt.Sprint(r.Document.Metadata[constant.MilvusFieldKnowledgeScope])
		sid := fmt.Sprint(r.Document.Metadata[constant.MilvusFieldStudentID])
		if scope == string(constant.KnowledgeScopePrivate) && sid == "userB" {
			t.Fatalf("越权:userA 检索到了 userB 的私有片段 content=%q", r.Document.Content)
		}
		if scope == string(constant.KnowledgeScopePublic) {
			sawPublic = true
		}
		if scope == string(constant.KnowledgeScopePrivate) && sid == "userA" {
			sawA = true
		}
	}
	if !sawPublic || !sawA {
		t.Fatalf("可见性不完整:sawPublic=%v sawA=%v(期望都为 true),召回 %d 条", sawPublic, sawA, len(res.Results))
	}
	t.Logf("trpc 多租户检索 OK:召回 %d 条 sawPublic=%v sawA=%v 且无 userB 越权", len(res.Results), sawPublic, sawA)
}

func dropTestCollection(ctx context.Context, mc config.MilvusConfig, name string) {
	c, err := milvusclient.New(ctx, &milvusclient.ClientConfig{Address: mc.Address, Username: mc.Username, Password: mc.Password})
	if err != nil {
		return
	}
	defer c.Close(ctx)
	_ = c.DropCollection(ctx, milvusclient.NewDropCollectionOption(name))
}
