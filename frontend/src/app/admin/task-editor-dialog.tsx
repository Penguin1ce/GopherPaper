"use client";

import { useEffect, useState } from "react";
import { Loader2 } from "lucide-react";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { createTask, updateTask, type TaskItem } from "./console-api";
import { TASK_PRIORITIES, TASK_STATUSES, dateInputToISO, toDateInput } from "./task-shared";

const selectClass =
  "h-9 w-full rounded-md border border-input bg-transparent px-2 text-sm shadow-sm outline-none focus-visible:ring-1 focus-visible:ring-ring";

type TaskEditorDialogProps = {
  token: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  task?: TaskItem | null;
  defaultStatus?: string;
  onSaved: () => void;
};

export function TaskEditorDialog({
  token,
  open,
  onOpenChange,
  task,
  defaultStatus,
  onSaved,
}: TaskEditorDialogProps) {
  const editing = Boolean(task);
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [status, setStatus] = useState("todo");
  const [priority, setPriority] = useState("medium");
  const [assignee, setAssignee] = useState("");
  const [labels, setLabels] = useState("");
  const [due, setDue] = useState("");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // 打开时用当前任务(或默认值)回填表单。
  useEffect(() => {
    if (!open) return;
    setError(null);
    if (task) {
      setTitle(task.title);
      setDescription(task.description ?? "");
      setStatus(task.status);
      setPriority(task.priority);
      setAssignee(task.assignee ?? "");
      setLabels((task.labels ?? []).join(", "));
      setDue(toDateInput(task.due_at));
    } else {
      setTitle("");
      setDescription("");
      setStatus(defaultStatus ?? "todo");
      setPriority("medium");
      setAssignee("");
      setLabels("");
      setDue("");
    }
  }, [open, task, defaultStatus]);

  const parsedLabels = labels
    .split(/[,，]/)
    .map((s) => s.trim())
    .filter(Boolean);

  const onSubmit = async () => {
    if (!title.trim()) {
      setError("请填写任务标题");
      return;
    }
    setSaving(true);
    setError(null);
    try {
      if (task) {
        await updateTask(token, task.id, {
          title: title.trim(),
          description,
          status,
          priority,
          assignee: assignee.trim(),
          labels: parsedLabels,
          ...(due ? { due_at: dateInputToISO(due) } : { clear_due: true }),
        });
      } else {
        await createTask(token, {
          title: title.trim(),
          description,
          status,
          priority,
          assignee: assignee.trim(),
          labels: parsedLabels,
          due_at: due ? dateInputToISO(due) : null,
        });
      }
      onOpenChange(false);
      onSaved();
    } catch (e) {
      setError(e instanceof Error ? e.message : "保存失败");
    } finally {
      setSaving(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={(next) => !saving && onOpenChange(next)}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{editing ? "编辑任务" : "新建任务"}</DialogTitle>
          <DialogDescription>
            {editing ? "修改任务字段后保存,变更会记入活动时间线。" : "填写任务信息,创建后可在看板中拖动流转。"}
          </DialogDescription>
        </DialogHeader>

        <div className="flex flex-col gap-3">
          <Field label="标题">
            <Input
              value={title}
              autoFocus
              placeholder="简要描述要做的事"
              onChange={(e) => setTitle(e.target.value)}
            />
          </Field>

          <Field label="描述">
            <Textarea
              value={description}
              rows={3}
              placeholder="补充背景、验收标准等(可选)"
              onChange={(e) => setDescription(e.target.value)}
            />
          </Field>

          <div className="grid grid-cols-2 gap-3">
            <Field label="状态">
              <select className={selectClass} value={status} onChange={(e) => setStatus(e.target.value)}>
                {TASK_STATUSES.map((s) => (
                  <option key={s.value} value={s.value}>
                    {s.label}
                  </option>
                ))}
              </select>
            </Field>
            <Field label="优先级">
              <select className={selectClass} value={priority} onChange={(e) => setPriority(e.target.value)}>
                {TASK_PRIORITIES.map((p) => (
                  <option key={p.value} value={p.value}>
                    {p.label}
                  </option>
                ))}
              </select>
            </Field>
          </div>

          <div className="grid grid-cols-2 gap-3">
            <Field label="负责人">
              <Input value={assignee} placeholder="姓名/工号" onChange={(e) => setAssignee(e.target.value)} />
            </Field>
            <Field label="截止日期">
              <Input type="date" value={due} onChange={(e) => setDue(e.target.value)} />
            </Field>
          </div>

          <Field label="标签(逗号分隔)">
            <Input
              value={labels}
              placeholder="后端, 性能, 高优"
              onChange={(e) => setLabels(e.target.value)}
            />
          </Field>

          {parsedLabels.length > 0 && (
            <div className="flex flex-wrap gap-1">
              {parsedLabels.map((l) => (
                <span key={l} className="rounded bg-muted px-1.5 py-0.5 text-[11px] text-muted-foreground">
                  {l}
                </span>
              ))}
            </div>
          )}

          {error && <p className="rounded-md bg-destructive/10 px-3 py-2 text-xs text-destructive">{error}</p>}
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={saving}>
            取消
          </Button>
          <Button onClick={() => void onSubmit()} disabled={saving}>
            {saving && <Loader2 className="size-4 animate-spin" />}
            {editing ? "保存" : "创建"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="flex flex-col gap-1.5">
      <span className="text-xs font-medium text-muted-foreground">{label}</span>
      {children}
    </label>
  );
}
