package core

import "context"

type paperCtxKey struct{}

type streamCtxKey struct{}

// StreamEvent 是生成过程的一次流式通知:工具调用、工具返回或文本增量。
// Kind 取 constant.StreamEventToolCall / StreamEventToolResult / StreamEventDelta。
type StreamEvent struct {
	Kind  string
	Tool  string // 工具名,工具事件时有值
	Delta string // 文本增量,delta 事件时有值
}

// StreamHandler 消费流式通知,由 handler 注入,把事件写成 SSE 推给前端。
type StreamHandler func(StreamEvent)

// WithStream 把流式通知回调注入 context,生成段经 CollectEvents 实时回调。
func WithStream(ctx context.Context, h StreamHandler) context.Context {
	return context.WithValue(ctx, streamCtxKey{}, h)
}

// StreamFrom 取出 context 中的流式回调,缺失返回 nil 表示同步聚合不外发。
func StreamFrom(ctx context.Context) StreamHandler {
	if h, ok := ctx.Value(streamCtxKey{}).(StreamHandler); ok {
		return h
	}
	return nil
}

// WithPaperID 把当前问答围绕的论文 ID 注入 context，供检索限定到该论文。
func WithPaperID(ctx context.Context, paperID string) context.Context {
	return context.WithValue(ctx, paperCtxKey{}, paperID)
}

// PaperIDFrom 取出 context 中的论文 ID，缺失返回空串表示跨库问答。
func PaperIDFrom(ctx context.Context) string {
	if v, ok := ctx.Value(paperCtxKey{}).(string); ok {
		return v
	}
	return ""
}
