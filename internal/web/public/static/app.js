const API_BASE = "/api/v1";
const AUTH_KEY = "gopherpaper.auth";

const state = {
  token: "",
  user: null,
  sessions: [],
  activeSessionID: "",
  messages: [],
  busy: false,
};

const el = {
  authView: document.querySelector("#authView"),
  chatView: document.querySelector("#chatView"),
  loginTab: document.querySelector("#loginTab"),
  registerTab: document.querySelector("#registerTab"),
  loginForm: document.querySelector("#loginForm"),
  registerForm: document.querySelector("#registerForm"),
  loginStudentID: document.querySelector("#loginStudentID"),
  loginPassword: document.querySelector("#loginPassword"),
  registerStudentID: document.querySelector("#registerStudentID"),
  registerName: document.querySelector("#registerName"),
  registerEmail: document.querySelector("#registerEmail"),
  registerClassID: document.querySelector("#registerClassID"),
  registerPassword: document.querySelector("#registerPassword"),
  registerCode: document.querySelector("#registerCode"),
  sendCodeButton: document.querySelector("#sendCodeButton"),
  refreshSessionsButton: document.querySelector("#refreshSessionsButton"),
  newSessionForm: document.querySelector("#newSessionForm"),
  newSessionTitle: document.querySelector("#newSessionTitle"),
  sessionList: document.querySelector("#sessionList"),
  userLine: document.querySelector("#userLine"),
  activeTitle: document.querySelector("#activeTitle"),
  logoutButton: document.querySelector("#logoutButton"),
  emptyState: document.querySelector("#emptyState"),
  messageList: document.querySelector("#messageList"),
  messageForm: document.querySelector("#messageForm"),
  messageInput: document.querySelector("#messageInput"),
  sendMessageButton: document.querySelector("#sendMessageButton"),
  toast: document.querySelector("#toast"),
};

function loadAuth() {
  try {
    const raw = localStorage.getItem(AUTH_KEY);
    if (!raw) {
      return;
    }
    const saved = JSON.parse(raw);
    state.token = saved.token || "";
    state.user = saved.user || null;
  } catch {
    localStorage.removeItem(AUTH_KEY);
  }
}

function saveAuth(user, token) {
  state.user = user;
  state.token = token;
  localStorage.setItem(AUTH_KEY, JSON.stringify({ user, token }));
}

function clearAuth() {
  state.user = null;
  state.token = "";
  state.sessions = [];
  state.activeSessionID = "";
  state.messages = [];
  localStorage.removeItem(AUTH_KEY);
}

async function api(path, options = {}) {
  const headers = {
    "Content-Type": "application/json",
    ...(options.headers || {}),
  };
  if (state.token) {
    headers.Authorization = `Bearer ${state.token}`;
  }
  const res = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers,
  });
  const text = await res.text();
  const body = text ? JSON.parse(text) : {};
  if (!res.ok || body.code !== 0) {
    const message = body.message || `请求失败 ${res.status}`;
    if (res.status === 401) {
      clearAuth();
      renderShell();
    }
    throw new Error(message);
  }
  return body.data;
}

function showToast(message, type = "ok") {
  el.toast.textContent = message;
  el.toast.classList.toggle("error", type === "error");
  el.toast.classList.remove("hidden");
  window.clearTimeout(showToast.timer);
  showToast.timer = window.setTimeout(() => {
    el.toast.classList.add("hidden");
  }, 2800);
}

function setBusy(busy) {
  state.busy = busy;
  el.sendMessageButton.disabled = busy;
  el.messageInput.disabled = busy;
}

function setAuthTab(tab) {
  const isLogin = tab === "login";
  el.loginTab.classList.toggle("active", isLogin);
  el.registerTab.classList.toggle("active", !isLogin);
  el.loginForm.classList.toggle("hidden", !isLogin);
  el.registerForm.classList.toggle("hidden", isLogin);
}

function renderShell() {
  const authed = Boolean(state.token);
  el.authView.classList.toggle("hidden", authed);
  el.chatView.classList.toggle("hidden", !authed);
  if (authed) {
    const user = state.user || {};
    el.userLine.textContent = `${user.name || user.student_id || "同学"} · ${user.email || "已登录"}`;
  }
}

function sessionTitle(session) {
  return session.title || "未命名会话";
}

function formatTime(value) {
  if (!value) {
    return "";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "";
  }
  return date.toLocaleString("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  });
}

function renderSessions() {
  el.sessionList.innerHTML = "";
  if (state.sessions.length === 0) {
    const empty = document.createElement("div");
    empty.className = "empty-state";
    empty.innerHTML = "<p>暂无历史会话</p>";
    el.sessionList.append(empty);
    return;
  }
  state.sessions.forEach((session) => {
    const item = document.createElement("div");
    item.className = "session-item";
    item.classList.toggle("active", session.id === state.activeSessionID);

    const main = document.createElement("button");
    main.className = "session-main";
    main.type = "button";
    main.addEventListener("click", () => openSession(session.id));

    const title = document.createElement("span");
    title.className = "session-title";
    title.textContent = sessionTitle(session);

    const time = document.createElement("span");
    time.className = "session-time";
    time.textContent = formatTime(session.updated_at || session.created_at);

    const remove = document.createElement("button");
    remove.className = "delete-session";
    remove.type = "button";
    remove.title = "删除会话";
    remove.setAttribute("aria-label", `删除 ${sessionTitle(session)}`);
    remove.textContent = "×";
    remove.addEventListener("click", () => deleteSession(session.id));

    main.append(title, time);
    item.append(main, remove);
    el.sessionList.append(item);
  });
}

function renderMessages() {
  const active = state.sessions.find((session) => session.id === state.activeSessionID);
  el.activeTitle.textContent = active ? sessionTitle(active) : "选择或新建一个会话";
  el.emptyState.classList.toggle("hidden", Boolean(active));
  el.messageList.classList.toggle("hidden", !active);
  el.messageList.innerHTML = "";
  if (!active) {
    return;
  }
  if (state.messages.length === 0) {
    const empty = document.createElement("div");
    empty.className = "empty-state";
    empty.innerHTML = "<p>这个会话还没有消息</p>";
    el.messageList.append(empty);
    return;
  }
  state.messages.forEach((message) => {
    el.messageList.append(messageNode(message));
  });
  el.messageList.scrollTop = el.messageList.scrollHeight;
}

function messageNode(message) {
  const wrap = document.createElement("article");
  const role = message.role === "assistant" ? "assistant" : "user";
  wrap.className = `message ${role}`;

  const bubble = document.createElement("div");
  bubble.className = "message-bubble";

  const text = document.createElement("p");
  text.className = "message-text";
  text.textContent = message.content || "";

  const meta = document.createElement("div");
  meta.className = "message-meta";
  const roleLabel = role === "assistant" ? "助教" : "我";
  const intent = message.intent ? ` · ${message.intent}` : "";
  meta.textContent = `${roleLabel}${intent}${message.created_at ? ` · ${formatTime(message.created_at)}` : ""}`;

  bubble.append(text);
  wrap.append(bubble, meta);

  if (message.meta && Object.keys(message.meta).length > 0) {
    const metaBox = document.createElement("pre");
    metaBox.className = "meta-box";
    metaBox.textContent = JSON.stringify(message.meta, null, 2);
    wrap.append(metaBox);
  }
  return wrap;
}

async function loadSessions(selectFirst = false) {
  const sessions = await api("/sessions");
  state.sessions = Array.isArray(sessions) ? sessions : [];
  renderSessions();
  if (selectFirst && !state.activeSessionID && state.sessions.length > 0) {
    await openSession(state.sessions[0].id);
  } else {
    renderMessages();
  }
}

async function openSession(id) {
  state.activeSessionID = id;
  renderSessions();
  const messages = await api(`/sessions/${encodeURIComponent(id)}/messages`);
  state.messages = Array.isArray(messages) ? messages : [];
  renderMessages();
}

async function createSession(title) {
  const session = await api("/sessions", {
    method: "POST",
    body: JSON.stringify({ title }),
  });
  state.sessions = [session, ...state.sessions.filter((item) => item.id !== session.id)];
  state.activeSessionID = session.id;
  state.messages = [];
  renderSessions();
  renderMessages();
  return session;
}

async function deleteSession(id) {
  if (!window.confirm("确定删除这个会话吗？")) {
    return;
  }
  await api(`/sessions/${encodeURIComponent(id)}`, { method: "DELETE" });
  state.sessions = state.sessions.filter((session) => session.id !== id);
  if (state.activeSessionID === id) {
    state.activeSessionID = "";
    state.messages = [];
  }
  renderSessions();
  renderMessages();
  showToast("会话已删除");
}

async function sendMessage(query) {
  let sessionID = state.activeSessionID;
  if (!sessionID) {
    const title = query.slice(0, 24) || "新会话";
    const session = await createSession(title);
    sessionID = session.id;
  }

  const userMessage = {
    id: `local-${Date.now()}`,
    session_id: sessionID,
    role: "user",
    content: query,
    created_at: new Date().toISOString(),
  };
  state.messages.push(userMessage);
  renderMessages();

  const data = await api(`/sessions/${encodeURIComponent(sessionID)}/messages`, {
    method: "POST",
    body: JSON.stringify({ query }),
  });
  if (data && data.message) {
    const assistant = { ...data.message };
    if (data.meta) {
      assistant.meta = data.meta;
    }
    state.messages.push(assistant);
    renderMessages();
  }
  await loadSessions(false);
}

async function login(studentID, password) {
  const data = await api("/user/login", {
    method: "POST",
    body: JSON.stringify({ student_id: studentID, password }),
  });
  saveAuth(
    {
      student_id: data.student_id,
      name: data.name,
      email: data.email,
    },
    data.token,
  );
  renderShell();
  await loadSessions(true);
  showToast("登录成功");
}

async function registerAndLogin(payload) {
  await api("/user/register", {
    method: "POST",
    body: JSON.stringify(payload),
  });
  await login(payload.student_id, payload.password);
}

async function sendCode(email) {
  await api("/user/send-code", {
    method: "POST",
    body: JSON.stringify({ email }),
  });
  showToast("验证码已发送");
}

function bindEvents() {
  el.loginTab.addEventListener("click", () => setAuthTab("login"));
  el.registerTab.addEventListener("click", () => setAuthTab("register"));

  el.loginForm.addEventListener("submit", async (event) => {
    event.preventDefault();
    try {
      await login(el.loginStudentID.value.trim(), el.loginPassword.value);
    } catch (err) {
      showToast(err.message, "error");
    }
  });

  el.sendCodeButton.addEventListener("click", async () => {
    const email = el.registerEmail.value.trim();
    if (!email) {
      showToast("请先填写邮箱", "error");
      return;
    }
    el.sendCodeButton.disabled = true;
    try {
      await sendCode(email);
    } catch (err) {
      showToast(err.message, "error");
    } finally {
      window.setTimeout(() => {
        el.sendCodeButton.disabled = false;
      }, 900);
    }
  });

  el.registerForm.addEventListener("submit", async (event) => {
    event.preventDefault();
    const payload = {
      student_id: el.registerStudentID.value.trim(),
      name: el.registerName.value.trim(),
      email: el.registerEmail.value.trim(),
      class_id: el.registerClassID.value.trim(),
      password: el.registerPassword.value,
      code: el.registerCode.value.trim(),
    };
    try {
      await registerAndLogin(payload);
    } catch (err) {
      showToast(err.message, "error");
    }
  });

  el.refreshSessionsButton.addEventListener("click", async () => {
    try {
      await loadSessions(false);
      showToast("会话已刷新");
    } catch (err) {
      showToast(err.message, "error");
    }
  });

  el.newSessionForm.addEventListener("submit", async (event) => {
    event.preventDefault();
    const title = el.newSessionTitle.value.trim() || "新会话";
    try {
      await createSession(title);
      el.newSessionTitle.value = "";
      showToast("会话已创建");
    } catch (err) {
      showToast(err.message, "error");
    }
  });

  el.logoutButton.addEventListener("click", () => {
    clearAuth();
    renderShell();
    renderSessions();
    renderMessages();
    showToast("已退出登录");
  });

  el.messageForm.addEventListener("submit", async (event) => {
    event.preventDefault();
    const query = el.messageInput.value.trim();
    if (!query || state.busy) {
      return;
    }
    el.messageInput.value = "";
    setBusy(true);
    try {
      await sendMessage(query);
    } catch (err) {
      showToast(err.message, "error");
    } finally {
      setBusy(false);
      el.messageInput.focus();
    }
  });

  el.messageInput.addEventListener("keydown", (event) => {
    if (event.key === "Enter" && !event.shiftKey) {
      event.preventDefault();
      el.messageForm.requestSubmit();
    }
  });
}

async function init() {
  bindEvents();
  loadAuth();
  renderShell();
  renderSessions();
  renderMessages();
  if (state.token) {
    try {
      await loadSessions(true);
    } catch (err) {
      showToast(err.message, "error");
    }
  }
}

init();
