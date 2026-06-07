package report_pipeline

import (
	"context"
	"strings"
	"testing"
	"time"

	"GopherPaper/internal/agent"
	"GopherPaper/internal/config"
	"GopherPaper/internal/factory"
	"GopherPaper/pkg/constant"
)

// TestGenerateReportTRPC 验证 report 链路的检索容错、trpc model 生成和 report_type 标记。
// 跑真实网关，缺配置则 skip。不依赖 Milvus，检索器未初始化时仍可生成无片段报告。
func TestGenerateReportTRPC(t *testing.T) {
	cfg, err := config.Load("../../../config/config.toml")
	if err != nil {
		t.Skipf("跳过:未找到可用 config.toml: %v", err)
	}
	mc := cfg.Models.Chat
	if mc.APIKey == "" || mc.BaseURL == "" {
		t.Skip("跳过:chat 模型未配置网关或密钥")
	}
	m := factory.NewTRPCChatModel(mc)

	in := &agent.ReportInput{
		PaperID:    "test-paper",
		OwnerID:    "test-user",
		ReportType: constant.ReportQuickRead,
		Query:      "图神经网络论文引用预测方法的速读概览",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	reply, err := GenerateReportTRPC(ctx, m, mc, in)
	if err != nil {
		t.Fatalf("GenerateReportTRPC 失败: %v", err)
	}
	if strings.TrimSpace(reply.Content) == "" {
		t.Fatal("报告内容为空")
	}
	if reply.Meta["report_type"] != string(constant.ReportQuickRead) {
		t.Fatalf("report_type 标记错误: %v", reply.Meta["report_type"])
	}
	t.Logf("trpc report 切片跑通,内容长度=%d", len(reply.Content))
}
