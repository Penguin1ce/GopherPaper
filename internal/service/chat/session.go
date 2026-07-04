package chat

import (
	"context"
	"strings"
	"sync"

	"GopherPaper/internal/ai"
	"GopherPaper/internal/ai/topic"
	chatdao "GopherPaper/internal/dao/chat"
	topicdao "GopherPaper/internal/dao/topic"
	"GopherPaper/internal/history"
	"GopherPaper/internal/model"
	"GopherPaper/internal/zlog"
	"GopherPaper/pkg/constant"
	"GopherPaper/pkg/errs"
)

// backfilling 记正在回填的用户,防同一用户并发触发重复跑同一批会话。
var backfilling sync.Map // studentID -> struct{}

// CreateSession 为用户新建一段会话，paperID 可空表示跨库问答。
// agentType 空为默认论文助教，pioneer 为小云雀会话，maodie 为精读页小耄耋。
func CreateSession(ctx context.Context, studentID, paperID, title, agentType string) (*model.Session, error) {
	paperID = strings.TrimSpace(paperID)
	agentType = strings.TrimSpace(agentType)
	if agentType != "" && agentType != constant.AgentPioneer && agentType != constant.AgentMaodie {
		return nil, errs.ErrAgentTypeInvalid
	}
	if agentType == constant.AgentMaodie && paperID == "" {
		return nil, errs.ErrPaperRequired
	}
	s := &model.Session{StudentID: studentID, PaperID: paperID, AgentType: agentType, Title: truncateTitle(title)}
	if err := chatdao.CreateSession(ctx, s); err != nil {
		return nil, err
	}
	return s, nil
}

// truncateTitle 按字符截断会话标题到列上限，超长补省略号，防 title 列溢出报 Error 1406。
func truncateTitle(title string) string {
	r := []rune(title)
	if len(r) <= constant.SessionTitleMaxRunes {
		return title
	}
	return string(r[:constant.SessionTitleMaxRunes-1]) + "…"
}

// ListSessions 列出学生的全部会话。
func ListSessions(ctx context.Context, studentID string) ([]model.Session, error) {
	return chatdao.ListSessions(ctx, studentID)
}

// ListPioneerTopics 列出学生的小云雀会话主题,供前端按主题分组渲染。
func ListPioneerTopics(ctx context.Context, studentID string) ([]model.Topic, error) {
	return topicdao.ListTopics(ctx, studentID, constant.AgentPioneer)
}

// ClearPioneerTopics 清空当前用户的全部小云雀主题,会话退回未归类,返回删除的主题数。
// 供演示重置归类:清空后点整理可重新归类。
func ClearPioneerTopics(ctx context.Context, studentID string) (int, error) {
	n, err := topicdao.ClearTopics(ctx, studentID, constant.AgentPioneer)
	if err != nil {
		return 0, err
	}
	zlog.Info("清空会话主题", "student_id", studentID, "removed", n)
	return int(n), nil
}

// BackfillPioneerTopics 异步把当前用户尚未归类的小云雀会话逐个归类,返回待处理会话数。
// 存量(上线前已有)会话不会被发消息链路触达,经本接口手动回填。
// 串行跑避免并发建重复主题;Classify 幂等,重复触发或服务重启再跑都只补未归类的。
// 同一用户已在回填时返回 ErrTopicBackfillBusy,防并发重入。
func BackfillPioneerTopics(ctx context.Context, studentID string) (int, error) {
	sessions, err := topicdao.ListSessionsToClassify(ctx, studentID, constant.AgentPioneer)
	if err != nil {
		return 0, err
	}
	if len(sessions) == 0 {
		return 0, nil
	}
	if _, busy := backfilling.LoadOrStore(studentID, struct{}{}); busy {
		return 0, errs.ErrTopicBackfillBusy
	}

	ids := make([]string, len(sessions))
	for i, s := range sessions {
		ids[i] = s.ID
	}
	zlog.Info("开始回填会话主题", "student_id", studentID, "count", len(ids))
	bg := context.WithoutCancel(ctx) // 脱离请求生命周期,保住身份不被取消
	go func() {
		defer backfilling.Delete(studentID)
		for _, id := range ids {
			if err := topic.Classify(bg, studentID, id); err != nil {
				zlog.Warn("回填会话主题失败", "session_id", id, "err", err)
			}
		}
		zlog.Info("会话主题回填完成", "student_id", studentID, "count", len(ids))
	}()
	return len(ids), nil
}

// DeleteSession 删除会话，仅限本人。会话元数据软删，历史事件从 Session 清理。
func DeleteSession(ctx context.Context, studentID, sessionID string) error {
	sess, err := ownedSession(ctx, studentID, sessionID)
	if err != nil {
		return err
	}
	// 先清历史事件再软删元数据:历史删除失败即整体失败,会话仍在可重试,
	// 避免出现「元数据已删但 Session 事件永久残留」的孤儿。Delete 对空会话幂等,重试可自愈。
	if err := history.Delete(ctx, studentID, sessionID); err != nil {
		return err
	}
	if sess.AgentType == constant.AgentPioneer {
		if err := ai.DeletePioneerSessionMemory(ctx, studentID, sessionID); err != nil {
			return err
		}
	}
	if err := chatdao.DeleteSession(ctx, sessionID); err != nil {
		return err
	}
	return nil
}

// DeleteSessionsForPaper 删除某学生绑定到某篇论文的全部会话及历史。
func DeleteSessionsForPaper(ctx context.Context, studentID, paperID string) error {
	sessions, err := chatdao.ListSessionsByPaper(ctx, studentID, paperID)
	if err != nil {
		zlog.Error("查询论文绑定会话失败", "student_id", studentID, "paper_id", paperID, "err", err)
		return err
	}
	zlog.Info("开始删除论文绑定会话", "student_id", studentID, "paper_id", paperID, "count", len(sessions))
	for _, s := range sessions {
		zlog.Info("删除论文绑定会话", "student_id", studentID, "paper_id", paperID, "session_id", s.ID, "title", s.Title)
		if err := DeleteSession(ctx, studentID, s.ID); err != nil {
			zlog.Error("删除论文绑定会话失败", "student_id", studentID, "paper_id", paperID, "session_id", s.ID, "err", err)
			return err
		}
	}
	zlog.Info("论文绑定会话删除完成", "student_id", studentID, "paper_id", paperID, "count", len(sessions))
	return nil
}

// ListMessages 还原会话历史消息，仅限本人，从 trpc Session 读。
func ListMessages(ctx context.Context, studentID, sessionID string) ([]model.Message, error) {
	if _, err := ownedSession(ctx, studentID, sessionID); err != nil {
		return nil, err
	}
	return history.List(ctx, studentID, sessionID)
}

// ownedSession 取会话并校验归属，非本人返回 errs.ErrSessionForbidden。
func ownedSession(ctx context.Context, studentID, sessionID string) (*model.Session, error) {
	s, err := chatdao.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if s.StudentID != studentID {
		return nil, errs.ErrSessionForbidden
	}
	return s, nil
}
