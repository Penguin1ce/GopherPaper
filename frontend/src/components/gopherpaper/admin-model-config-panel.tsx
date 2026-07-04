"use client";

import {
  Activity,
  AlertTriangle,
  CheckCircle2,
  Clock3,
  Gauge,
  Loader2,
  RotateCcw,
  Save,
  SlidersHorizontal,
  TestTube2,
  Zap,
} from "lucide-react";
import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from "react";

import { adminRequest } from "@/components/gopherpaper/admin-auth";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger } from "@/components/ui/select";

type ModelKind = "chat" | "embedding" | "rerank";

type ModelConfigItem = {
  role: string;
  label: string;
  description: string;
  kind: ModelKind;
  provider: string;
  base_url: string;
  model: string;
  api_key_mask: string;
  has_api_key: boolean;
  dim: number;
  max_tokens: number;
  reasoning_effort: string;
  thinking: string;
  enabled: boolean;
  timeout: number;
  source: string;
  active: boolean;
  restart_required: boolean;
  last_test_status?: string;
  last_test_error?: string;
  last_test_at?: string;
  last_test_latency_ms?: number;
  warnings?: string[];
};

type ModelConfigListResponse = {
  items: ModelConfigItem[];
};

type ApplyResponse = {
  applied_roles: string[];
  warnings?: string[];
};

type ModelConfigTestResponse = {
  ok: boolean;
  message: string;
  latency_ms?: number;
};

type Draft = {
  provider: string;
  base_url: string;
  model: string;
  dim: number;
  max_tokens: number;
  reasoning_effort: string;
  thinking: string;
  enabled: boolean;
  timeout: number;
};

type SaveResult = {
  ok: boolean;
  message: string;
};

type TestResultState = ModelConfigTestResponse & {
  tested_at?: string;
};

type ModelHelp = {
  summary: string;
  usage: string;
  suggestion: string;
};

type ModelConfigRequest = <T>(path: string, options?: RequestInit) => Promise<T>;

type ModelConfigScope = "admin" | "user";

type ProviderPreset = {
  id: string;
  label: string;
  provider: Draft["provider"];
  baseURL: string;
  rerankURL?: string;
  note: string;
};

const roleOrder = ["chat", "pioneer", "maodie", "intent", "vlm", "translate", "embedding", "rerank"];

const providerPresets: ProviderPreset[] = [
  {
    id: "siliconflow",
    label: "硅基流动",
    provider: "openai",
    baseURL: "https://api.siliconflow.cn/v1",
    rerankURL: "https://api.siliconflow.cn/v1/rerank",
    note: "适合国产模型、embedding 和 rerank，注意账户余额。",
  },
  {
    id: "openai",
    label: "OpenAI 官方",
    provider: "openai",
    baseURL: "https://api.openai.com/v1",
    note: "标准 OpenAI 兼容地址，适合直接使用官方 API。",
  },
  {
    id: "deepseek",
    label: "DeepSeek",
    provider: "openai",
    baseURL: "https://api.deepseek.com",
    note: "适合文本对话链路，不建议直接用于 embedding。",
  },
  {
    id: "volcengine",
    label: "火山方舟",
    provider: "openai",
    baseURL: "https://ark.cn-beijing.volces.com/api/v3",
    note: "适合豆包/方舟兼容模型，模型名按控制台复制。",
  },
  {
    id: "ollama",
    label: "Ollama 本地",
    provider: "ollama",
    baseURL: "http://127.0.0.1:11434/v1",
    note: "适合本地模型调试，需要本机 Ollama 正在运行。",
  },
  {
    id: "custom",
    label: "自定义",
    provider: "openai",
    baseURL: "",
    note: "保留当前输入，适合其它中转站或私有网关。",
  },
];

const modelHelpByRole: Record<string, ModelHelp> = {
  chat: {
    summary: "系统主力模型",
    usage: "用于论文问答、结构化抽取、研读报告和多数下游生成链路。",
    suggestion: "优先选择质量稳定、上下文能力强的模型，保证回答质量和出处表达。",
  },
  pioneer: {
    summary: "小云雀工具模型",
    usage: "用于开放式学术检索、多轮工具调用和外部论文导入。",
    suggestion: "建议选择响应快、工具调用兼容性好的模型，不必一味追求最强推理。",
  },
  maodie: {
    summary: "小耄耋局部问答",
    usage: "用于精读页围绕当前页、选中文本和局部上下文进行快速问答。",
    suggestion: "可使用轻量模型提升阅读页响应速度；留空时会回退主问答模型。",
  },
  intent: {
    summary: "问题意图分类",
    usage: "只负责判断问题应该走事实、摘要、方法、闲聊还是其它链路。",
    suggestion: "建议使用便宜快速的小模型，避免分类步骤拖慢整体问答。",
  },
  vlm: {
    summary: "视觉理解模型",
    usage: "用于论文图片描述生成，以及召回图片后的带图问答。",
    suggestion: "必须选择支持图片输入的多模态模型，否则图片相关能力会失败。",
  },
  translate: {
    summary: "精读页翻译",
    usage: "用于用户在阅读论文时触发的段落翻译，不走 RAG。",
    suggestion: "建议使用低成本、响应快、中文表达自然的小模型。",
  },
  embedding: {
    summary: "论文向量化入库",
    usage: "用于论文上传入库和知识库检索，把 chunk 写入 Milvus。",
    suggestion: "谨慎修改维度。dim 必须和 Milvus collection 对齐，变化后需要重启并重建索引。",
  },
  rerank: {
    summary: "RAG 结果重排",
    usage: "向量检索先召回候选，再由 rerank 精排后交给问答模型。",
    suggestion: "可以关闭。关闭后速度更快，但相关性可能下降。",
  },
};

function itemToDraft(item: ModelConfigItem): Draft {
  return {
    provider: item.provider || "openai",
    base_url: item.base_url || "",
    model: item.model || "",
    dim: item.dim || 0,
    max_tokens: item.max_tokens || 0,
    reasoning_effort: item.reasoning_effort || "",
    thinking: item.thinking || "",
    enabled: item.enabled,
    timeout: item.kind === "rerank" ? item.timeout || 15 : item.timeout || 0,
  };
}

function sortItems(items: ModelConfigItem[]) {
  return [...items].sort((a, b) => roleOrder.indexOf(a.role) - roleOrder.indexOf(b.role));
}

function modelHelp(role: string): ModelHelp {
  return (
    modelHelpByRole[role] ?? {
      summary: "系统 AI 链路模型",
      usage: "用于当前角色对应的模型调用。",
      suggestion: "修改前建议先测试连接，再应用配置。",
    }
  );
}

function isFallbackModelRole(role: string) {
  return role === "pioneer" || role === "maodie";
}

function isBlankFallbackDraft(item: ModelConfigItem, draft?: Draft) {
  return isFallbackModelRole(item.role) && Boolean(draft) && !normalize(draft?.base_url ?? "") && !normalize(draft?.model ?? "");
}

function fallbackModelMessage(item: ModelConfigItem) {
  return `${item.label} 未配置独立模型，当前会回退主问答模型，不影响使用。`;
}

function isSavedSource(item: ModelConfigItem) {
  return item.source === "database" || item.source === "user";
}

function sourceLabel(item: ModelConfigItem, scope: ModelConfigScope) {
  if (item.source === "user") return "个人配置";
  if (item.source === "system") return "系统默认";
  if (item.source === "database") return "管理员保存";
  return scope === "user" ? "系统默认" : "config.toml";
}

function keyStatusLabel(item: ModelConfigItem, scope: ModelConfigScope) {
  if (scope === "user" && item.source !== "user") return "未配置个人密钥";
  return item.has_api_key ? "已配置密钥" : "未配置密钥";
}

export function AdminModelConfigPanel({
  token = "",
  basePath = "/admin/model-configs",
  request,
  title = "模型配置中心",
  description = "为不同 AI 链路配置供应商、中转站、模型名和密钥。保存后不会立即影响线上请求，点击应用后普通模型与 rerank 热生效；Embedding 需要重启。",
  scope = "admin",
}: {
  token?: string;
  basePath?: string;
  request?: ModelConfigRequest;
  title?: string;
  description?: string;
  scope?: ModelConfigScope;
}) {
  const [items, setItems] = useState<ModelConfigItem[]>([]);
  const [drafts, setDrafts] = useState<Record<string, Draft>>({});
  const [keyEdits, setKeyEdits] = useState<Record<string, string>>({});
  const [replaceKeys, setReplaceKeys] = useState<Record<string, boolean>>({});
  const [loading, setLoading] = useState(false);
  const [savingRole, setSavingRole] = useState<string | null>(null);
  const [testingRole, setTestingRole] = useState<string | null>(null);
  const [restoringRole, setRestoringRole] = useState<string | null>(null);
  const [applying, setApplying] = useState(false);
  const [bulkTesting, setBulkTesting] = useState(false);
  const [bulkProgress, setBulkProgress] = useState<{ done: number; total: number; label: string } | null>(null);
  const [notice, setNotice] = useState<{ type: "ok" | "error"; text: string } | null>(null);
  const [testResults, setTestResults] = useState<Record<string, TestResultState>>({});
  const [activeRole, setActiveRole] = useState("");
  const manualActiveRef = useRef<{ role: string; until: number } | null>(null);

  const requestModelConfig = useCallback(
    async <T,>(path: string, options?: RequestInit): Promise<T> => {
      if (request) {
        return request<T>(path, options);
      }
      if (!token) {
        throw new Error("缺少管理员登录态");
      }
      return adminRequest<T>(path, token, options);
    },
    [request, token],
  );

  const loadConfigs = useCallback(async () => {
    if (!request && !token) return;
    setLoading(true);
    try {
      const data = await requestModelConfig<ModelConfigListResponse>(basePath);
      const sorted = sortItems(data.items ?? []);
      setItems(sorted);
      setDrafts(Object.fromEntries(sorted.map((item) => [item.role, itemToDraft(item)])));
      setKeyEdits({});
      setReplaceKeys({});
    } catch (err) {
      setNotice({ type: "error", text: err instanceof Error ? err.message : "模型配置加载失败" });
    } finally {
      setLoading(false);
    }
  }, [basePath, request, requestModelConfig, token]);

  useEffect(() => {
    void loadConfigs();
  }, [loadConfigs]);

  useEffect(() => {
    if (!items.length) return;
    setActiveRole((current) => (items.some((item) => item.role === current) ? current : items[0].role));

    const observer = new IntersectionObserver(
      (entries) => {
        const manualActive = manualActiveRef.current;
        if (manualActive && manualActive.until > Date.now()) {
          setActiveRole(manualActive.role);
          return;
        }
        manualActiveRef.current = null;

        const current = entries
          .filter((entry) => entry.isIntersecting)
          .sort((a, b) => b.intersectionRatio - a.intersectionRatio)[0];
        if (current?.target instanceof HTMLElement) {
          setActiveRole(current.target.dataset.role || "");
        }
      },
      {
        rootMargin: "-18% 0px -58% 0px",
        threshold: [0.15, 0.35, 0.55, 0.75],
      },
    );

    for (const item of items) {
      const el = document.getElementById(`model-${item.role}`);
      if (el) observer.observe(el);
    }

    return () => observer.disconnect();
  }, [items]);

  const selectRole = useCallback((role: string) => {
    manualActiveRef.current = { role, until: Date.now() + 900 };
    setActiveRole(role);
    document.getElementById(`model-${role}`)?.scrollIntoView({
      behavior: "smooth",
      block: "start",
    });
  }, []);

  const activeCount = useMemo(() => items.filter((item) => item.active).length, [items]);
  const testStatusByRole = useMemo(() => {
    const status = new Map<string, string>();
    for (const item of items) {
      if (item.last_test_status) status.set(item.role, item.last_test_status);
    }
    for (const [role, result] of Object.entries(testResults)) {
      status.set(role, result.ok ? "ok" : "failed");
    }
    return status;
  }, [items, testResults]);
  const passedTestCount = useMemo(
    () => items.filter((item) => testStatusByRole.get(item.role) === "ok").length,
    [items, testStatusByRole],
  );
  const failedItems = useMemo(
    () => items.filter((item) => testStatusByRole.get(item.role) === "failed"),
    [items, testStatusByRole],
  );
  const failedTestCount = failedItems.length;
  const pendingCount = useMemo(
    () =>
      items.filter((item) =>
        scope === "user" ? item.source === "user" : item.source === "database" && !item.active,
      ).length,
    [items, scope],
  );
  const canOperate = !loading && !applying && !bulkTesting;

  const focusFailedModel = useCallback(
    (role?: string) => {
      const targetRole = role || failedItems[0]?.role;
      if (!targetRole) return;
      window.requestAnimationFrame(() => selectRole(targetRole));
    },
    [failedItems, selectRole],
  );

  const clearTestResult = (role: string) => {
    setTestResults((current) => {
      const next = { ...current };
      delete next[role];
      return next;
    });
  };

  const updateDraft = (role: string, patch: Partial<Draft>) => {
    clearTestResult(role);
    setDrafts((current) => ({
      ...current,
      [role]: { ...current[role], ...patch },
    }));
  };

  const saveRole = async (
    item: ModelConfigItem,
    options: { silent?: boolean; reload?: boolean } = {},
  ): Promise<SaveResult> => {
    const draft = drafts[item.role];
    if (!draft) return { ok: false, message: "表单还没有加载完成" };
    const dirty = isDraftDirty(item, draft, Boolean(replaceKeys[item.role]));
    if (!dirty) {
      const message = "没有需要保存的修改";
      if (!options.silent) {
        setNotice({ type: "ok", text: message });
      }
      return { ok: true, message };
    }
    setSavingRole(item.role);
    try {
      if (isBlankFallbackDraft(item, draft)) {
        if (isSavedSource(item)) {
          await requestModelConfig<ModelConfigItem>(`${basePath}/${item.role}/restore`, {
            method: "POST",
          });
        }
        const message = fallbackModelMessage(item);
        const result: TestResultState = {
          ok: true,
          message,
          latency_ms: 0,
          tested_at: new Date().toISOString(),
        };
        setTestResults((current) => ({ ...current, [item.role]: result }));
        if (!options.silent) {
          setNotice({ type: "ok", text: message });
        }
        if (options.reload !== false) {
          await loadConfigs();
        }
        return { ok: true, message };
      }

      const body: Record<string, unknown> = { ...draft };
      if (replaceKeys[item.role]) {
        body.api_key = keyEdits[item.role] ?? "";
      }
      await requestModelConfig<ModelConfigItem>(`${basePath}/${item.role}`, {
        method: "PUT",
        body: JSON.stringify(body),
      });
      if (!options.silent) {
        setNotice({
          type: "ok",
          text:
            scope === "user"
              ? `${item.label} 已保存，只影响当前账号；下一次模型调用会加载新配置`
              : `${item.label} 已保存，点击“应用已保存配置”后生效`,
        });
      }
      if (options.reload !== false) {
        await loadConfigs();
      }
      return { ok: true, message: "" };
    } catch (err) {
      const message = err instanceof Error ? err.message : "保存失败";
      if (!options.silent) {
        setNotice({ type: "error", text: message });
      }
      return { ok: false, message };
    } finally {
      setSavingRole(null);
    }
  };

  const testRole = async (
    item: ModelConfigItem,
    options: { reloadAfter?: boolean } = {},
  ): Promise<TestResultState> => {
    clearTestResult(item.role);
    const fallbackToChat = isBlankFallbackDraft(item, drafts[item.role]);
    const draft = drafts[item.role];
    const dirty = draft ? isDraftDirty(item, draft, Boolean(replaceKeys[item.role])) : false;
    if (dirty) {
      const saved = await saveRole(item, { silent: true, reload: false });
      if (!saved.ok) {
        const failed: TestResultState = {
          ok: false,
          message: `保存失败：${humanizeModelTestMessage(saved.message)}`,
          tested_at: new Date().toISOString(),
        };
        setTestResults((current) => ({ ...current, [item.role]: failed }));
        return failed;
      }
    }
    if (fallbackToChat) {
      const result: TestResultState = {
        ok: true,
        message: fallbackModelMessage(item),
        latency_ms: 0,
        tested_at: new Date().toISOString(),
      };
      setTestResults((current) => ({ ...current, [item.role]: result }));
      if (options.reloadAfter !== false) {
        await loadConfigs();
      }
      return result;
    }

    setTestingRole(item.role);
    try {
      const res = await requestModelConfig<ModelConfigTestResponse>(`${basePath}/${item.role}/test`, {
        method: "POST",
      });
      const result: TestResultState = {
        ...res,
        message: res.ok ? res.message : humanizeModelTestMessage(res.message),
        tested_at: new Date().toISOString(),
      };
      setTestResults((current) => ({ ...current, [item.role]: result }));
      if (options.reloadAfter !== false) {
        await loadConfigs();
      }
      return result;
    } catch (err) {
      const result: TestResultState = {
        ok: false,
        message: humanizeModelTestMessage(err instanceof Error ? err.message : "测试失败"),
        tested_at: new Date().toISOString(),
      };
      setTestResults((current) => ({ ...current, [item.role]: result }));
      return result;
    } finally {
      setTestingRole(null);
    }
  };

  const testAll = async () => {
    if (!items.length || bulkTesting) return;
    setNotice(null);
    setBulkTesting(true);
    setBulkProgress({ done: 0, total: items.length, label: "准备测试" });
    let okCount = 0;
    const failedRoles: string[] = [];
    const failedLabels: string[] = [];
    for (const [index, item] of items.entries()) {
      setBulkProgress({ done: index, total: items.length, label: item.label });
      const result = await testRole(item, { reloadAfter: false });
      if (result.ok) {
        okCount += 1;
      } else {
        failedRoles.push(item.role);
        failedLabels.push(item.label);
      }
    }
    setBulkProgress({ done: items.length, total: items.length, label: "测试完成" });
    await loadConfigs();
    setBulkTesting(false);
    setBulkProgress(null);
    setNotice({
      type: okCount === items.length ? "ok" : "error",
      text: failedRoles.length
        ? `批量测试完成：${okCount}/${items.length} 个模型通过，失败项包括：${failedLabels[0]}。可点击顶部失败统计手动定位。`
        : `批量测试完成：${okCount}/${items.length} 个模型通过，详情已回填到对应卡片`,
    });
  };

  const restoreRole = async (item: ModelConfigItem) => {
    setRestoringRole(item.role);
    try {
      await requestModelConfig<ModelConfigItem>(`${basePath}/${item.role}/restore`, {
        method: "POST",
      });
      clearTestResult(item.role);
      setNotice({
        type: "ok",
        text: scope === "user" ? `${item.label} 已恢复为系统默认` : `${item.label} 已恢复为 config.toml 默认值`,
      });
      await loadConfigs();
    } catch (err) {
      setNotice({ type: "error", text: err instanceof Error ? err.message : "恢复失败" });
    } finally {
      setRestoringRole(null);
    }
  };

  const applyConfigs = async () => {
    setApplying(true);
    try {
      const res = await requestModelConfig<ApplyResponse>(`${basePath}/apply`, {
        method: "POST",
      });
      const applied = res.applied_roles.length
        ? scope === "user"
          ? `当前账号已重新加载 ${res.applied_roles.length} 项个人配置`
          : `已应用 ${res.applied_roles.length} 项配置`
        : scope === "user"
          ? "当前账号没有个人覆盖配置，继续使用系统默认"
          : "没有需要热应用的配置";
      const warnings = res.warnings?.length ? `；${res.warnings.join("；")}` : "";
      setNotice({ type: "ok", text: applied + warnings });
      await loadConfigs();
    } catch (err) {
      setNotice({ type: "error", text: err instanceof Error ? err.message : "应用失败" });
    } finally {
      setApplying(false);
    }
  };

  return (
    <section className="rounded-lg border border-border bg-card">
      <div className="grid gap-4 border-b border-border p-4 xl:grid-cols-[minmax(0,1fr)_minmax(32rem,40rem)]">
        <div className="min-w-0">
          <div className="flex items-center gap-2">
            <SlidersHorizontal className="size-4 text-primary" />
            <h1 className="text-base font-semibold">{title}</h1>
          </div>
          <p className="mt-1 max-w-3xl text-sm leading-6 text-muted-foreground">
            {description}
          </p>
        </div>
        <div className="rounded-2xl border border-border/80 bg-background/70 p-2.5 shadow-sm">
          <div className="grid grid-cols-2 gap-1.5 sm:grid-cols-4">
            <StatusMetric label="已应用" value={`${activeCount}/${items.length || 0}`} tone="neutral" />
            <StatusMetric label="可连接" value={passedTestCount} tone={passedTestCount ? "success" : "muted"} />
            <StatusMetric
              label="测试失败"
              value={failedTestCount}
              tone={failedTestCount ? "danger" : "muted"}
              onClick={failedTestCount ? () => focusFailedModel() : undefined}
              title="点击定位到第一个测试失败的模型"
            />
            <StatusMetric
              label={scope === "user" ? "个人配置" : "待应用"}
              value={pendingCount}
              tone={scope === "user" ? "muted" : pendingCount ? "warning" : "muted"}
            />
          </div>
          <div className="mt-2.5 rounded-xl border border-border/70 bg-muted/30 p-2">
            <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
              <div className="min-w-0 text-xs leading-5 text-muted-foreground">
                {bulkTesting && bulkProgress ? (
                  <span className="inline-flex items-center gap-2 text-blue-700">
                    <Loader2 className="size-3.5 animate-spin" />
                    正在测试 {bulkProgress.label}，进度 {bulkProgress.done}/{bulkProgress.total}
                  </span>
                ) : failedTestCount ? (
                  <span className="text-red-700">发现测试失败项，可点击上方红色卡片定位。</span>
                ) : scope === "user" ? (
                  <span>个人配置只作用于当前登录账号；未配置的角色继续使用系统默认。</span>
                ) : (
                  <span>先测试连通性，再应用已保存配置。</span>
                )}
              </div>
              <div className="flex shrink-0 flex-wrap items-center gap-1.5 sm:justify-end">
                <Button
                  variant="ghost"
                  size="sm"
                  className="h-8 px-2.5 text-muted-foreground hover:text-foreground"
                  onClick={() => void loadConfigs()}
                  disabled={loading || bulkTesting}
                >
                  {loading ? <Loader2 className="size-4 animate-spin" /> : null}
                  刷新
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  className="h-8 min-w-[8.5rem] bg-background"
                  onClick={() => void testAll()}
                  disabled={!canOperate || !items.length}
                >
                  {bulkTesting ? <Loader2 className="size-4 animate-spin" /> : <Activity className="size-4" />}
                  一键测试全部
                </Button>
                <Button className="h-8 shadow-sm" size="sm" onClick={() => void applyConfigs()} disabled={!canOperate}>
                  {applying ? <Loader2 className="size-4 animate-spin" /> : <Zap className="size-4" />}
                  应用配置
                </Button>
              </div>
            </div>
          </div>
        </div>
      </div>

      {bulkTesting && bulkProgress ? (
        <div className="mx-4 mt-4 h-1 overflow-hidden rounded-full bg-blue-100">
          <div
            className="h-full rounded-full bg-blue-500 transition-all duration-300"
            style={{ width: `${Math.round((bulkProgress.done / Math.max(bulkProgress.total, 1)) * 100)}%` }}
          />
        </div>
      ) : null}

      {notice && (
        <button
          type="button"
          onClick={() => setNotice(null)}
          className={`m-4 mb-0 block w-[calc(100%-2rem)] rounded-lg border px-3 py-2 text-left text-sm ${
            notice.type === "ok"
              ? "border-emerald-200 bg-emerald-50 text-emerald-800"
              : "border-red-200 bg-red-50 text-red-800"
          }`}
        >
          {notice.text}
        </button>
      )}

      <div className="grid gap-4 p-4 lg:grid-cols-[18rem_minmax(0,1fr)]">
        <ModelConfigNav
          items={items}
          activeCount={activeCount}
          activeRole={activeRole}
          loading={loading}
          onSelectRole={selectRole}
        />

        <div className="min-w-0 space-y-4">
          {loading && items.length === 0 ? (
            <div className="flex items-center justify-center gap-2 rounded-xl border border-dashed border-border py-10 text-sm text-muted-foreground">
              <Loader2 className="size-4 animate-spin" />
              正在加载模型配置
            </div>
          ) : null}

          {items.map((item) => (
            <ModelConfigCard
              key={item.role}
              item={item}
              draft={drafts[item.role]}
              apiKey={keyEdits[item.role] ?? ""}
              replaceKey={Boolean(replaceKeys[item.role])}
              busy={savingRole === item.role || testingRole === item.role || restoringRole === item.role || bulkTesting}
              saving={savingRole === item.role}
              testing={testingRole === item.role}
              restoring={restoringRole === item.role}
              testResult={testResults[item.role]}
              onDraftChange={(patch) => updateDraft(item.role, patch)}
              onAPIKeyChange={(value) => {
                clearTestResult(item.role);
                setKeyEdits((current) => ({ ...current, [item.role]: value }));
              }}
              onReplaceKeyChange={(value) => {
                clearTestResult(item.role);
                setReplaceKeys((current) => ({ ...current, [item.role]: value }));
              }}
              onSave={() => void saveRole(item)}
              onTest={() => void testRole(item)}
              onRestore={() => void restoreRole(item)}
              scope={scope}
            />
          ))}
        </div>
      </div>
    </section>
  );
}

function StatusMetric({
  label,
  value,
  tone,
  onClick,
  title,
}: {
  label: string;
  value: ReactNode;
  tone: "neutral" | "success" | "danger" | "warning" | "muted";
  onClick?: () => void;
  title?: string;
}) {
  const toneClass = {
    neutral: "border-border bg-card text-foreground",
    success: "border-emerald-200 bg-emerald-50 text-emerald-800",
    danger: "border-red-200 bg-red-50 text-red-800 hover:bg-red-100",
    warning: "border-amber-200 bg-amber-50 text-amber-800",
    muted: "border-border bg-muted/40 text-muted-foreground",
  }[tone];
  const interactiveClass = onClick
    ? "cursor-pointer focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50"
    : "cursor-default";

  return (
    <button
      type="button"
      disabled={!onClick}
      onClick={onClick}
      title={title}
      className={`rounded-xl border px-3 py-2 text-left transition ${toneClass} ${interactiveClass}`}
    >
      <span className="block text-[0.68rem] font-medium leading-none opacity-75">{label}</span>
      <span className="mt-1 block text-sm font-semibold leading-none">{value}</span>
      {onClick ? <span className="mt-1 block text-[0.65rem] leading-none opacity-75">点击定位</span> : null}
    </button>
  );
}

function ModelConfigNav({
  items,
  activeCount,
  activeRole,
  loading,
  onSelectRole,
}: {
  items: ModelConfigItem[];
  activeCount: number;
  activeRole: string;
  loading: boolean;
  onSelectRole: (role: string) => void;
}) {
  return (
    <aside className="lg:sticky lg:top-4 lg:self-start">
      <div className="flex rounded-xl border border-border bg-background/80 p-3 shadow-sm lg:min-h-[calc(100dvh-2rem)] lg:flex-col">
        <div className="border-b border-border pb-3">
          <p className="text-sm font-semibold">模型导航</p>
          <div className="mt-3 flex items-center justify-between rounded-lg bg-muted/50 px-3 py-2 text-xs">
            <span className="text-muted-foreground">已应用</span>
            <span className="font-medium">
              {activeCount}/{items.length || 0}
            </span>
          </div>
        </div>

        <nav
          className="mt-3 grid gap-2 lg:flex-1"
          style={{ gridTemplateRows: items.length ? `repeat(${items.length}, minmax(0, 1fr))` : undefined }}
        >
          {loading && items.length === 0 ? (
            <p className="rounded-lg border border-dashed border-border px-3 py-6 text-center text-xs text-muted-foreground">
              正在加载导航
            </p>
          ) : null}

          {items.map((item) => {
            const active = activeRole === item.role;
            return (
              <a
                key={item.role}
                href={`#model-${item.role}`}
                onClick={(event) => {
                  event.preventDefault();
                  onSelectRole(item.role);
                }}
                className={`group flex flex-col justify-center rounded-lg border px-3 py-2 outline-none transition focus-visible:ring-2 focus-visible:ring-ring ${
                  active
                    ? "border-primary/40 bg-primary/10 text-primary shadow-sm"
                    : "border-transparent hover:border-primary/30 hover:bg-primary/5"
                }`}
              >
                <span className="flex items-center justify-between gap-2">
                  <span className="truncate text-sm font-medium">{item.label}</span>
                  <span
                    className={`size-2 rounded-full ${
                      hasFailedTest(item)
                        ? "bg-red-500"
                        : item.restart_required
                        ? "bg-amber-500"
                        : item.active
                          ? "bg-emerald-500"
                          : "bg-muted-foreground/40"
                    }`}
                  />
                </span>
                <span className={`mt-1 block text-xs ${active ? "text-primary/80" : "text-muted-foreground"}`}>
                  {navStatusLabel(item)}
                </span>
              </a>
            );
          })}
        </nav>
      </div>
    </aside>
  );
}

function ModelConfigCard({
  item,
  draft,
  apiKey,
  replaceKey,
  busy,
  saving,
  testing,
  restoring,
  testResult,
  onDraftChange,
  onAPIKeyChange,
  onReplaceKeyChange,
  onSave,
  onTest,
  onRestore,
  scope,
}: {
  item: ModelConfigItem;
  draft?: Draft;
  apiKey: string;
  replaceKey: boolean;
  busy: boolean;
  saving: boolean;
  testing: boolean;
  restoring: boolean;
  testResult?: TestResultState;
  onDraftChange: (patch: Partial<Draft>) => void;
  onAPIKeyChange: (value: string) => void;
  onReplaceKeyChange: (value: boolean) => void;
  onSave: () => void;
  onTest: () => void;
  onRestore: () => void;
  scope: ModelConfigScope;
}) {
  if (!draft) return null;
  const help = modelHelp(item.role);
  const dirty = isDraftDirty(item, draft, replaceKey);
  const latestTest = testResult ?? persistedTestResult(item);

  return (
    <article
      id={`model-${item.role}`}
      data-role={item.role}
      className="scroll-mt-5 rounded-xl border border-border bg-background/70 p-4 shadow-sm"
    >
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <h2 className="text-sm font-semibold">{item.label}</h2>
            <ConfigStatus item={item} dirty={dirty} />
          </div>
          <p className="mt-1 text-xs leading-5 text-muted-foreground">{item.description}</p>
          <p className="mt-2 text-xs leading-5 text-muted-foreground">
            {help.summary} · {help.suggestion}
          </p>
        </div>
        <Badge variant={item.kind === "embedding" ? "secondary" : item.kind === "rerank" ? "outline" : "default"}>
          {kindLabel(item.kind)}
        </Badge>
      </div>

      <ConfigDifferencePanel item={item} dirty={dirty} scope={scope} />

      <div className="mt-4 grid gap-3 md:grid-cols-2">
        <Field label="供应商预设">
          <ProviderPresetSelect item={item} draft={draft} onDraftChange={onDraftChange} />
        </Field>

        {item.kind !== "rerank" ? (
          <Field label="供应商类型">
            <Select
              value={draft.provider}
              onValueChange={(value) => {
                if (value) onDraftChange({ provider: value });
              }}
            >
              <SelectTrigger className="h-9 w-full rounded-lg bg-background">
                <span>{draft.provider === "ollama" ? "Ollama / 本地兼容" : "OpenAI 兼容"}</span>
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="openai">OpenAI 兼容</SelectItem>
                <SelectItem value="ollama">Ollama / 本地兼容</SelectItem>
              </SelectContent>
            </Select>
          </Field>
        ) : (
          <Field label="启用 Rerank">
            <Select
              value={draft.enabled ? "enabled" : "disabled"}
              onValueChange={(value) => {
                if (value) onDraftChange({ enabled: value === "enabled" });
              }}
            >
              <SelectTrigger className="h-9 w-full rounded-lg bg-background">
                <span>{draft.enabled ? "启用" : "关闭"}</span>
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="enabled">启用</SelectItem>
                <SelectItem value="disabled">关闭</SelectItem>
              </SelectContent>
            </Select>
          </Field>
        )}

        <Field label="模型名">
          <Input
            className="h-9 rounded-lg bg-background"
            value={draft.model}
            placeholder="例如 doubao-seed-2-0-pro-260215"
            onChange={(e) => onDraftChange({ model: e.target.value })}
          />
        </Field>

        <Field label={item.kind === "rerank" ? "Rerank 端点" : "中转站 / Base URL"}>
          <Input
            className="h-9 rounded-lg bg-background"
            value={draft.base_url}
            placeholder={item.kind === "rerank" ? "https://api.example.com/v1/rerank" : "https://api.example.com/v1"}
            onChange={(e) => onDraftChange({ base_url: e.target.value })}
          />
        </Field>

        <Field label="API Key">
          <div className="flex gap-2">
            <Input
              className="h-9 rounded-lg bg-background"
              type={replaceKey ? "password" : "text"}
              value={replaceKey ? apiKey : item.api_key_mask || "未设置"}
              disabled={!replaceKey}
              placeholder="留空表示清空密钥"
              onChange={(e) => onAPIKeyChange(e.target.value)}
            />
            <Button
              type="button"
              variant="outline"
              className="h-9 shrink-0"
              onClick={() => onReplaceKeyChange(!replaceKey)}
            >
              {replaceKey ? "不修改" : "替换"}
            </Button>
          </div>
        </Field>

        {item.kind === "embedding" ? (
          <Field label="向量维度">
            <Input
              className="h-9 rounded-lg bg-background"
              type="number"
              min={1}
              value={draft.dim}
              onChange={(e) => onDraftChange({ dim: Number(e.target.value) || 0 })}
            />
          </Field>
        ) : null}

        {item.kind === "rerank" ? (
          <Field label="超时秒数">
            <Input
              className="h-9 rounded-lg bg-background"
              type="number"
              min={1}
              value={draft.timeout}
              onChange={(e) => onDraftChange({ timeout: Number(e.target.value) || 0 })}
            />
          </Field>
        ) : null}

        {item.kind === "chat" ? (
          <>
            <Field label="Max Tokens">
              <Input
                className="h-9 rounded-lg bg-background"
                type="number"
                min={0}
                value={draft.max_tokens}
                onChange={(e) => onDraftChange({ max_tokens: Number(e.target.value) || 0 })}
              />
            </Field>
            <Field label="Thinking">
              <Select
                value={draft.thinking || "default"}
                onValueChange={(value) => {
                  if (value) onDraftChange({ thinking: value === "default" ? "" : value });
                }}
              >
                <SelectTrigger className="h-9 w-full rounded-lg bg-background">
                  <span>{thinkingLabel(draft.thinking)}</span>
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="default">默认</SelectItem>
                  <SelectItem value="disabled">关闭</SelectItem>
                  <SelectItem value="enabled">启用</SelectItem>
                </SelectContent>
              </Select>
            </Field>
            <Field label="Reasoning Effort">
              <Select
                value={draft.reasoning_effort || "default"}
                onValueChange={(value) => {
                  if (value) {
                    onDraftChange({ reasoning_effort: value === "default" ? "" : value });
                  }
                }}
              >
                <SelectTrigger className="h-9 w-full rounded-lg bg-background">
                  <span>{reasoningLabel(draft.reasoning_effort)}</span>
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="default">默认</SelectItem>
                  <SelectItem value="low">低</SelectItem>
                  <SelectItem value="medium">中</SelectItem>
                  <SelectItem value="high">高</SelectItem>
                </SelectContent>
              </Select>
            </Field>
          </>
        ) : null}
      </div>

      {item.warnings?.length ? (
        <div className="mt-3 rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-xs leading-5 text-amber-800">
          {item.warnings.join("；")}
        </div>
      ) : null}

      <LatestTestPanel result={latestTest} />

      <div className="mt-4 flex flex-wrap items-center justify-between gap-2 border-t border-border pt-3">
        <span className="text-xs text-muted-foreground">
          来源：{sourceLabel(item, scope)}，{keyStatusLabel(item, scope)}
        </span>
        <div className="flex flex-wrap gap-2">
          <Button variant="outline" size="sm" onClick={onRestore} disabled={busy || !isSavedSource(item)}>
            {restoring ? <Loader2 className="size-4 animate-spin" /> : <RotateCcw className="size-4" />}
            {scope === "user" ? "恢复系统默认" : "恢复默认"}
          </Button>
          <Button variant="outline" size="sm" onClick={onTest} disabled={busy}>
            {testing ? <Loader2 className="size-4 animate-spin" /> : <TestTube2 className="size-4" />}
            保存并测试
          </Button>
          <Button size="sm" onClick={onSave} disabled={busy}>
            {saving ? <Loader2 className="size-4 animate-spin" /> : <Save className="size-4" />}
            保存
          </Button>
        </div>
      </div>
    </article>
  );
}

function ProviderPresetSelect({
  item,
  draft,
  onDraftChange,
}: {
  item: ModelConfigItem;
  draft: Draft;
  onDraftChange: (patch: Partial<Draft>) => void;
}) {
  const activePreset = detectPreset(draft.base_url);
  const selectedPreset = providerPresets.find((preset) => preset.id === activePreset) ?? providerPresets[0];

  return (
    <Select
      value={activePreset}
      onValueChange={(value) => {
        const preset = providerPresets.find((candidate) => candidate.id === value);
        if (!preset || preset.id === "custom") return;
        onDraftChange({
          provider: preset.provider,
          base_url: item.kind === "rerank" ? preset.rerankURL || preset.baseURL : preset.baseURL,
        });
      }}
    >
      <SelectTrigger className="h-9 w-full rounded-lg bg-background" title={selectedPreset.note}>
        <span>{selectedPreset.label}</span>
      </SelectTrigger>
      <SelectContent>
        {providerPresets.map((preset) => (
          <SelectItem key={preset.id} value={preset.id}>
            {preset.label}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}

function ConfigDifferencePanel({
  item,
  dirty,
  scope,
}: {
  item: ModelConfigItem;
  dirty: boolean;
  scope: ModelConfigScope;
}) {
  if (!dirty && (item.active || !isSavedSource(item))) return null;

  return (
    <div className="mt-3 rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-xs leading-5 text-amber-800">
      <div className="flex items-start gap-2">
        <AlertTriangle className="mt-0.5 size-3.5 shrink-0" />
        <div>
          <p className="font-medium">
            {dirty
              ? "当前表单有未保存修改"
              : item.restart_required
                ? "已保存，但需要重启后端生效"
                : scope === "user"
                  ? "已保存，等待当前账号重新加载"
                  : "已保存，但尚未应用到运行配置"}
          </p>
          <p className="mt-0.5">
            {dirty
              ? "先点击保存或保存并测试，否则刷新页面后这些输入会丢失。"
              : item.restart_required
                ? "Embedding 配置会影响向量入库和检索，改动后建议重启并确认向量库维度。"
                : scope === "user"
                  ? "点击页面右上角“应用已保存配置”会清理当前账号的模型缓存，不影响其他用户。"
                  : "点击页面右上角“应用已保存配置”后，普通模型和 rerank 会热生效。"}
          </p>
        </div>
      </div>
    </div>
  );
}

function LatestTestPanel({ result }: { result?: TestResultState }) {
  if (!result) return null;
  return (
    <div
      role="status"
      aria-live="polite"
      className={`mt-3 rounded-lg border px-3 py-2 text-xs leading-5 ${
        result.ok
          ? "border-emerald-200 bg-emerald-50 text-emerald-800"
          : "border-red-200 bg-red-50 text-red-800"
      }`}
    >
      <div className="flex flex-wrap items-center justify-between gap-2">
        <span className="flex items-center gap-1.5 font-medium">
          {result.ok ? <CheckCircle2 className="size-3.5" /> : <AlertTriangle className="size-3.5" />}
          {result.ok ? "测试通过" : "测试失败"}
        </span>
        <span className="flex flex-wrap items-center gap-2">
          {typeof result.latency_ms === "number" ? (
            <span className="inline-flex items-center gap-1 rounded-full bg-background/70 px-2 py-0.5 font-medium">
              <Gauge className="size-3" />
              延迟 {formatLatency(result.latency_ms)}
            </span>
          ) : null}
          {result.tested_at ? (
            <span className="inline-flex items-center gap-1 rounded-full bg-background/70 px-2 py-0.5">
              <Clock3 className="size-3" />
              {formatDateTime(result.tested_at)}
            </span>
          ) : null}
        </span>
      </div>
      <p className="mt-1 whitespace-pre-line break-words">{humanizeModelTestMessage(result.message)}</p>
    </div>
  );
}

function ConfigStatus({ item, dirty }: { item: ModelConfigItem; dirty: boolean }) {
  if (dirty) {
    return (
      <Badge variant="secondary" className="gap-1 border-blue-200 text-blue-700">
        <AlertTriangle className="size-3" />
        未保存
      </Badge>
    );
  }
  if (item.restart_required) {
    return (
      <Badge variant="secondary" className="gap-1 border-amber-200 text-amber-700">
        <AlertTriangle className="size-3" />
        重启生效
      </Badge>
    );
  }
  if (hasFailedTest(item)) {
    return (
      <Badge variant="secondary" className="gap-1 border-red-200 text-red-700">
        <AlertTriangle className="size-3" />
        连接失败
      </Badge>
    );
  }
  if (item.source === "system") {
    return (
      <Badge variant="outline" className="gap-1 border-sky-200 text-sky-700">
        <CheckCircle2 className="size-3" />
        系统默认
      </Badge>
    );
  }
  if (item.active) {
    return (
      <Badge variant="outline" className="gap-1 border-emerald-200 text-emerald-700">
        <CheckCircle2 className="size-3" />
        已应用
      </Badge>
    );
  }
  return (
    <Badge variant="secondary" className="gap-1 border-amber-200 text-amber-700">
      <AlertTriangle className="size-3" />
      待应用
    </Badge>
  );
}

function navStatusLabel(item: ModelConfigItem) {
  if (hasFailedTest(item)) return "测试失败";
  if (item.restart_required) return "需重启";
  if (item.source === "system") return "系统默认";
  if (item.active) return "已应用";
  if (isSavedSource(item)) return item.source === "user" ? "个人配置" : "待应用";
  return "默认配置";
}

function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="space-y-1.5">
      <Label className="text-xs text-muted-foreground">{label}</Label>
      {children}
    </div>
  );
}

function hasFailedTest(item: ModelConfigItem) {
  return item.last_test_status === "failed";
}

function humanizeModelTestMessage(message: string) {
  const text = message.trim();
  if (!text) return "测试失败";
  if (text.includes("本地 Ollama") || text.includes("服务已连上") || text.includes("Ollama 服务已连接")) {
    return text;
  }

  const lower = text.toLowerCase();
  if (
    lower.includes("127.0.0.1:11434") &&
    (lower.includes("actively refused") || lower.includes("connection refused") || lower.includes("connectex"))
  ) {
    return "本地 Ollama 服务没有启动，当前电脑没有程序监听 11434 端口。请先运行 `D:\\Ollama\\ollama.exe serve`，再重新测试。";
  }
  if (lower.includes("actively refused") || lower.includes("connection refused") || lower.includes("connectex")) {
    return "目标模型服务拒绝连接。请确认模型服务已经启动，或者检查 Base URL 的地址和端口是否填错。";
  }
  if (lower.includes("403 forbidden") || lower.includes("http 403")) {
    return "服务已连上，但供应商拒绝访问。常见原因是账户余额不足、API Key 权限不够、模型未开通，或中转站禁止当前模型。";
  }
  if (lower.includes("401 unauthorized") || lower.includes("http 401")) {
    return "服务已连上，但认证失败。请检查 API Key 是否填写正确，或者密钥是否已经过期。";
  }
  if (lower.includes("404 not found") || lower.includes("http 404")) {
    return "服务已连上，但接口地址不存在。OpenAI 兼容地址通常要以 `/v1` 结尾；Ollama 应使用 `http://127.0.0.1:11434/v1`。";
  }
  if (lower.includes("429") || lower.includes("too many requests")) {
    return "服务已连上，但请求被限流。请稍后再试，或检查供应商配额、并发限制和余额。";
  }
  if (lower.includes("no such host")) {
    return "模型服务域名解析失败。请检查 Base URL 的域名是否写错，或确认网络/DNS 是否正常。";
  }
  if (lower.includes("timeout") || lower.includes("deadline exceeded")) {
    return "连接模型服务超时。常见原因是网络不通、供应商服务慢、中转站不可达，或本地模型正在加载。";
  }
  if (lower.includes("tls") || lower.includes("x509") || lower.includes("certificate")) {
    return "HTTPS 证书或 TLS 握手失败。请检查 Base URL 是否应该使用 http/https，或中转站证书是否正常。";
  }
  return text;
}

function persistedTestResult(item: ModelConfigItem): TestResultState | undefined {
  if (!item.last_test_status) return undefined;
  const ok = item.last_test_status === "ok";
  return {
    ok,
    message: ok ? "最近一次连接测试通过" : humanizeModelTestMessage(item.last_test_error || "最近一次连接测试失败"),
    latency_ms: item.last_test_latency_ms,
    tested_at: item.last_test_at,
  };
}

function isDraftDirty(item: ModelConfigItem, draft: Draft, replaceKey: boolean) {
  if (replaceKey) return true;

  const baseChanged =
    normalize(draft.base_url) !== normalize(item.base_url) ||
    normalize(draft.model) !== normalize(item.model);

  if (item.kind === "rerank") {
    if (Boolean(draft.enabled) !== Boolean(item.enabled)) return true;
    if (!draft.enabled && !item.enabled) return false;
    return baseChanged || Number(draft.timeout || 0) !== Number(item.timeout || 0);
  }

  const providerChanged = normalize(draft.provider) !== normalize(item.provider || "openai");

  if (item.kind === "embedding") {
    return providerChanged || baseChanged || Number(draft.dim || 0) !== Number(item.dim || 0);
  }

  return (
    providerChanged ||
    baseChanged ||
    Number(draft.max_tokens || 0) !== Number(item.max_tokens || 0) ||
    normalize(draft.reasoning_effort) !== normalize(item.reasoning_effort) ||
    normalize(draft.thinking) !== normalize(item.thinking)
  );
}

function detectPreset(baseURL: string) {
  const normalized = normalizeBaseURL(baseURL);
  const preset = providerPresets.find((candidate) => {
    if (candidate.id === "custom") return false;
    return (
      normalizeBaseURL(candidate.baseURL) === normalized ||
      Boolean(candidate.rerankURL && normalizeBaseURL(candidate.rerankURL) === normalized)
    );
  });
  return preset?.id ?? "custom";
}

function normalize(value: string) {
  return value.trim();
}

function normalizeBaseURL(value: string) {
  return value.trim().replace(/\/+$/, "");
}

function kindLabel(kind: ModelKind) {
  if (kind === "embedding") return "向量";
  if (kind === "rerank") return "重排";
  return "对话";
}

function formatLatency(ms: number) {
  if (!Number.isFinite(ms) || ms < 0) return "未知";
  if (ms < 1000) return `${ms} ms`;
  return `${(ms / 1000).toFixed(ms < 10000 ? 2 : 1)} s`;
}

function formatDateTime(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "时间未知";
  return date.toLocaleString("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  });
}

function thinkingLabel(value: string) {
  if (value === "enabled") return "启用";
  if (value === "disabled") return "关闭";
  return "默认";
}

function reasoningLabel(value: string) {
  if (value === "low") return "低";
  if (value === "medium") return "中";
  if (value === "high") return "高";
  return "默认";
}
