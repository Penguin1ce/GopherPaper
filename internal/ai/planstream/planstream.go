// Package planstream 是挂 React planner 的 agent 的标签感知事件收集器,供小云雀与论文助教
// agentic 链路共用。planner 把模型输出按 /*PLANNING*/ /*ACTION*/ /*REASONING*/ /*REPLANNING*/
// /*FINAL_ANSWER*/ 分段流出:规划/动作/思考段路由到 plan 流事件给前端面板,只有 FINAL_ANSWER
// 之后的文本作为正文 delta 与最终落库答案,保证 history 存的是干净答案、正文气泡不含标签。
package planstream

import (
	"context"
	"fmt"
	"regexp"
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

// CollectEvents 聚合带 planner 的 runner 事件流。
// 计划栏:partial 文本喂标签切分器,规划/动作/思考段实时外发到计划栏,FINAL_ANSWER 正文段作为
// 应答增量流式进气泡(逐字出);切分器跨轮带 Reset 保证气泡只展示末轮正文。
// 最终答案:ReAct 多轮里只保留「最后一轮」的内容 —— 终轮即用户可见结论,中间轮是思考与工具调用,
// 不能拼接(否则把过程当结论吐出);再经 extractFinalAnswer 剥可能的标签。这样不依赖模型守标签协议:
// doubao 常只打 PLANNING、后续用自然语言叙述,取末轮仍能拿到干净答案。
func CollectEvents(ctx context.Context, ch <-chan *event.Event) (string, error) {
	emit := core.StreamFrom(ctx)
	sp := newPlanSplitter(emit)
	var lastContent string
	for ev := range ch {
		if ev.Error != nil {
			return "", fmt.Errorf("planstream: %s", ev.Error.Message)
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
		return "", fmt.Errorf("planstream: 模型返回空内容")
	}
	return answer, nil
}

// planSplitter 是有状态的流式标签切分器:增量逐段喂入,按当前段把文本路由到 plan/delta 事件。
// 标签可能跨增量边界,故保留可能是半个标签的尾巴(holdback),凑齐再判定。
type planSplitter struct {
	emit        core.StreamHandler
	buf         string // 尚未分类的尾巴,可能含半个标签
	section     string // 当前段 phase,空串表示 FINAL_ANSWER 正文
	turnHadBody bool   // 当前轮是否已外发过正文增量
	everHadBody bool   // 全程是否已外发过正文增量(用于跨轮重置)
}

func newPlanSplitter(emit core.StreamHandler) *planSplitter {
	return &planSplitter{emit: emit}
}

// endTurn 在一个助手轮次收尾时重置:丢弃可能半截的尾巴,段位归正文。
// 这样下一轮没有显式标签的自然语言叙述默认归正文(不外发),不会污染计划栏。
func (s *planSplitter) endTurn() {
	s.buf = ""
	s.section = ""
	s.turnHadBody = false
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

// flush 把规划/动作/思考段外发到计划栏,把 FINAL_ANSWER 正文段作为应答增量流式上屏(气泡逐字出)。
// 多轮 ReAct 只展示末轮(与 CollectEvents 的 lastContent 覆盖式一致):新一轮首个正文增量带 Reset,
// 让前端先清空上一轮已流式的正文再追加;done 事件最终仍以 extractFinalAnswer 的干净答案覆盖落库。
func (s *planSplitter) flush(text string) {
	if text == "" || s.emit == nil {
		return
	}
	if s.section == "" {
		reset := !s.turnHadBody && s.everHadBody
		s.turnHadBody = true
		s.everHadBody = true
		s.emit(core.StreamEvent{Kind: constant.StreamEventDelta, Delta: text, Reset: reset})
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

const finalAnswerPrefix = "FINAL ANSWER:"

// extractFinalAnswer 从全程模型输出里取最终答案:
//  1. 优先取最后一个 FINAL_ANSWER 标签之后的文本(模型守协议时最干净)。
//  2. 退而取 "FINAL ANSWER:" 之后。
//  3. 模型没打 FINAL_ANSWER 却带了规划/思考/动作标签(doubao 常见):这些段全是过程,
//     面向用户的结论在最后一个过程标签之后,取它并丢掉残留的「我将…」动作旁白,
//     避免把内部思考与动作叙述当结论吐进气泡。
//  4. 全程无标签:整体即答案。
func extractFinalAnswer(full string) string {
	full = strings.TrimSpace(full)
	if full == "" {
		return ""
	}
	if idx := strings.LastIndex(full, react.FinalAnswerTag); idx >= 0 {
		if ans := strings.TrimSpace(full[idx+len(react.FinalAnswerTag):]); ans != "" {
			return ans
		}
	}
	if up := strings.ToUpper(full); strings.Contains(up, finalAnswerPrefix) {
		idx := strings.LastIndex(up, finalAnswerPrefix)
		if ans := strings.TrimSpace(full[idx+len(finalAnswerPrefix):]); ans != "" {
			return ans
		}
	}
	if idx, tag := lastProcessTag(full); idx >= 0 {
		// 取最后过程标签之后的文本;若尾随一个空的 FINAL_ANSWER 标签先剥掉,再去旁白。
		rest := strings.TrimSpace(strings.ReplaceAll(full[idx+len(tag):], react.FinalAnswerTag, ""))
		if rest != "" {
			return dropLeadingIntent(rest)
		}
	}
	return full
}

// lastProcessTag 找最后一个过程标签(规划/重规划/动作/思考,不含 FINAL_ANSWER)的下标与字面量,无则 -1。
func lastProcessTag(s string) (int, string) {
	best, bestTag := -1, ""
	for _, t := range planTags {
		if t.phase == "" {
			continue // 跳过 FINAL_ANSWER,它在上层单独处理
		}
		if i := strings.LastIndex(s, t.tag); i > best {
			best, bestTag = i, t.tag
		}
	}
	return best, bestTag
}

// paraSep 按空行切分段落。
var paraSep = regexp.MustCompile(`\n\s*\n`)

// intentPrefixes 是动作旁白的开头,如「我将询问用户…」这类面向自己的叙述,而非面向用户的结论。
var intentPrefixes = []string{
	"我将", "我会", "我先", "我准备", "接下来我", "下面我",
	"I will ", "I'll ", "I am going to ", "I'm going to ",
}

// dropLeadingIntent 丢掉答案开头的动作旁白段(如「我将询问用户…」),保留其后真正面向用户的结论。
// 仅在后面还有段落时才丢,避免把唯一一段误删空;段间以空行分隔。
func dropLeadingIntent(s string) string {
	raw := paraSep.Split(s, -1)
	paras := make([]string, 0, len(raw))
	for _, p := range raw {
		if p = strings.TrimSpace(p); p != "" {
			paras = append(paras, p)
		}
	}
	i := 0
	for i < len(paras)-1 && hasAnyPrefix(paras[i], intentPrefixes) {
		i++
	}
	if i == 0 {
		return s
	}
	return strings.Join(paras[i:], "\n\n")
}

func hasAnyPrefix(s string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}
