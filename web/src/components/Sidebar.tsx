import { useApp } from "../store";
import { formatTime, sessionsForPaper, sessionTitle } from "../utils";
import { Empty, useGuard } from "./ui";

export function Sidebar() {
  const {
    user,
    sessions,
    activePaperID,
    activeSessionID,
    openSession,
    removeSession,
    refreshSessions,
    logout,
  } = useApp();
  const guard = useGuard();

  const initials = (user?.name || user?.student_id || "GP")
    .slice(0, 2)
    .toUpperCase();

  // 只展示当前论文的会话(按最近活动倒序);未选论文时展示全部。
  const visibleSessions = sessionsForPaper(sessions, activePaperID);

  return (
    <aside className="side-pane">
      <div className="brand-row">
        <div className="brand-mark">
          <span className="brand-logo" aria-hidden>
            G
          </span>
          <div>
            <p className="eyebrow">GopherPaper</p>
            <h2>知识工作台</h2>
          </div>
        </div>
        <button
          type="button"
          className="ghost-button"
          onClick={logout}
          title="退出登录"
        >
          退出
        </button>
      </div>

      <div className="user-card">
        <span className="avatar" aria-hidden>
          {initials}
        </span>
        <div className="user-meta">
          <strong>{user?.name || user?.student_id || "同学"}</strong>
          <p>
            {user?.student_id || "未记录学号"} · {user?.email || "已登录"}
          </p>
        </div>
      </div>

      {/* 小云雀是独立页:多面手 agent,查论文/点咖啡,新标签页打开。 */}
      <a className="pioneer-entry" href="/pioneer" target="_blank" rel="noreferrer">
        ⚡ 小云雀
        <span className="pioneer-entry-sub">查论文 · 点咖啡</span>
      </a>

      <div className="side-section">
        <div className="section-heading">
          <h3>历史会话</h3>
          <button
            type="button"
            className="icon-button"
            title="刷新会话"
            onClick={() =>
              guard(async () => {
                await refreshSessions();
              })
            }
          >
            ↻
          </button>
        </div>

        <div className="session-list">
          {visibleSessions.length === 0 ? (
            <Empty
              title="暂无会话"
              text={
                activePaperID
                  ? "这篇论文还没有会话,在右侧提问会自动创建。"
                  : "创建会话后会出现在这里。"
              }
              inline
            />
          ) : (
            visibleSessions.map((s) => (
              <div
                key={s.id}
                className={`session-item${
                  s.id === activeSessionID ? " active" : ""
                }`}
              >
                <button
                  type="button"
                  className="session-main"
                  onClick={() => guard(() => openSession(s.id))}
                >
                  <span className="session-title">{sessionTitle(s)}</span>
                  <span className="session-time">
                    {s.paper_id ? "论文会话" : "普通会话"} ·{" "}
                    {formatTime(s.updated_at || s.created_at)}
                  </span>
                </button>
                <button
                  type="button"
                  className="delete-session"
                  title="删除会话"
                  onClick={() => {
                    if (window.confirm("确定删除这个会话吗?")) {
                      guard(() => removeSession(s.id));
                    }
                  }}
                >
                  ×
                </button>
              </div>
            ))
          )}
        </div>
      </div>
    </aside>
  );
}
