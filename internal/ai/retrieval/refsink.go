package retrieval

import (
	"context"
	"sync"
)

// refSink 是一次 agentic 问答里累积出处的收集器:工具每轮检索把命中出处写入,
// 循环结束后由编排层一次性取出填进 Reply.Meta["sources"]。
// agentic 链路里检索散在多轮工具调用中,无法像固定流那样在编排层直接拿到 docs,故用 ctx 收集器透传。
// 按 Reference.ID 去重(多轮可能反复命中同一片段),保留首次出现顺序;带锁防并行工具调用并发写。
type refSink struct {
	mu   sync.Mutex
	seen map[string]struct{}
	refs []Reference
}

type refSinkKey struct{}

// WithRefSink 在 ctx 上挂一个出处收集器,供本次 agentic 问答的检索工具累积命中出处。
func WithRefSink(ctx context.Context) context.Context {
	return context.WithValue(ctx, refSinkKey{}, &refSink{seen: map[string]struct{}{}})
}

// AddRefs 把本轮检索的出处累加进 ctx 的收集器,按 ID 去重;ctx 无收集器时静默忽略
// (固定流链路不挂收集器,工具复用时不报错)。
func AddRefs(ctx context.Context, refs []Reference) {
	s, ok := ctx.Value(refSinkKey{}).(*refSink)
	if !ok {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range refs {
		if _, dup := s.seen[r.ID]; dup {
			continue
		}
		s.seen[r.ID] = struct{}{}
		s.refs = append(s.refs, r)
	}
}

// DrainRefs 取出并清空 ctx 收集器里累积的全部出处,循环结束后调用一次;ctx 无收集器时返回 nil。
func DrainRefs(ctx context.Context) []Reference {
	s, ok := ctx.Value(refSinkKey{}).(*refSink)
	if !ok {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.refs
	s.refs = nil
	s.seen = map[string]struct{}{}
	return out
}
