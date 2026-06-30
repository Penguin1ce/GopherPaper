// Package topic 是小云雀会话的主题自动归类引擎，无状态(向量与模型都从既有包取)。
//
// 选型沿用业界主流的「embedding 最近质心分配 + 阈值新建 + 小模型命名 + 质心过近合并」:
// 不让 LLM 硬分类单句(首条消息分类不准),而是取会话累计文本(问+答)向量化,
// 与已有主题质心比余弦相似度——超阈值并入最相似主题,否则新建主题并请小模型起名;
// 两主题质心过近时合并,天然把相近会话聚到一起。
//
// 质心存成员(归一化)向量之和而非均值:加入(+vec)、扣减(-vec)、合并(两和相加)都是
// 精确可逆的加减,余弦比较与向量长度无关故无需显式归一化。借此支持前几轮的重排(refine):
// 内容变厚后会话可搬到更贴切主题,从旧主题精确扣减贡献、不丢精度、不重复计数。
package topic

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"sync"

	trpcmodel "trpc.group/trpc-go/trpc-agent-go/model"

	"GopherPaper/internal/ai/core"
	"GopherPaper/internal/aimodel"
	chatdao "GopherPaper/internal/dao/chat"
	topicdao "GopherPaper/internal/dao/topic"
	"GopherPaper/internal/history"
	"GopherPaper/internal/knowledge"
	"GopherPaper/internal/model"
	"GopherPaper/internal/zlog"
	"GopherPaper/pkg/constant"
)

// nameSystemPrompt 指导小模型给一段对话起短主题名。
const nameSystemPrompt = "你是会话主题命名助手。根据下面的对话内容,起一个不超过8个汉字的主题名,概括这段对话在聊什么。只输出主题名本身,不要标点、引号或多余文字。"

// classifying 让同一会话的归类串行执行,防 refine 期相邻轮的 goroutine 并发扣减/加入致质心错乱。
var classifying sync.Map // sessionID -> struct{}

// Classify 把会话归入最相似主题或新建主题,落 Session.TopicID 与归类向量。
// 首轮拿到问+答即归类(比裸首句鲁棒);前 TopicRefineMaxTurns 轮内容变厚后允许重排——
// 可留在原主题(仅刷新该会话向量贡献)、搬到更贴切主题或新建,超过轮数则锁定不再动避免 UI 横跳。
// 由 SendMessage 在每轮后异步触发。
func Classify(ctx context.Context, studentID, sessionID string) error {
	if _, busy := classifying.LoadOrStore(sessionID, struct{}{}); busy {
		return nil // 同一会话已有归类在跑,跳过;下一轮会再触发
	}
	defer classifying.Delete(sessionID)

	sess, err := chatdao.GetSession(ctx, sessionID)
	if err != nil {
		return err
	}
	msgs, err := history.List(ctx, studentID, sessionID)
	if err != nil {
		return err
	}
	text := conversationTextFrom(msgs)
	if text == "" {
		return nil
	}
	turns := assistantTurns(msgs)
	// 已归类且超过重排轮数:主题已稳定,锁定不再动。
	if sess.TopicID != "" && turns > constant.TopicRefineMaxTurns {
		return nil
	}

	vec, err := knowledge.Embed(ctx, text)
	if err != nil {
		return fmt.Errorf("ai/topic: 会话向量化失败: %w", err)
	}
	normalize(vec)

	topics, err := topicdao.ListTopics(ctx, studentID, constant.AgentPioneer)
	if err != nil {
		return err
	}
	best, bestSim := nearest(topics, vec)
	targetID := ""
	if best != nil && bestSim >= constant.TopicAssignThreshold {
		targetID = best.ID
	}

	oldTopicID := sess.TopicID
	oldVec := decode(sess.TopicVec)

	// 仍属同一主题:只把该会话的旧向量贡献替换成新向量(成员数不变)。
	if targetID != "" && targetID == oldTopicID {
		if t := findTopic(topics, oldTopicID); t != nil {
			sum := addVec(subVec(decode(t.Centroid), oldVec), vec)
			if err := topicdao.UpdateTopicCentroid(ctx, t.ID, encode(sum), t.MemberCount); err != nil {
				return err
			}
		}
		if err := topicdao.SetSessionTopicVec(ctx, sessionID, oldTopicID, encode(vec)); err != nil {
			return err
		}
		zlog.Info("会话主题维持不变(刷新向量)",
			"student_id", studentID, "session_id", sessionID,
			"topic_id", oldTopicID, "sim", bestSim, "turns", turns)
		mergeClose(ctx, studentID, oldTopicID)
		return nil
	}

	// 换主题或首次归类:先从旧主题扣除该会话贡献,旧主题空了就删。
	if oldTopicID != "" && len(oldVec) > 0 {
		if t := findTopic(topics, oldTopicID); t != nil {
			if t.MemberCount <= 1 {
				if err := topicdao.DeleteTopic(ctx, t.ID); err != nil {
					return err
				}
			} else if err := topicdao.UpdateTopicCentroid(ctx, t.ID, encode(subVec(decode(t.Centroid), oldVec)), t.MemberCount-1); err != nil {
				return err
			}
		}
	}

	var assignedID string
	if targetID != "" {
		t := findTopic(topics, targetID)
		if err := topicdao.UpdateTopicCentroid(ctx, targetID, encode(addVec(decode(t.Centroid), vec)), t.MemberCount+1); err != nil {
			return err
		}
		assignedID = targetID
		action := "会话归入既有主题"
		if oldTopicID != "" {
			action = "会话重排到更贴切主题"
		}
		zlog.Info(action,
			"student_id", studentID, "session_id", sessionID,
			"topic_id", t.ID, "topic_name", t.Name, "from_topic", oldTopicID,
			"sim", bestSim, "member_count", t.MemberCount+1)
	} else {
		// 新建主题,小模型起名,失败回退用文本片段。
		name := nameTopic(ctx, studentID, text)
		t := &model.Topic{
			StudentID:   studentID,
			AgentType:   constant.AgentPioneer,
			Name:        name,
			Centroid:    encode(vec),
			MemberCount: 1,
		}
		if err := topicdao.CreateTopic(ctx, t); err != nil {
			return err
		}
		assignedID = t.ID
		zlog.Info("新建会话主题",
			"student_id", studentID, "session_id", sessionID,
			"topic_id", t.ID, "topic_name", name, "from_topic", oldTopicID,
			"best_sim", bestSim, "existing_topics", len(topics))
	}
	if err := topicdao.SetSessionTopicVec(ctx, sessionID, assignedID, encode(vec)); err != nil {
		return err
	}

	// 合并:新主题/更新后扫一遍,把与刚归类主题过近的另一主题并掉,防相近主题碎裂。
	mergeClose(ctx, studentID, assignedID)
	return nil
}

// conversationTextFrom 把会话消息拼成带角色前缀的累计文本,按字符上限截断作 embedding 输入。
func conversationTextFrom(msgs []model.Message) string {
	var sb strings.Builder
	for _, m := range msgs {
		content := strings.TrimSpace(m.Content)
		if content == "" {
			continue
		}
		prefix := "用户"
		if m.Role == model.RoleAssistant {
			prefix = "助手"
		}
		sb.WriteString(prefix)
		sb.WriteString(": ")
		sb.WriteString(content)
		sb.WriteString("\n")
	}
	return truncateRunes(sb.String(), constant.TopicEmbedMaxRunes)
}

// assistantTurns 数助手回复条数,近似对话轮数,用于判断是否还在可重排窗口内。
func assistantTurns(msgs []model.Message) int {
	n := 0
	for _, m := range msgs {
		if m.Role == model.RoleAssistant {
			n++
		}
	}
	return n
}

// findTopic 在快照里按 id 找主题,无则 nil。
func findTopic(topics []model.Topic, id string) *model.Topic {
	for i := range topics {
		if topics[i].ID == id {
			return &topics[i]
		}
	}
	return nil
}

// nameTopic 用该用户 intent 小模型给对话起主题名,失败或超长时回退文本片段。
func nameTopic(ctx context.Context, studentID, text string) string {
	fallback := truncateRunes(strings.TrimSpace(strings.SplitN(text, "\n", 2)[0]), constant.TopicNameMaxRunes)
	fallback = strings.TrimPrefix(fallback, "用户: ")
	if fallback == "" {
		fallback = "新主题"
	}

	models, err := aimodel.ModelsForUser(studentID)
	if err != nil {
		return fallback
	}
	req := &trpcmodel.Request{
		Messages: []trpcmodel.Message{
			trpcmodel.NewSystemMessage(nameSystemPrompt),
			trpcmodel.NewUserMessage(text),
		},
	}
	if models.IntentMC.MaxTokens > 0 {
		req.MaxTokens = &models.IntentMC.MaxTokens
	}
	out, err := core.GenerateText(ctx, models.Intent, req)
	if err != nil {
		zlog.Warn("主题命名失败,回退文本片段", "err", err)
		return fallback
	}
	name := truncateRunes(cleanName(out), constant.TopicNameMaxRunes)
	if name == "" {
		return fallback
	}
	return name
}

// cleanName 清掉小模型可能多带的引号、空白与换行。
func cleanName(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, "\n\r"); i >= 0 {
		s = s[:i]
	}
	return strings.Trim(s, " \t\"'「」『』:：")
}

// mergeClose 把与 anchorID 主题质心相似度超合并阈值的另一主题并入,成员多者为目标。
// 质心是成员向量之和,合并即两和相加、成员数相加,精确无损。
// 每次只并最近的一对,避免一次连环合并;后续轮次会继续收敛。
func mergeClose(ctx context.Context, studentID, anchorID string) {
	topics, err := topicdao.ListTopics(ctx, studentID, constant.AgentPioneer)
	if err != nil {
		return
	}
	var anchor *model.Topic
	for i := range topics {
		if topics[i].ID == anchorID {
			anchor = &topics[i]
			break
		}
	}
	if anchor == nil {
		return
	}
	anchorVec := decode(anchor.Centroid)

	var other *model.Topic
	bestSim := constant.TopicMergeThreshold
	for i := range topics {
		if topics[i].ID == anchorID {
			continue
		}
		sim := cosine(anchorVec, decode(topics[i].Centroid))
		if sim >= bestSim {
			bestSim = sim
			other = &topics[i]
		}
	}
	if other == nil {
		return
	}

	dst, src := anchor, other
	if src.MemberCount > dst.MemberCount {
		dst, src = other, anchor
	}
	total := dst.MemberCount + src.MemberCount
	merged := addVec(decode(dst.Centroid), decode(src.Centroid))
	if err := topicdao.MergeTopics(ctx, src.ID, dst.ID, encode(merged), total); err != nil {
		zlog.Warn("主题合并失败", "src", src.ID, "dst", dst.ID, "err", err)
		return
	}
	zlog.Info("合并相近主题",
		"student_id", studentID,
		"src_id", src.ID, "src_name", src.Name,
		"dst_id", dst.ID, "dst_name", dst.Name,
		"sim", bestSim, "member_count", total)
}

// nearest 返回与 vec 余弦相似度最大的主题及其相似度,无主题返回 nil,-1。
func nearest(topics []model.Topic, vec []float64) (*model.Topic, float64) {
	var best *model.Topic
	bestSim := -1.0
	for i := range topics {
		c := decode(topics[i].Centroid)
		if len(c) == 0 {
			continue
		}
		sim := cosine(vec, c)
		if sim > bestSim {
			bestSim = sim
			best = &topics[i]
		}
	}
	return best, bestSim
}

// addVec 返回 a+b(质心是成员向量之和,加入成员即加上其向量)。一方为空回退另一方,长度不一返回 a 的副本。
func addVec(a, b []float64) []float64 {
	if len(a) == 0 {
		return append([]float64(nil), b...)
	}
	if len(b) == 0 || len(a) != len(b) {
		return append([]float64(nil), a...)
	}
	out := make([]float64, len(a))
	for i := range a {
		out[i] = a[i] + b[i]
	}
	return out
}

// subVec 返回 a-b(从质心和里扣减某会话向量)。b 为空或长度不一返回 a 的副本。
func subVec(a, b []float64) []float64 {
	if len(b) == 0 || len(a) != len(b) {
		return append([]float64(nil), a...)
	}
	out := make([]float64, len(a))
	for i := range a {
		out[i] = a[i] - b[i]
	}
	return out
}

// cosine 算两向量余弦相似度,长度不一或零向量返回 0。
func cosine(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += a[i] * b[i]
		na += a[i] * a[i]
		nb += b[i] * b[i]
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

// normalize 原地把向量归一化为单位长度,零向量不变。
func normalize(v []float64) {
	var n float64
	for _, x := range v {
		n += x * x
	}
	if n == 0 {
		return
	}
	n = math.Sqrt(n)
	for i := range v {
		v[i] /= n
	}
}

// encode/decode 把质心向量在 MySQL 里以 JSON([]float64)落字符串。
func encode(v []float64) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "[]"
	}
	return string(b)
}

func decode(s string) []float64 {
	if s == "" {
		return nil
	}
	var v []float64
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return nil
	}
	return v
}

// truncateRunes 按字符截断,超长直接砍掉尾部(主题名/向量化输入都不需要省略号)。
func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}
