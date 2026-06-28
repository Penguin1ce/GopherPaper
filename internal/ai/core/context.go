package core

import (
	"context"
	"sync"
)

type paperCtxKey struct{}

type streamCtxKey struct{}

type flowSinkKey struct{}

// FlowSink 收集本轮工具生成的论文思路图,供入口(PioneerChat)取出塞进 reply.Meta 持久化。
// 工具在 runner 协程内写,入口在 runner 结束后读,跨协程故加锁。
type FlowSink struct {
	mu  sync.Mutex
	val any
}

// Set 记录一份思路图(取最后一次)。
func (s *FlowSink) Set(v any) {
	s.mu.Lock()
	s.val = v
	s.mu.Unlock()
}

// Get 取出已记录的思路图,未记录返回 nil。
func (s *FlowSink) Get() any {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.val
}

// WithFlowSink 把思路图收集器注入 context,供工具回传成图结果。
func WithFlowSink(ctx context.Context, s *FlowSink) context.Context {
	return context.WithValue(ctx, flowSinkKey{}, s)
}

// FlowSinkFrom 取出 context 中的思路图收集器,缺失返回 nil。
func FlowSinkFrom(ctx context.Context) *FlowSink {
	if s, ok := ctx.Value(flowSinkKey{}).(*FlowSink); ok {
		return s
	}
	return nil
}

// StreamEvent 是生成过程的一次流式通知:工具调用、工具返回、文本增量或规划阶段文本。
// Kind 取 constant.StreamEventToolCall / StreamEventToolResult / StreamEventDelta / StreamEventPlan。
type StreamEvent struct {
	Kind    string
	Tool    string // 工具名,工具事件时有值
	ToolID  string // 工具调用 id,仅内部用于把 tool_call 与 tool_result 对齐展示名
	Delta   string // 文本增量,delta 与 plan 事件时有值
	Phase   string // 规划阶段(planning/replanning/action/reasoning),plan 事件时有值
	Reset   bool   // delta 事件:true 表示新一轮答案开始,前端先清空已流式正文再追加(先锋者多轮只展示末轮)
	Payload any    // 自定义事件载荷,供需要结构化前端交互的工具使用
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
