// Package history 是会话历史的存储与读取，由 trpc-agent-go 的 MySQL Session 承载。
//
// 迁移 Step 4：取代原先「Redis 上下文缓存 + chat_queue 入库 worker + MySQL messages 表」
// 三件套。会话的每轮对话作为 Session 事件落 MySQL，既做多轮上下文窗口，也做历史展示。
//
// 依 CLAUDE.md「包级函数 + Init，不做 DI」：Service 藏在包级变量 svc 之后，
// 其他包直接调 history.Load / Append / List / Delete，不持有句柄。
//
// 持久化用同步写：MySQL 是历史唯一存储（system-of-record，与业务库同实例），
// AppendEvent 的几次写相对秒级模型调用可忽略，同步换取「应答即落库、下一轮立即可见」。
// 若要削延迟可在 NewService 加 WithEnableAsyncPersist（进程内 worker，非 MQ）。
// 表由框架自动创建（session_*），软删除默认开启；TTL 设为 0（不过期）。
package history

import (
	"context"
	"fmt"

	trpcevent "trpc.group/trpc-go/trpc-agent-go/event"
	trpcmodel "trpc.group/trpc-go/trpc-agent-go/model"
	trpcsession "trpc.group/trpc-go/trpc-agent-go/session"
	mysqlsession "trpc.group/trpc-go/trpc-agent-go/session/mysql"

	"GopherPaper/internal/config"
	"GopherPaper/internal/model"
	"GopherPaper/pkg/constant"
)

// svc 是包级 MySQL Session 服务，由 Init 初始化后供本包函数引用。
var svc trpcsession.Service

// invocationID 标记本服务写入的事件来源，便于排查。
const invocationID = "gopherpaper-chat"

// Init 用项目的 MySQL DSN 建 trpc MySQL Session 服务（自动建表）。须在 dao.InitMySQL 之后调用。
func Init(cfg config.MySQLConfig) error {
	s, err := mysqlsession.NewService(
		mysqlsession.WithMySQLClientDSN(cfg.DSN),
		mysqlsession.WithSessionTTL(0), // 历史唯一存储，不过期
	)
	if err != nil {
		return fmt.Errorf("history: 初始化 MySQL Session 失败: %w", err)
	}
	svc = s
	return nil
}

// Load 取会话最近 limit 条历史，按时间升序，用于拼多轮上下文。会话不存在返回空。
func Load(ctx context.Context, userID, sessionID string, limit int) ([]model.Message, error) {
	sess, err := svc.GetSession(ctx, keyOf(userID, sessionID), trpcsession.WithEventNum(limit))
	if err != nil {
		return nil, fmt.Errorf("history: 读取会话历史失败: %w", err)
	}
	if sess == nil {
		return nil, nil
	}
	return toMessages(sessionID, sess.GetEvents()), nil
}

// List 取会话全部历史，按时间升序，供前端展示。会话不存在返回空。
func List(ctx context.Context, userID, sessionID string) ([]model.Message, error) {
	sess, err := svc.GetSession(ctx, keyOf(userID, sessionID))
	if err != nil {
		return nil, fmt.Errorf("history: 读取会话历史失败: %w", err)
	}
	if sess == nil {
		return nil, nil
	}
	return toMessages(sessionID, sess.GetEvents()), nil
}

// Append 把一轮消息追加进会话，会话不存在则先建。开启异步持久化后写入快速返回。
func Append(ctx context.Context, userID, sessionID string, msgs ...*model.Message) error {
	if len(msgs) == 0 {
		return nil
	}
	key := keyOf(userID, sessionID)
	sess, err := svc.GetSession(ctx, key, trpcsession.WithEventNum(1))
	if err != nil {
		return fmt.Errorf("history: 定位会话失败: %w", err)
	}
	if sess == nil {
		sess, err = svc.CreateSession(ctx, key, nil)
		if err != nil {
			return fmt.Errorf("history: 创建会话失败: %w", err)
		}
	}
	for _, m := range msgs {
		if err := svc.AppendEvent(ctx, sess, toEvent(m)); err != nil {
			return fmt.Errorf("history: 追加会话事件失败: %w", err)
		}
	}
	return nil
}

// Delete 删除会话及其全部历史事件。
func Delete(ctx context.Context, userID, sessionID string) error {
	if err := svc.DeleteSession(ctx, keyOf(userID, sessionID)); err != nil {
		return fmt.Errorf("history: 删除会话失败: %w", err)
	}
	return nil
}

// keyOf 拼 trpc Session 三元组 appName/userID/sessionID。
func keyOf(userID, sessionID string) trpcsession.Key {
	return trpcsession.Key{AppName: constant.SessionAppName, UserID: userID, SessionID: sessionID}
}

// toEvent 把一条消息编为 Session 事件：内容走 Response.Choices，意图借 Tag 透传。
func toEvent(m *model.Message) *trpcevent.Event {
	e := trpcevent.New(invocationID, m.Role)
	e.Response = &trpcmodel.Response{
		Choices: []trpcmodel.Choice{{
			Message: trpcmodel.Message{Role: trpcmodel.Role(m.Role), Content: m.Content},
		}},
	}
	if m.Intent != "" {
		e.Tag = string(m.Intent)
	}
	return e
}

// toMessages 把 Session 事件还原为消息，跳过无文本内容的非对话事件。
func toMessages(sessionID string, events []trpcevent.Event) []model.Message {
	msgs := make([]model.Message, 0, len(events))
	for i := range events {
		e := events[i]
		if e.Response == nil || len(e.Response.Choices) == 0 {
			continue
		}
		msg := e.Response.Choices[0].Message
		if msg.Content == "" {
			continue
		}
		msgs = append(msgs, model.Message{
			ID:        e.ID,
			SessionID: sessionID,
			Role:      string(msg.Role),
			Content:   msg.Content,
			Intent:    constant.IntentType(e.Tag),
			CreatedAt: e.Timestamp,
		})
	}
	return msgs
}
