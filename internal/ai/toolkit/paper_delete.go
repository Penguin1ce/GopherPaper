package toolkit

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"trpc.group/trpc-go/trpc-agent-go/tool"
	"trpc.group/trpc-go/trpc-agent-go/tool/function"

	"GopherPaper/internal/ai/core"
	"GopherPaper/internal/model"
	"GopherPaper/internal/tenant"
	"GopherPaper/pkg/constant"
	"GopherPaper/pkg/errs"
)

type deletePaperInput struct {
	PaperID string `json:"paper_id" jsonschema:"description=要删除的论文 id,必须来自 list_my_papers 的结果,required"`
}

type deletePaperOutput struct {
	PaperID string `json:"paper_id" jsonschema:"description=已删除的论文 id"`
	Title   string `json:"title" jsonschema:"description=已删除的论文标题或文件名"`
	Status  string `json:"status" jsonschema:"description=删除操作状态,可能为 confirmation_required 或 deleted"`
	Message string `json:"message" jsonschema:"description=面向用户的删除结果提示"`
}

type paperDeleteConfirmCtxKey struct{}

const paperDeleteConfirmationTTL = 10 * time.Minute

type pendingPaperDeleteConfirmation struct {
	OwnerID   string
	PaperID   string
	ExpiresAt time.Time
}

var pendingPaperDeleteConfirmations sync.Map

func newDeletePaperTool() tool.Tool {
	return function.NewFunctionTool(deleteMyPaper,
		function.WithName("delete_my_paper"),
		function.WithDescription("删除当前用户论文库中的一篇论文及其派生数据。强副作用工具:必须先用 list_my_papers 定位唯一论文并说明影响范围。首次调用会请求前端弹出删除确认框;只有用户在弹窗中确认后才会真正删除。"),
	)
}

// WithPaperDeleteConfirmation 把前端删除确认弹窗回传的一次性令牌注入工具上下文。
func WithPaperDeleteConfirmation(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, paperDeleteConfirmCtxKey{}, strings.TrimSpace(token))
}

func deleteMyPaper(ctx context.Context, in deletePaperInput) (deletePaperOutput, error) {
	if deletePaperForTool == nil {
		return deletePaperOutput{}, fmt.Errorf("delete_my_paper: 论文删除能力未就绪")
	}
	owner := tenant.MustStudentID(ctx)
	if owner == "" {
		return deletePaperOutput{}, fmt.Errorf("delete_my_paper: 缺少当前用户身份")
	}
	paperID := strings.TrimSpace(in.PaperID)
	if paperID == "" {
		return deletePaperOutput{}, fmt.Errorf("delete_my_paper: paper_id 不能为空")
	}

	p, err := getPaperForTool(ctx, paperID)
	if err != nil {
		return deletePaperOutput{}, fmt.Errorf("delete_my_paper: 查询论文失败: %w", err)
	}
	if p.OwnerID != owner {
		return deletePaperOutput{}, fmt.Errorf("delete_my_paper: %w", errs.ErrPaperForbidden)
	}
	title := displayPaperTitle(p)
	if !consumePaperDeleteConfirmation(paperDeleteConfirmationFrom(ctx), owner, paperID) {
		token, err := newPaperDeleteConfirmation(owner, paperID)
		if err != nil {
			return deletePaperOutput{}, fmt.Errorf("delete_my_paper: 创建删除确认令牌失败: %w", err)
		}
		emitPaperDeleteConfirmation(ctx, p, token)
		return deletePaperOutput{
			PaperID: paperID,
			Title:   title,
			Status:  "confirmation_required",
			Message: "已向用户弹出删除确认框。请停止调用工具,等待用户点击确认后再继续。",
		}, nil
	}
	if err := deletePaperForTool(ctx, owner, paperID); err != nil {
		return deletePaperOutput{}, fmt.Errorf("delete_my_paper: 删除论文失败: %w", err)
	}
	return deletePaperOutput{
		PaperID: paperID,
		Title:   title,
		Status:  "deleted",
		Message: fmt.Sprintf("已删除《%s》及其绑定会话、报告、图片、向量索引和知识图谱节点。", title),
	}, nil
}

func displayPaperTitle(p *model.Paper) string {
	if p == nil {
		return ""
	}
	if title := strings.TrimSpace(p.Title); title != "" {
		return title
	}
	if name := strings.TrimSpace(p.FileName); name != "" {
		return name
	}
	return p.ID
}

func paperDeleteConfirmationFrom(ctx context.Context) string {
	if token, ok := ctx.Value(paperDeleteConfirmCtxKey{}).(string); ok {
		return strings.TrimSpace(token)
	}
	return ""
}

func newPaperDeleteConfirmation(ownerID, paperID string) (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	token := hex.EncodeToString(b[:])
	now := time.Now()
	pendingPaperDeleteConfirmations.Store(token, pendingPaperDeleteConfirmation{
		OwnerID:   ownerID,
		PaperID:   paperID,
		ExpiresAt: now.Add(paperDeleteConfirmationTTL),
	})
	cleanupExpiredPaperDeleteConfirmations(now)
	return token, nil
}

func consumePaperDeleteConfirmation(token, ownerID, paperID string) bool {
	token = strings.TrimSpace(token)
	if token == "" {
		return false
	}
	now := time.Now()
	v, ok := pendingPaperDeleteConfirmations.Load(token)
	if !ok {
		cleanupExpiredPaperDeleteConfirmations(now)
		return false
	}
	pendingPaperDeleteConfirmations.Delete(token)
	pending, ok := v.(pendingPaperDeleteConfirmation)
	if !ok || now.After(pending.ExpiresAt) {
		cleanupExpiredPaperDeleteConfirmations(now)
		return false
	}
	return pending.OwnerID == ownerID && pending.PaperID == paperID
}

func cleanupExpiredPaperDeleteConfirmations(now time.Time) {
	pendingPaperDeleteConfirmations.Range(func(key, value any) bool {
		pending, ok := value.(pendingPaperDeleteConfirmation)
		if !ok || now.After(pending.ExpiresAt) {
			pendingPaperDeleteConfirmations.Delete(key)
		}
		return true
	})
}

func emitPaperDeleteConfirmation(ctx context.Context, p *model.Paper, token string) {
	stream := core.StreamFrom(ctx)
	if stream == nil || p == nil {
		return
	}
	stream(core.StreamEvent{
		Kind: constant.StreamEventConfirmDeletePaper,
		Payload: map[string]string{
			"paper_id":           p.ID,
			"title":              displayPaperTitle(p),
			"file_name":          p.FileName,
			"status":             string(p.Status),
			"confirmation_token": token,
			"message":            "确认后将删除论文、绑定会话、报告、图片、向量索引和知识图谱节点。",
		},
	})
}
