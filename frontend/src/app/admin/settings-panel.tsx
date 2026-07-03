"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Loader2, Save, Settings2 } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { type SettingItem, fetchSettings, updateSettings } from "./console-api";

export function SettingsPanel({ token }: { token: string }) {
  const [items, setItems] = useState<SettingItem[]>([]);
  const [values, setValues] = useState<Record<string, string>>({});
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);

  const load = useCallback(async () => {
    if (!token) return;
    setLoading(true);
    try {
      const res = await fetchSettings(token);
      setItems(res.items ?? []);
      const v: Record<string, string> = {};
      for (const it of res.items ?? []) v[it.key] = it.value;
      setValues(v);
    } catch {
      setItems([]);
    } finally {
      setLoading(false);
    }
  }, [token]);

  useEffect(() => {
    void load();
  }, [load]);

  const groups = useMemo(() => {
    const m = new Map<string, SettingItem[]>();
    for (const it of items) {
      const arr = m.get(it.group) ?? [];
      arr.push(it);
      m.set(it.group, arr);
    }
    return Array.from(m.entries());
  }, [items]);

  const onSave = async () => {
    setSaving(true);
    setSaved(false);
    try {
      await updateSettings(
        token,
        items.map((it) => ({ key: it.key, value: values[it.key] ?? it.value })),
      );
      setSaved(true);
      setTimeout(() => setSaved(false), 2000);
    } catch {
      // ignore
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="rounded-xl border border-border bg-card p-4 shadow-sm">
      <div className="mb-3 flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Settings2 className="size-4 text-primary" />
          <h3 className="text-sm font-semibold">系统设置</h3>
        </div>
        <Button size="sm" onClick={() => void onSave()} disabled={saving || loading}>
          {saving ? <Loader2 className="size-4 animate-spin" /> : <Save className="size-4" />}
          {saved ? "已保存" : "保存"}
        </Button>
      </div>

      {loading ? (
        <div className="py-10 text-center text-muted-foreground">
          <Loader2 className="mx-auto mb-2 size-5 animate-spin" />
          加载中
        </div>
      ) : (
        <div className="grid gap-5 md:grid-cols-2">
          {groups.map(([group, groupItems]) => (
            <div key={group} className="rounded-lg border border-border/60 p-3">
              <h4 className="mb-3 text-xs font-semibold uppercase text-muted-foreground">{group}</h4>
              <div className="space-y-3">
                {groupItems.map((it) => (
                  <div key={it.key} className="flex items-center justify-between gap-3">
                    <span className="text-sm">{it.label}</span>
                    {it.type === "bool" ? (
                      <button
                        type="button"
                        role="switch"
                        aria-checked={values[it.key] === "true"}
                        onClick={() =>
                          setValues((v) => ({ ...v, [it.key]: v[it.key] === "true" ? "false" : "true" }))
                        }
                        className={`relative h-5 w-9 shrink-0 rounded-full transition-colors ${
                          values[it.key] === "true" ? "bg-primary" : "bg-muted"
                        }`}
                      >
                        <span
                          className={`absolute top-0.5 size-4 rounded-full bg-white transition-transform ${
                            values[it.key] === "true" ? "translate-x-4" : "translate-x-0.5"
                          }`}
                        />
                      </button>
                    ) : (
                      <Input
                        className="h-8 w-28"
                        inputMode={it.type === "number" ? "numeric" : undefined}
                        value={values[it.key] ?? ""}
                        onChange={(e) => setValues((v) => ({ ...v, [it.key]: e.target.value }))}
                      />
                    )}
                  </div>
                ))}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
