package pioneer

import (
	"strings"
	"testing"

	"GopherPaper/internal/ai/core"
	"GopherPaper/pkg/constant"
)

// capture 收集切分器外发的 plan 事件,按 phase 聚合文本;同时记录是否误发了正文(delta)事件。
type capture struct {
	plan    map[string]string
	gotName map[string]bool
}

func newCapture() (*planSplitter, *capture) {
	c := &capture{plan: map[string]string{}, gotName: map[string]bool{}}
	sp := newPlanSplitter(func(ev core.StreamEvent) {
		c.gotName[ev.Kind] = true
		if ev.Kind == constant.StreamEventPlan {
			c.plan[ev.Phase] += ev.Delta
		}
	})
	return sp, c
}

func TestPlanSplitter_OnlyPlanSectionsEmitted(t *testing.T) {
	// 正文段(FINAL_ANSWER 之后)不应外发任何事件,只有计划段进 plan 流。
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
	if c.gotName[constant.StreamEventDelta] {
		t.Error("正文段不应外发 delta 事件")
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
