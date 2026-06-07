// Package chat 是会话与多轮对话的业务逻辑，包级函数直接读写 dao/chat、history 与 ai。
//
//	会话归属校验：任何按 id 的操作先核对 session.StudentID 与当前学生是否一致
//	多轮对话 SendMessage：读 Session 历史 → 调 AI → 追加 Session 事件 → 直接返回
//	历史与上下文均由 trpc Session（Redis）承载，见 internal/history
package chat
