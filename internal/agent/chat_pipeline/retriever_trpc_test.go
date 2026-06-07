package chat_pipeline

import (
	"context"
	"testing"
	"time"

	"github.com/milvus-io/milvus/client/v2/milvusclient"

	"GopherPaper/internal/config"
	"GopherPaper/internal/factory"
	"GopherPaper/internal/knowledge"
	"GopherPaper/pkg/constant"
)

// TestRetrieveForPaperBridge 验证阶段B 桥接:UpsertChunksTRPC 写入 → RetrieveForPaper 经
// trpc vectorstore 检索并转回 schema.Document → References 还原出处。需 Milvus+Ollama。
func TestRetrieveForPaperBridge(t *testing.T) {
	cfg, err := config.Load("../../../config/config.toml")
	if err != nil {
		t.Skipf("跳过:未找到 config.toml: %v", err)
	}
	ctx := context.Background()
	emb := factory.NewTRPCEmbedder(cfg.Embedding)
	if _, e := emb.GetEmbedding(ctx, "ping"); e != nil {
		t.Skipf("跳过:Ollama embedder 不可用: %v", e)
	}

	const coll = "knowledge_chat_bridge_test"
	dropBridgeCollection(ctx, cfg.Milvus, coll)
	if err := knowledge.InitTRPCStore(ctx, cfg.Milvus, coll, emb, cfg.Embedding.Dim); err != nil {
		t.Fatalf("InitTRPCStore: %v", err)
	}
	defer dropBridgeCollection(ctx, cfg.Milvus, coll)

	if _, err := knowledge.UpsertChunksTRPC(ctx, []knowledge.Chunk{
		{Scope: constant.KnowledgeScopePublic, Content: "注意力机制是一种重要的深度学习建模方法", SourceFile: "pub.pdf", PageNo: 1},
		{Scope: constant.KnowledgeScopePrivate, OwnerID: "userX", DocID: "docX", Content: "本文提出图神经网络方法用于论文引用预测", SourceFile: "x.pdf", PageNo: 2},
	}); err != nil {
		t.Fatalf("UpsertChunksTRPC: %v", err)
	}

	var docs []*Doc
	for range 20 {
		docs, err = RetrieveForPaper(ctx, "论文的研究方法", "userX", "")
		if err != nil {
			t.Fatalf("RetrieveForPaper: %v", err)
		}
		if len(docs) >= 2 {
			break
		}
		time.Sleep(time.Second)
	}
	if len(docs) == 0 {
		t.Fatal("检索为空")
	}

	refs := References(docs)
	if len(refs) != len(docs) {
		t.Fatalf("References 数量不符: docs=%d refs=%d", len(docs), len(refs))
	}
	hasSource := false
	for _, r := range refs {
		if r.SourceFile != "" && r.Scope != "" {
			hasSource = true
		}
	}
	if !hasSource {
		t.Fatalf("出处缺 scope/source_file: %+v", refs)
	}
	t.Logf("阶段B 检索桥接 OK: docs=%d refs=%d", len(docs), len(refs))
}

func dropBridgeCollection(ctx context.Context, mc config.MilvusConfig, name string) {
	c, err := milvusclient.New(ctx, &milvusclient.ClientConfig{Address: mc.Address, Username: mc.Username, Password: mc.Password})
	if err != nil {
		return
	}
	defer c.Close(ctx)
	_ = c.DropCollection(ctx, milvusclient.NewDropCollectionOption(name))
}
