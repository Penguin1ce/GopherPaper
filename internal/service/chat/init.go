// Package chat 是会话与多轮对话的业务逻辑，包级函数直接读写 dao/chat、history 与 ai。
// 会话归属校验先核对 session.StudentID 与当前学生是否一致。
// SendMessage 读取历史、调用 AI、追加 Session 事件并返回。
// 历史与上下文均由 internal/history 承载。
package chat
