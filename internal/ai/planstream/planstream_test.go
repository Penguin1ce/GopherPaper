package planstream

import (
	"context"
	"errors"
	"strings"
	"testing"

	"trpc.group/trpc-go/trpc-agent-go/event"
	trpcmodel "trpc.group/trpc-go/trpc-agent-go/model"

	"GopherPaper/internal/ai/core"
	"GopherPaper/pkg/constant"
)

// capture 收集切分器外发的事件:plan 事件按 phase 聚合文本,delta 事件聚合正文并记录 reset 序列。
type capture struct {
	plan    map[string]string
	delta   string
	resets  []bool
	gotName map[string]bool
}

func newCapture() (*planSplitter, *capture) {
	c := &capture{plan: map[string]string{}, gotName: map[string]bool{}}
	sp := newPlanSplitter(func(ev core.StreamEvent) {
		c.gotName[ev.Kind] = true
		switch ev.Kind {
		case constant.StreamEventPlan:
			c.plan[ev.Phase] += ev.Delta
		case constant.StreamEventDelta:
			c.delta += ev.Delta
			c.resets = append(c.resets, ev.Reset)
		}
	})
	return sp, c
}

func TestPlanSplitter_BodyStreamsAsDelta(t *testing.T) {
	// 计划段进 plan 流,FINAL_ANSWER 正文段作为 delta 流式外发(气泡逐字出),且不污染计划栏。
	sp, c := newCapture()
	sp.feed("/*PLANNING*/步骤1\n步骤2/*ACTION*/查论文/*REASONING*/找到了/*FINAL_ANSWER*/这是答案")
	if got := strings.TrimSpace(c.plan["planning"]); got != "步骤1\n步骤2" {
		t.Errorf("planning = %q", got)
	}
	if got := strings.TrimSpace(c.plan["action"]); got != "查论文" {
		t.Errorf("action = %q", got)
	}
	if got := strings.TrimSpace(c.plan["reasoning"]); got != "找到了" {
		t.Errorf("reasoning = %q", got)
	}
	if got := strings.TrimSpace(c.delta); got != "这是答案" {
		t.Errorf("正文应作为 delta 外发, delta = %q", got)
	}
	for _, v := range c.plan {
		if strings.Contains(v, "这是答案") {
			t.Errorf("正文不应进计划栏: %q", v)
		}
		if strings.Contains(v, "/*") {
			t.Errorf("计划段不应含标签碎片: %q", v)
		}
	}
}

func TestPlanSplitter_BodyResetAcrossTurns(t *testing.T) {
	// 多轮 ReAct:第一轮首个正文增量不重置,跨轮后第二轮首个正文增量带 Reset,前端据此只展示末轮。
	sp, c := newCapture()
	sp.feed("/*FINAL_ANSWER*/第一轮答案")
	sp.endTurn()
	sp.feed("/*FINAL_ANSWER*/第二轮答案")
	if len(c.resets) < 2 {
		t.Fatalf("应至少两次 delta, resets = %v", c.resets)
	}
	if c.resets[0] {
		t.Error("首轮首个正文增量不应带 Reset")
	}
	if !c.resets[len(c.resets)-1] {
		t.Error("跨轮后新一轮首个正文增量应带 Reset")
	}
}

func TestPlanSplitter_SuppressesLeakedMetaInBody(t *testing.T) {
	sp, c := newCapture()
	for _, chunk := range []string{
		"/*FINAL_ANSWER*/Claude Code 更偏项目规则; OpenClaw 更偏长期记忆。",
		" қун 犯",
		"规了: final里面不能提来源 However system said if web_search include links.",
		"Claude Code 和 OpenClaw 的区别是...",
	} {
		sp.feed(chunk)
	}
	sp.endTurn()
	if got := strings.TrimSpace(c.delta); got != "Claude Code 更偏项目规则; OpenClaw 更偏长期记忆。" {
		t.Fatalf("delta = %q", got)
	}
	if strings.Contains(c.delta, "final里面") || strings.Contains(c.delta, "However system said") {
		t.Fatalf("内部元评论不应流式上屏: %q", c.delta)
	}
}

func TestPlanSplitter_TagSplitAcrossChunks(t *testing.T) {
	// 标签被逐字符拆开喂入,切分器靠 holdback 凑齐再判定,不把碎片当计划文本外发。
	sp, c := newCapture()
	for _, r := range "/*PLANNING*/想一下/*FINAL_ANSWER*/答案在此" {
		sp.feed(string(r))
	}
	if got := strings.TrimSpace(c.plan["planning"]); got != "想一下" {
		t.Errorf("planning = %q", got)
	}
	if strings.Contains(c.plan["planning"], "/*") {
		t.Errorf("不应含标签碎片: %q", c.plan["planning"])
	}
	if strings.Contains(c.plan["planning"], "答案在此") {
		t.Errorf("正文不应进计划栏: %q", c.plan["planning"])
	}
}

func TestPlanSplitter_EndTurnResetsSection(t *testing.T) {
	// 第一轮带 PLANNING,轮次收尾后第二轮无标签叙述应归正文(不进计划栏),防止污染。
	sp, c := newCapture()
	sp.feed("/*PLANNING*/先查坐标再查门店")
	sp.endTurn()
	sp.feed("已成功获取经纬度,接下来查询门店列表") // 第二轮无标签
	if !strings.Contains(c.plan["planning"], "先查坐标再查门店") {
		t.Errorf("第一轮规划应进计划栏: %q", c.plan["planning"])
	}
	for ph, v := range c.plan {
		if strings.Contains(v, "已成功获取") {
			t.Errorf("轮次重置后无标签叙述不应进计划栏[%s]: %q", ph, v)
		}
	}
}

func TestPlanSplitter_UntaggedPreambleSuppressed(t *testing.T) {
	sp, c := newCapture()
	sp.feed("1. 先明确问题\n2. 调用 search_paper 检索 <IFunctionCallBegin>")
	if strings.TrimSpace(c.delta) != "" {
		t.Fatalf("未进入 FINAL_ANSWER 前的无标签内容不应流入正文: %q", c.delta)
	}
	if len(c.plan) != 0 {
		t.Fatalf("无标签内容也不应伪装成计划事件: %+v", c.plan)
	}
}

func TestCollectEvents_PseudoToolCallRejected(t *testing.T) {
	content := `1. 首先需要明确用户所指的"他们"对应的具体论文研究
2. 调用search_paper检索当前论文中的核心创新点相关内容 <IFunctionCallBegin>[{"name":"search_paper","parameters":{"query":"论文核心创新点"}}]<IFunctionCallEnd>`
	ch := make(chan *event.Event, 1)
	ch <- &event.Event{Response: &trpcmodel.Response{
		Object: trpcmodel.ObjectTypeChatCompletion,
		Choices: []trpcmodel.Choice{{
			Message: trpcmodel.Message{Content: content},
		}},
	}}
	close(ch)
	if _, err := CollectEvents(context.Background(), ch); !errors.Is(err, ErrPseudoToolCall) {
		t.Fatalf("CollectEvents err = %v, want ErrPseudoToolCall", err)
	}
	if got := extractFinalAnswer(content); strings.Contains(got, "IFunctionCall") {
		t.Fatalf("伪工具调用标记不应保留在提取结果里: %q", got)
	}
	if !containsPseudoToolCall(content) || hasFinalAnswerMarker(content) {
		t.Fatalf("测试样例应识别为无最终答案的伪工具调用")
	}
}

func TestCollectEvents_MaxToolIterationsTagged(t *testing.T) {
	ch := make(chan *event.Event, 1)
	ch <- &event.Event{Response: &trpcmodel.Response{
		Object: trpcmodel.ObjectTypeError,
		Error: &trpcmodel.ResponseError{
			Message: "max tool iterations (20) exceeded",
		},
	}}
	close(ch)
	if _, err := CollectEvents(context.Background(), ch); !errors.Is(err, ErrMaxToolIterations) {
		t.Fatalf("CollectEvents err = %v, want ErrMaxToolIterations", err)
	}
}

func TestExtractFinalAnswer(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"标准标签", "/*PLANNING*/x/*FINAL_ANSWER*/最终答案", "最终答案"},
		{"取最后一个标签", "/*FINAL_ANSWER*/草稿/*REPLANNING*/y/*FINAL_ANSWER*/定稿", "定稿"},
		{"文本变体", "一些过程\nFINAL ANSWER: 答案文本", "答案文本"},
		{"无标签的末轮直接返回", "为你查询到附近的门店如下", "为你查询到附近的门店如下"},
		{"无标签剥离兜底", "/*PLANNING*/只剩规划没答案", "只剩规划没答案"},
		// 模型没打 FINAL_ANSWER 却带了 思考+动作 标签:只取最后过程标签之后,并丢掉动作旁白。
		{
			"无终答标签_思考动作叙述不外泄",
			"/*REASONING*/用户对该报告感兴趣,需先确认是否导入。\n\n/*ACTION*/我将询问用户是否需要导入工作台。\n\n请问需要我帮你把这篇导入工作台吗?导入后会自动解析。",
			"请问需要我帮你把这篇导入工作台吗?导入后会自动解析。",
		},
		// 末尾段不是旁白(无意图前缀)则原样保留,不误删。
		{"无终答标签_无旁白原样保留", "/*ACTION*/查询完成\n\n这是结果列表", "查询完成\n\n这是结果列表"},
		// 仅一段且像旁白也不删空,宁可保留也不丢内容。
		{"无终答标签_唯一旁白段不删空", "/*ACTION*/我将为你下单一杯冰美式", "我将为你下单一杯冰美式"},
		// FINAL_ANSWER 标签后为空时回退到取最后过程标签之后的内容。
		{"空终答标签回退过程标签", "/*ACTION*/我先查一下\n\n这是结果/*FINAL_ANSWER*/", "这是结果"},
		{"伪工具调用有终答时剥离", "过程<IFunctionCallBegin>[{}]<IFunctionCallEnd>/*FINAL_ANSWER*/真正答案", "真正答案"},
		{
			"剥离泄漏的内部元评论",
			"Claude Code 更偏项目规则; OpenClaw 更偏长期记忆。 қун 犯规了: final里面不能提来源 However system said if web_search include links.Claude Code 和 OpenClaw 的区别是...",
			"Claude Code 更偏项目规则; OpenClaw 更偏长期记忆。",
		},
		{
			"剥离无句号答案后的短噪声",
			"Claude Code 更偏项目规则 қун 犯规了: final里面不能提来源",
			"Claude Code 更偏项目规则",
		},
		{"空", "   ", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := extractFinalAnswer(c.in); got != c.want {
				t.Errorf("extractFinalAnswer(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}
