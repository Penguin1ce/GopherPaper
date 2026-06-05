const API_BASE = "/api/v1";
const AUTH_KEY = "gopherpaper.auth";
const READY_STATUSES = new Set(["ready", "indexed", "extracted"]);

const state = {
  token: "",
  user: null,
  papers: [],
  sessions: [],
  messages: [],
  activePaperID: "",
  activeSessionID: "",
  busy: false,
  paperPollTimer: 0,
  ws: null,
  wsRetry: 0,
};

const el = {
  authView: document.querySelector("#authView"),
  workspaceView: document.querySelector("#workspaceView"),
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
  logoutButton: document.querySelector("#logoutButton"),
  userName: document.querySelector("#userName"),
  userLine: document.querySelector("#userLine"),
  flowPaper: document.querySelector("#flowPaper"),
  flowSession: document.querySelector("#flowSession"),
  flowQuestion: document.querySelector("#flowQuestion"),
  refreshSessionsButton: document.querySelector("#refreshSessionsButton"),
  sessionList: document.querySelector("#sessionList"),
  refreshPapersButton: document.querySelector("#refreshPapersButton"),
  uploadForm: document.querySelector("#uploadForm"),
  paperFile: document.querySelector("#paperFile"),
  fileDropTitle: document.querySelector("#fileDropTitle"),
  fileDropHint: document.querySelector("#fileDropHint"),
  uploadButton: document.querySelector("#uploadButton"),
  activePaperBox: document.querySelector("#activePaperBox"),
  activePaperTitle: document.querySelector("#activePaperTitle"),
  activePaperMeta: document.querySelector("#activePaperMeta"),
  checkPaperStatusButton: document.querySelector("#checkPaperStatusButton"),
  createSessionForPaperButton: document.querySelector("#createSessionForPaperButton"),
  paperSearchForm: document.querySelector("#paperSearchForm"),
  paperSearchInput: document.querySelector("#paperSearchInput"),
  paperList: document.querySelector("#paperList"),
  activeSessionPaper: document.querySelector("#activeSessionPaper"),
  activeTitle: document.querySelector("#activeTitle"),
  newSessionForm: document.querySelector("#newSessionForm"),
  newSessionTitle: document.querySelector("#newSessionTitle"),
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
  stopPaperPolling();
  disconnectWs();
  state.user = null;
  state.token = "";
  state.papers = [];
  state.sessions = [];
  state.messages = [];
  state.activePaperID = "";
  state.activeSessionID = "";
  localStorage.removeItem(AUTH_KEY);
}

function authHeaders(extra = {}) {
  const headers = { ...extra };
  if (state.token) {
    headers.Authorization = `Bearer ${state.token}`;
  }
  return headers;
}

async function api(path, options = {}) {
  const headers = authHeaders({
    "Content-Type": "application/json",
    ...(options.headers || {}),
  });
  const res = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers,
  });
  return readEnvelope(res);
}

async function uploadAPI(path, formData) {
  const res = await fetch(`${API_BASE}${path}`, {
    method: "POST",
    headers: authHeaders(),
    body: formData,
  });
  return readEnvelope(res);
}

async function readEnvelope(res) {
  const text = await res.text();
  let body = {};
  if (text) {
    try {
      body = JSON.parse(text);
    } catch {
      body = { message: text };
    }
  }
  if (!res.ok || body.code !== 0) {
    const message = body.message || `请求失败 ${res.status}`;
    if (res.status === 401) {
      clearAuth();
      renderAll();
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

function renderAll() {
  renderShell();
  renderFlow();
  renderPapers();
  renderSessions();
  renderMessages();
}

function renderShell() {
  const authed = Boolean(state.token);
  el.authView.classList.toggle("hidden", authed);
  el.workspaceView.classList.toggle("hidden", !authed);
  if (!authed) {
    return;
  }
  const user = state.user || {};
  el.userName.textContent = user.name || user.student_id || "同学";
  el.userLine.textContent = `${user.student_id || "未记录学号"} · ${user.email || "已登录"}`;
}

function renderFlow() {
  const hasPaper = Boolean(state.activePaperID);
  const hasSession = Boolean(state.activeSessionID);
  el.flowPaper.classList.toggle("active", hasPaper);
  el.flowSession.classList.toggle("active", hasSession);
  el.flowQuestion.classList.toggle("active", hasSession && state.messages.length > 0);
}

function paperTitle(paper) {
  return paper.title || paper.file_name || "未命名论文";
}

function sessionTitle(session) {
  return session.title || "未命名会话";
}

function activePaper() {
  return state.papers.find((paper) => paper.id === state.activePaperID) || null;
}

function activeSession() {
  return state.sessions.find((session) => session.id === state.activeSessionID) || null;
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

function formatSize(bytes) {
  if (!Number.isFinite(bytes) || bytes <= 0) {
    return "未知大小";
  }
  if (bytes < 1024 * 1024) {
    return `${(bytes / 1024).toFixed(1)} KB`;
  }
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
}

function statusLabel(status) {
  const labels = {
    uploaded: "已上传",
    parsing: "解析中",
    extracted: "已抽取",
    indexed: "已索引",
    ready: "可提问",
    failed: "失败",
  };
  return labels[status] || status || "未知";
}

function statusTone(status) {
  if (status === "ready" || status === "indexed" || status === "extracted") {
    return "ready";
  }
  if (status === "failed") {
    return "failed";
  }
  return "working";
}

function renderPapers() {
  el.paperList.innerHTML = "";
  const selected = activePaper();
  el.activePaperBox.classList.toggle("hidden", !selected);
  if (selected) {
    el.activePaperTitle.textContent = paperTitle(selected);
    el.activePaperMeta.textContent = [
      statusLabel(selected.status),
      formatSize(selected.size),
      selected.page_count ? `${selected.page_count} 页` : "",
      formatTime(selected.updated_at || selected.created_at),
    ].filter(Boolean).join(" · ");
  }

  if (state.papers.length === 0) {
    el.paperList.append(emptyNode("还没有论文", "上传 PDF 后会出现在这里。"));
    return;
  }

  state.papers.forEach((paper) => {
    const item = document.createElement("button");
    item.className = "paper-item";
    item.classList.toggle("active", paper.id === state.activePaperID);
    item.type = "button";
    item.addEventListener("click", () => selectPaper(paper.id));

    const top = document.createElement("div");
    top.className = "paper-item-top";

    const title = document.createElement("strong");
    title.textContent = paperTitle(paper);

    const badge = document.createElement("span");
    badge.className = `status-badge ${statusTone(paper.status)}`;
    badge.textContent = statusLabel(paper.status);

    const detail = document.createElement("p");
    detail.textContent = [
      paper.file_name,
      formatSize(paper.size),
      formatTime(paper.updated_at || paper.created_at),
    ].filter(Boolean).join(" · ");

    top.append(title, badge);
    item.append(top, detail);
    el.paperList.append(item);
  });
}

function renderSessions() {
  el.sessionList.innerHTML = "";
  if (state.sessions.length === 0) {
    el.sessionList.append(emptyNode("暂无会话", "创建会话后会出现在这里。"));
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

    const paper = document.createElement("span");
    paper.className = "session-time";
    paper.textContent = session.paper_id ? `论文会话 · ${formatTime(session.updated_at || session.created_at)}` : `普通会话 · ${formatTime(session.updated_at || session.created_at)}`;

    const remove = document.createElement("button");
    remove.className = "delete-session";
    remove.type = "button";
    remove.title = "删除会话";
    remove.setAttribute("aria-label", `删除 ${sessionTitle(session)}`);
    remove.textContent = "×";
    remove.addEventListener("click", () => deleteSession(session.id));

    main.append(title, paper);
    item.append(main, remove);
    el.sessionList.append(item);
  });
}

function renderMessages() {
  const session = activeSession();
  const paper = session && session.paper_id ? state.papers.find((item) => item.id === session.paper_id) : activePaper();
  el.activeTitle.textContent = session ? sessionTitle(session) : "选择或新建一个会话";
  el.activeSessionPaper.textContent = paper ? `Paper · ${paperTitle(paper)}` : "Question";
  el.emptyState.classList.toggle("hidden", Boolean(session));
  el.messageList.classList.toggle("hidden", !session);
  el.messageList.innerHTML = "";

  if (!session) {
    renderFlow();
    return;
  }
  if (state.messages.length === 0) {
    el.messageList.append(emptyNode("这个会话还没有消息", "在下方输入问题，开始第一轮论文问答。"));
    renderFlow();
    return;
  }
  state.messages.forEach((message) => {
    el.messageList.append(messageNode(message));
  });
  el.messageList.scrollTop = el.messageList.scrollHeight;
  renderFlow();
}

function emptyNode(title, text) {
  const node = document.createElement("div");
  node.className = "empty-state inline";
  const heading = document.createElement("h3");
  heading.textContent = title;
  const p = document.createElement("p");
  p.textContent = text;
  node.append(heading, p);
  return node;
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

async function loadPapers(selectFirst = false, query = "") {
  const path = query ? `/papers/search?q=${encodeURIComponent(query)}` : "/papers";
  const papers = await api(path);
  state.papers = Array.isArray(papers) ? papers : [];
  if (state.activePaperID && !state.papers.some((paper) => paper.id === state.activePaperID)) {
    state.activePaperID = "";
  }
  if (selectFirst && !state.activePaperID && state.papers.length > 0) {
    state.activePaperID = state.papers[0].id;
  }
  renderPapers();
  renderMessages();
  startPaperPolling();
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

function selectPaper(id) {
  state.activePaperID = id;
  const paper = activePaper();
  if (paper && !el.newSessionTitle.value.trim()) {
    el.newSessionTitle.value = `${paperTitle(paper)} 问答`;
  }
  renderPapers();
  renderMessages();
}

async function uploadPaper(file) {
  const formData = new FormData();
  formData.append("file", file);
  const paper = await uploadAPI("/papers", formData);
  if (paper && paper.id) {
    state.papers = [paper, ...state.papers.filter((item) => item.id !== paper.id)];
    state.activePaperID = paper.id;
    renderPapers();
    renderMessages();
    startPaperPolling();
  }
  return paper;
}

async function refreshActivePaperStatus() {
  const paper = activePaper();
  if (!paper) {
    showToast("请先选择一篇论文", "error");
    return;
  }
  const status = await api(`/papers/${encodeURIComponent(paper.id)}/status`);
  const next = { ...paper, ...status };
  state.papers = state.papers.map((item) => item.id === next.id ? next : item);
  renderPapers();
  showToast(`论文状态：${statusLabel(next.status)}`);
}

// connectWs 建立解析进度的 WebSocket 主推通道,断线自动重连。
// 状态主推走这里,轮询与手动查询只作兜底。
function connectWs() {
  if (!state.token || typeof WebSocket === "undefined") {
    return;
  }
  disconnectWs();
  const proto = location.protocol === "https:" ? "wss:" : "ws:";
  const url = `${proto}//${location.host}${API_BASE}/ws?token=${encodeURIComponent(state.token)}`;
  let socket;
  try {
    socket = new WebSocket(url);
  } catch {
    return;
  }
  state.ws = socket;
  socket.addEventListener("message", (event) => {
    let msg;
    try {
      msg = JSON.parse(event.data);
    } catch {
      return;
    }
    if (!msg || msg.type !== "paper_status" || !msg.paper_id) {
      return;
    }
    applyPaperStatus(msg.paper_id, msg.status, msg.detail);
  });
  socket.addEventListener("close", () => {
    if (state.ws !== socket) {
      return;
    }
    state.ws = null;
    // 登录态仍在则延迟重连,兜底断线场景。
    if (state.token) {
      state.wsRetry = window.setTimeout(connectWs, 3000);
    }
  });
  socket.addEventListener("error", () => socket.close());
}

function disconnectWs() {
  if (state.wsRetry) {
    window.clearTimeout(state.wsRetry);
    state.wsRetry = 0;
  }
  if (state.ws) {
    const socket = state.ws;
    state.ws = null;
    try {
      socket.close();
    } catch {
      // 忽略关闭异常
    }
  }
}

// applyPaperStatus 把 WS 推送的状态合进本地列表并刷新可提问态。
function applyPaperStatus(paperID, status, detail) {
  const existing = state.papers.find((paper) => paper.id === paperID);
  if (!existing) {
    // 列表里还没有这篇(刚上传未刷新),拉一次补齐。
    loadPapers(false).catch(() => {});
    return;
  }
  const prev = existing.status;
  if (prev === status) {
    return;
  }
  state.papers = state.papers.map((paper) =>
    paper.id === paperID ? { ...paper, status, fail_reason: detail || paper.fail_reason } : paper,
  );
  renderPapers();
  if (status === "ready") {
    showToast(`「${paperTitle(existing)}」已就绪，可提问`);
  } else if (status === "failed") {
    showToast(`「${paperTitle(existing)}」解析失败：${detail || "未知原因"}`, "error");
  }
}

function startPaperPolling() {
  stopPaperPolling();
  if (!state.token || state.papers.every((paper) => READY_STATUSES.has(paper.status) || paper.status === "failed")) {
    return;
  }
  state.paperPollTimer = window.setInterval(async () => {
    try {
      const pending = state.papers.filter((paper) => !READY_STATUSES.has(paper.status) && paper.status !== "failed");
      await Promise.all(pending.map(async (paper) => {
        const status = await api(`/papers/${encodeURIComponent(paper.id)}/status`);
        state.papers = state.papers.map((item) => item.id === paper.id ? { ...item, ...status } : item);
      }));
      renderPapers();
      if (state.papers.every((paper) => READY_STATUSES.has(paper.status) || paper.status === "failed")) {
        stopPaperPolling();
      }
    } catch (err) {
      stopPaperPolling();
      showToast(err.message, "error");
    }
  }, 4200);
}

function stopPaperPolling() {
  if (state.paperPollTimer) {
    window.clearInterval(state.paperPollTimer);
    state.paperPollTimer = 0;
  }
}

async function openSession(id) {
  state.activeSessionID = id;
  const session = activeSession();
  if (session && session.paper_id) {
    state.activePaperID = session.paper_id;
  }
  renderSessions();
  renderPapers();
  const messages = await api(`/sessions/${encodeURIComponent(id)}/messages`);
  state.messages = Array.isArray(messages) ? messages : [];
  renderMessages();
}

async function createSession(title, paperID = "") {
  const payload = { title };
  if (paperID) {
    payload.paper_id = paperID;
  }
  const session = await api("/sessions", {
    method: "POST",
    body: JSON.stringify(payload),
  });
  state.sessions = [session, ...state.sessions.filter((item) => item.id !== session.id)];
  state.activeSessionID = session.id;
  if (session.paper_id) {
    state.activePaperID = session.paper_id;
  }
  state.messages = [];
  renderSessions();
  renderPapers();
  renderMessages();
  return session;
}

async function createSessionForActivePaper() {
  const paper = activePaper();
  if (!paper) {
    showToast("请先选择一篇论文", "error");
    return;
  }
  const title = el.newSessionTitle.value.trim() || `${paperTitle(paper)} 问答`;
  await createSession(title, paper.id);
  el.newSessionTitle.value = "";
  showToast("已新建论文会话");
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
    const paper = activePaper();
    const title = paper ? `${paperTitle(paper)} 问答` : query.slice(0, 24) || "新会话";
    const session = await createSession(title, paper ? paper.id : "");
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
  connectWs();
  await Promise.all([loadPapers(true), loadSessions(true)]);
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

  el.logoutButton.addEventListener("click", () => {
    clearAuth();
    renderAll();
    showToast("已退出登录");
  });

  el.refreshSessionsButton.addEventListener("click", async () => {
    try {
      await loadSessions(false);
      showToast("会话已刷新");
    } catch (err) {
      showToast(err.message, "error");
    }
  });

  el.refreshPapersButton.addEventListener("click", async () => {
    try {
      await loadPapers(false);
      showToast("论文已刷新");
    } catch (err) {
      showToast(err.message, "error");
    }
  });

  el.paperFile.addEventListener("change", () => {
    const file = el.paperFile.files[0];
    if (!file) {
      el.fileDropTitle.textContent = "选择一篇 PDF 论文";
      el.fileDropHint.textContent = "支持 50MB 以内的 PDF 文件";
      return;
    }
    el.fileDropTitle.textContent = file.name;
    el.fileDropHint.textContent = `${formatSize(file.size)} · 等待上传`;
  });

  el.uploadForm.addEventListener("submit", async (event) => {
    event.preventDefault();
    const file = el.paperFile.files[0];
    if (!file) {
      showToast("请选择 PDF 文件", "error");
      return;
    }
    el.uploadButton.disabled = true;
    try {
      await uploadPaper(file);
      el.uploadForm.reset();
      el.fileDropTitle.textContent = "选择一篇 PDF 论文";
      el.fileDropHint.textContent = "已上传，后台会继续解析索引";
      showToast("论文已上传");
    } catch (err) {
      showToast(err.message, "error");
    } finally {
      el.uploadButton.disabled = false;
    }
  });

  el.paperSearchForm.addEventListener("submit", async (event) => {
    event.preventDefault();
    try {
      await loadPapers(false, el.paperSearchInput.value.trim());
    } catch (err) {
      showToast(err.message, "error");
    }
  });

  el.checkPaperStatusButton.addEventListener("click", async () => {
    try {
      await refreshActivePaperStatus();
    } catch (err) {
      showToast(err.message, "error");
    }
  });

  el.createSessionForPaperButton.addEventListener("click", async () => {
    try {
      await createSessionForActivePaper();
    } catch (err) {
      showToast(err.message, "error");
    }
  });

  el.newSessionForm.addEventListener("submit", async (event) => {
    event.preventDefault();
    try {
      const paper = activePaper();
      const title = el.newSessionTitle.value.trim() || (paper ? `${paperTitle(paper)} 问答` : "新会话");
      await createSession(title, paper ? paper.id : "");
      el.newSessionTitle.value = "";
      showToast("会话已创建");
    } catch (err) {
      showToast(err.message, "error");
    }
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
  renderAll();
  if (state.token) {
    connectWs();
    try {
      await Promise.all([loadPapers(true), loadSessions(true)]);
    } catch (err) {
      showToast(err.message, "error");
    }
  }
}

init();
