// planstream.go 是小云雀挂 React planner 后的标签感知收集器。
// planner 把模型输出按 /*PLANNING*/ /*ACTION*/ /*REASONING*/ /*REPLANNING*/ /*FINAL_ANSWER*/
// 分段流出:规划/动作/思考段路由到 plan 流事件给前端面板,只有 FINAL_ANSWER 之后的文本
// 作为正文 delta 与最终落库答案,保证 history 存的是干净答案、正文气泡不含标签。
package pioneer

import (
	"context"
	"fmt"
	"strings"

	"trpc.group/trpc-go/trpc-agent-go/event"
	trpcmodel "trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/planner/react"

	"GopherPaper/internal/ai/core"
	"GopherPaper/pkg/constant"
)

// planTags 把 planner 标签映射到对外 phase,FINAL_ANSWER 段 phase 为空表示走正文 delta。
var planTags = []struct{ tag, phase string }{
	{react.PlanningTag, "planning"},
	{react.ReplanningTag, "replanning"},
	{react.ActionTag, "action"},
	{react.ReasoningTag, "reasoning"},
	{react.FinalAnswerTag, ""},
}

// collectPlanEvents 聚合带 planner 的 runner 事件流。
// 计划栏:partial 文本喂标签切分器,只把规划/动作/思考段实时外发(正文段不进气泡)。
// 最终答案:ReAct 多轮里只保留「最后一轮」的内容 —— 终轮即用户可见结论,中间轮是思考与工具调用,
// 不能拼接(否则把过程当结论吐出);再经 extractFinalAnswer 剥可能的标签。这样不依赖模型守标签协议:
// doubao 常只打 PLANNING、后续用自然语言叙述,取末轮仍能拿到干净答案。
func collectPlanEvents(ctx context.Context, ch <-chan *event.Event) (string, error) {
	emit := core.StreamFrom(ctx)
	sp := newPlanSplitter(emit)
	var lastContent string
	for ev := range ch {
		if ev.Error != nil {
			return "", fmt.Errorf("pioneer: %s", ev.Error.Message)
		}
		if ev.Object == trpcmodel.ObjectTypeToolResponse {
			if emit != nil {
				for _, c := range ev.Choices {
					if c.Message.ToolName != "" {
						emit(core.StreamEvent{Kind: constant.StreamEventToolResult, Tool: c.Message.ToolName})
					}
				}
			}
			continue
		}
		if ev.IsRunnerCompletion() {
			continue
		}
		for _, c := range ev.Choices {
			if ev.IsPartial {
				if emit != nil && c.Delta.Content != "" {
					sp.feed(c.Delta.Content)
				}
				continue
			}
			// 非 partial:一个完整助手轮次收尾。
			if emit != nil {
				for _, tc := range c.Message.ToolCalls {
					emit(core.StreamEvent{Kind: constant.StreamEventToolCall, Tool: tc.Function.Name})
				}
			}
			if strings.TrimSpace(c.Message.Content) != "" {
				lastContent = c.Message.Content // 覆盖式,只留最后一轮
			}
			sp.endTurn() // 轮次边界重置,防止下一轮无标签内容污染计划栏
		}
	}
	answer := extractFinalAnswer(lastContent)
	if answer == "" {
		return "", fmt.Errorf("pioneer: 模型返回空内容")
	}
	return answer, nil
}

// planSplitter 是有状态的流式标签切分器:增量逐段喂入,按当前段把文本路由到 plan/delta 事件。
// 标签可能跨增量边界,故保留可能是半个标签的尾巴(holdback),凑齐再判定。
type planSplitter struct {
	emit    core.StreamHandler
	buf     string // 尚未分类的尾巴,可能含半个标签
	section string // 当前段 phase,空串表示 FINAL_ANSWER 正文
}

func newPlanSplitter(emit core.StreamHandler) *planSplitter {
	return &planSplitter{emit: emit}
}

// endTurn 在一个助手轮次收尾时重置:丢弃可能半截的尾巴,段位归正文。
// 这样下一轮没有显式标签的自然语言叙述默认归正文(不外发),不会污染计划栏。
func (s *planSplitter) endTurn() {
	s.buf = ""
	s.section = ""
}

// feed 吞入一段文本增量,尽可能把已能判定的部分按当前段外发。
func (s *planSplitter) feed(delta string) {
	s.buf += delta
	for {
		idx, tag, phase := earliestTag(s.buf)
		if idx < 0 {
			// 没有完整标签:外发安全前缀,保留可能是半个标签的尾巴。
			out, hold := splitSafe(s.buf)
			s.flush(out)
			s.buf = hold
			return
		}
		s.flush(s.buf[:idx]) // 标签前的文本属当前段
		s.section = phase    // 切段并丢弃标签本身
		s.buf = s.buf[idx+len(tag):]
	}
}

// flush 只把规划/动作/思考段外发到计划栏;正文段(section 为空)不外发 ——
// 最终答案统一由 collectPlanEvents 取末轮内容,气泡不做正文流式,避免把中间叙述当结论上屏。
func (s *planSplitter) flush(text string) {
	if text == "" || s.emit == nil || s.section == "" {
		return
	}
	s.emit(core.StreamEvent{Kind: constant.StreamEventPlan, Phase: s.section, Delta: text})
}

// earliestTag 找 buf 中最靠前的完整标签,返回其下标、字面量与对应 phase;无则 idx<0。
func earliestTag(buf string) (int, string, string) {
	best := -1
	var bestTag, bestPhase string
	for _, t := range planTags {
		i := strings.Index(buf, t.tag)
		if i >= 0 && (best < 0 || i < best) {
			best, bestTag, bestPhase = i, t.tag, t.phase
		}
	}
	return best, bestTag, bestPhase
}

// splitSafe 把 buf 切成可立即外发的前缀与需保留的尾巴:尾巴是 buf 末尾能成为某标签前缀的
// 最长子串(再来几个字符就可能凑成完整标签),其余安全外发。
func splitSafe(buf string) (out, hold string) {
	maxHold := 0
	for _, t := range planTags {
		max := min(len(t.tag)-1, len(buf))
		for n := max; n > maxHold; n-- {
			if strings.HasPrefix(t.tag, buf[len(buf)-n:]) {
				maxHold = n
				break
			}
		}
	}
	return buf[:len(buf)-maxHold], buf[len(buf)-maxHold:]
}

// extractFinalAnswer 从全程模型输出里取最终答案:优先取最后一个 FINAL_ANSWER 标签之后的文本,
// 退而求其次取 "FINAL ANSWER:" 之后,再兜底剥离所有标签返回剩余。
func extractFinalAnswer(full string) string {
	full = strings.TrimSpace(full)
	if full == "" {
		return ""
	}
	if idx := strings.LastIndex(full, react.FinalAnswerTag); idx >= 0 {
		return strings.TrimSpace(full[idx+len(react.FinalAnswerTag):])
	}
	if up := strings.ToUpper(full); strings.Contains(up, "FINAL ANSWER:") {
		idx := strings.LastIndex(up, "FINAL ANSWER:")
		return strings.TrimSpace(full[idx+len("FINAL ANSWER:"):])
	}
	return strings.TrimSpace(stripTags(full))
}

// stripTags 剥掉所有已知 planner 标签,仅在模型没打 FINAL_ANSWER 时兜底用。
func stripTags(s string) string {
	for _, t := range planTags {
		if t.tag != "" {
			s = strings.ReplaceAll(s, t.tag, "")
		}
	}
	return s
}
