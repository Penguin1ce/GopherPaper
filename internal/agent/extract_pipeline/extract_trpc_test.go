package extract_pipeline

import (
	"context"
	"strings"
	"testing"
	"time"

	"GopherPaper/internal/agent"
	"GopherPaper/internal/config"
	"GopherPaper/internal/factory"
)

// TestExtractTRPC 验证 extract 链路端到端输出结构化信息。
// 跑真实网关，缺配置则 skip。
func TestExtractTRPC(t *testing.T) {
	cfg, err := config.Load("../../../config/config.toml")
	if err != nil {
		t.Skipf("跳过:未找到可用 config.toml: %v", err)
	}
	mc := cfg.Models.Chat
	if mc.APIKey == "" || mc.BaseURL == "" {
		t.Skip("跳过:chat 模型未配置网关或密钥")
	}
	m := factory.NewTRPCChatModel(mc)

	doc := &agent.ParsedDoc{
		Paragraphs: []agent.Paragraph{
			{SectionPath: "Abstract", Text: "本文提出一种基于图神经网络的论文引用预测方法 GopherGNN,通过双塔编码器联合建模文本语义与引用拓扑,在三个公开数据集上相比基线提升 8.3% 的 MAP。", PageNo: 1},
			{SectionPath: "Method", Text: "GopherGNN 由文本编码器与图编码器组成,二者输出经对比学习对齐,损失函数为 InfoNCE。", PageNo: 2},
			{SectionPath: "Results", Text: "在 DBLP/PubMed/arXiv 上,GopherGNN 的 MAP 分别为 0.71/0.68/0.65,均优于 GCN 与 BERT 基线。", PageNo: 5},
		},
		PageCount: 8,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	got, err := ExtractTRPC(ctx, m, mc, doc)
	if err != nil {
		t.Fatalf("ExtractTRPC 失败: %v", err)
	}
	if strings.TrimSpace(got.Title) == "" && strings.TrimSpace(got.Abstract) == "" {
		t.Fatalf("抽取结果为空: %+v", got)
	}
	t.Logf("trpc extract 跑通: title=%q methods_len=%d innovations=%d", got.Title, len(got.Methods), len(got.Innovations))
}
