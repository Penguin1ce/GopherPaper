"use client";

import { useEffect, useState, type FormEvent, type ReactNode } from "react";
import {
  ArrowLeft,
  Camera,
  CheckCircle2,
  IdCard,
  LogOut,
  Mail,
  ShieldCheck,
  UserRound,
} from "lucide-react";
import Link from "next/link";

import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { Button, buttonVariants } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Separator } from "@/components/ui/separator";
import { useApp } from "@/lib/gopherpaper/store";
import { cn } from "@/lib/utils";
import { AvatarDialog } from "./avatar-dialog";
import { Empty } from "./app-ui";
import { WorkspaceFrame, WorkspacePanel } from "./workspace-frame";

function fieldFallback(value: string | undefined, fallback = "未设置") {
  return value && value.trim() ? value : fallback;
}

export function ProfileCenter() {
  const {
    authed,
    user,
    logout,
    refreshUser,
    updateProfile,
    updateEmail,
    updateAvatar,
    clearAvatar,
    sendCode,
  } = useApp();
  const [name, setName] = useState("");
  const [avatarOpen, setAvatarOpen] = useState(false);
  const [emailEditing, setEmailEditing] = useState(false);
  const [nextEmail, setNextEmail] = useState("");
  const [emailCode, setEmailCode] = useState("");
  const [emailSending, setEmailSending] = useState(false);
  const [emailSaving, setEmailSaving] = useState(false);
  const [saving, setSaving] = useState(false);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState("");
  const [emailError, setEmailError] = useState("");

  const fallback = (user?.name || user?.student_id || "G").slice(0, 1).toUpperCase();
  const displayName = fieldFallback(user?.name, user?.student_id || "GopherPaper 用户");

  useEffect(() => {
    setName(user?.name || "");
  }, [user?.name]);

  useEffect(() => {
    if (!emailEditing) return;
    setNextEmail(user?.email || "");
    setEmailCode("");
    setEmailError("");
  }, [emailEditing, user?.email]);

  useEffect(() => {
    if (!authed) return;
    setRefreshing(true);
    refreshUser().catch(() => {}).finally(() => setRefreshing(false));
  }, [authed, refreshUser]);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSaving(true);
    setError("");
    try {
      await updateProfile({ name });
      await refreshUser();
    } catch (err) {
      setError(err instanceof Error ? err.message : "保存个人资料失败");
    } finally {
      setSaving(false);
    }
  }

  async function sendEmailCode() {
    const email = nextEmail.trim();
    setEmailError("");
    if (!email) {
      setEmailError("请输入新邮箱");
      return;
    }
    setEmailSending(true);
    try {
      await sendCode(email);
    } catch (err) {
      setEmailError(err instanceof Error ? err.message : "验证码发送失败");
    } finally {
      setEmailSending(false);
    }
  }

  async function submitEmail(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setEmailSaving(true);
    setEmailError("");
    try {
      await updateEmail({ email: nextEmail.trim(), code: emailCode.trim() });
      logout(false);
    } catch (err) {
      setEmailError(err instanceof Error ? err.message : "邮箱更新失败");
    } finally {
      setEmailSaving(false);
    }
  }

  if (!authed) {
    return (
      <main className="flex h-dvh items-center justify-center bg-muted/50 p-6">
        <div className="text-center">
          <Empty title="请先登录" text="登录后即可查看和修改个人中心资料。" />
          <Link href="/" className={cn(buttonVariants({ variant: "outline" }), "mt-4")}>
            返回首页
          </Link>
        </div>
      </main>
    );
  }

  return (
    <WorkspaceFrame>
      <WorkspacePanel as="aside" className="hidden w-96 flex-col lg:flex">
        <div className="space-y-5 border-b p-4">
          <div className="flex items-center gap-3">
            <Link
              href="/"
              className={buttonVariants({ variant: "ghost", size: "icon" })}
              title="返回工作台"
              aria-label="返回工作台"
            >
              <ArrowLeft className="size-4" />
            </Link>
            <div className="min-w-0">
              <div className="truncate text-sm font-medium">个人中心</div>
              <div className="truncate text-xs text-muted-foreground">
                账号资料 · 头像设置 · 绑定状态
              </div>
            </div>
          </div>

          <div className="rounded-2xl border bg-card p-4 shadow-sm">
            <div className="flex items-start gap-3">
              <Avatar key={user?.avatar_url || fallback} className="size-16 rounded-xl">
                {user?.avatar_url && (
                  <AvatarImage src={user.avatar_url} alt="用户头像" className="rounded-xl" />
                )}
                <AvatarFallback className="rounded-xl bg-primary font-serif text-xl font-semibold text-primary-foreground">
                  {fallback}
                </AvatarFallback>
              </Avatar>
              <div className="min-w-0 flex-1">
                <div className="truncate text-base font-semibold">{displayName}</div>
                <div className="mt-1 truncate font-mono text-xs text-muted-foreground">
                  {user?.student_id}
                </div>
              </div>
            </div>
          </div>
        </div>

        <div className="space-y-2 p-4">
          <InfoRow icon={IdCard} label="学号" value={fieldFallback(user?.student_id)} />
          <InfoRow icon={Mail} label="邮箱" value={fieldFallback(user?.email)} />
          <InfoRow icon={ShieldCheck} label="状态" value="已登录" />
        </div>
      </WorkspacePanel>

      <WorkspacePanel className="min-w-0 flex-1 overflow-auto">
        <section className="mx-auto flex w-full max-w-5xl flex-col gap-5 p-4 lg:p-6">
          <div className="flex flex-col gap-3 rounded-2xl border bg-card p-5 shadow-sm sm:flex-row sm:items-center sm:justify-between">
            <div>
              <div className="flex items-center gap-2">
                <h1 className="text-xl font-semibold tracking-tight">个人中心</h1>
                {refreshing && <Badge variant="secondary">同步中</Badge>}
              </div>
              <p className="mt-1 text-sm text-muted-foreground">
                管理头像和基础资料。学号是知识库隔离键，当前版本保持只读。
              </p>
            </div>
            <div className="flex gap-2">
              <Link href="/" className={buttonVariants({ variant: "outline" })}>
                <ArrowLeft className="size-4" />
                返回工作台
              </Link>
              <Button type="button" variant="ghost" onClick={() => logout()}>
                <LogOut className="size-4" />
                退出
              </Button>
            </div>
          </div>

          <div className="grid gap-5 lg:grid-cols-[minmax(0,1fr)_360px]">
            <Card>
              <CardHeader>
                <CardTitle>基础资料</CardTitle>
                <CardDescription>当前先支持修改显示姓名，账号 ID 暂不开放直接修改。</CardDescription>
              </CardHeader>
              <CardContent>
                <form className="space-y-5" onSubmit={submit}>
                  <div className="space-y-4">
                    <div className="max-w-xl space-y-2">
                      <Label htmlFor="profile-student-id">用户 ID / 学号</Label>
                      <Input id="profile-student-id" value={user?.student_id || ""} readOnly />
                      <p className="text-xs text-muted-foreground">
                        该字段用于论文归属、RAG 隔离和登录凭证，不能在普通资料页修改。
                      </p>
                    </div>
                    <div className="max-w-xl space-y-2">
                      <Label htmlFor="profile-name">显示姓名</Label>
                      <Input
                        id="profile-name"
                        value={name}
                        maxLength={64}
                        placeholder="输入你的姓名或昵称"
                        onChange={(event) => setName(event.target.value)}
                      />
                    </div>
                    <div className="max-w-xl space-y-2">
                      <Label htmlFor="profile-class">班级</Label>
                      <Input id="profile-class" value={user?.class_id || "未设置"} readOnly />
                    </div>
                  </div>
                  <Separator />
                  {error && (
                    <p className="rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">
                      {error}
                    </p>
                  )}
                  <div className="flex flex-col gap-2 sm:flex-row sm:justify-end">
                    <Button
                      type="button"
                      variant="outline"
                      onClick={() => setName(user?.name || "")}
                      disabled={saving}
                    >
                      重置
                    </Button>
                    <Button type="submit" disabled={saving}>
                      {saving ? "保存中..." : "保存资料"}
                    </Button>
                  </div>
                </form>
              </CardContent>
            </Card>

            <div className="space-y-5">
              <Card>
                <CardHeader>
                  <CardTitle>头像</CardTitle>
                  <CardDescription>头像会保存到后端，换设备登录后同步显示。</CardDescription>
                </CardHeader>
                <CardContent>
                  <div className="flex items-center gap-4">
                    <Avatar key={user?.avatar_url || fallback} className="size-20 rounded-2xl">
                      {user?.avatar_url && (
                        <AvatarImage src={user.avatar_url} alt="用户头像" className="rounded-2xl" />
                      )}
                      <AvatarFallback className="rounded-2xl bg-primary font-serif text-2xl font-semibold text-primary-foreground">
                        {fallback}
                      </AvatarFallback>
                    </Avatar>
                    <div className="min-w-0 flex-1">
                      <div className="text-sm font-medium">当前头像</div>
                      <p className="mt-1 text-xs text-muted-foreground">
                        可上传图片、拖动裁剪位置，也可恢复默认首字母头像。
                      </p>
                      <Button className="mt-3" type="button" onClick={() => setAvatarOpen(true)}>
                        <Camera className="size-4" />
                        调整头像
                      </Button>
                    </div>
                  </div>
                </CardContent>
              </Card>

              <Card>
                <CardHeader>
                  <CardTitle>账号绑定</CardTitle>
                  <CardDescription>邮箱修改需要验证新邮箱，成功后需要重新登录。</CardDescription>
                </CardHeader>
                <CardContent className="space-y-4">
                  <BindingRow
                    icon={Mail}
                    label="邮箱"
                    value={fieldFallback(user?.email)}
                    action={
                      <Button
                        type="button"
                        variant="outline"
                        size="sm"
                        onClick={() => setEmailEditing((value) => !value)}
                      >
                        {emailEditing ? "取消" : "编辑"}
                      </Button>
                    }
                  />
                  {emailEditing && (
                    <form className="space-y-3 rounded-lg border bg-muted/20 p-3" onSubmit={submitEmail}>
                      <div className="space-y-2">
                        <Label htmlFor="profile-new-email">新邮箱</Label>
                        <Input
                          id="profile-new-email"
                          type="email"
                          value={nextEmail}
                          placeholder="输入新的绑定邮箱"
                          onChange={(event) => setNextEmail(event.target.value)}
                        />
                      </div>
                      <div className="grid gap-2 sm:grid-cols-[minmax(0,1fr)_auto]">
                        <div className="space-y-2">
                          <Label htmlFor="profile-email-code">验证码</Label>
                          <Input
                            id="profile-email-code"
                            value={emailCode}
                            maxLength={6}
                            placeholder="6 位验证码"
                            onChange={(event) => setEmailCode(event.target.value)}
                          />
                        </div>
                        <Button
                          type="button"
                          variant="outline"
                          className="self-end"
                          disabled={emailSending || emailSaving}
                          onClick={sendEmailCode}
                        >
                          {emailSending ? "发送中..." : "发送验证码"}
                        </Button>
                      </div>
                      <p className="text-xs text-muted-foreground">
                        验证码会发送到新邮箱。绑定成功后当前登录会失效，需要重新登录。
                      </p>
                      {emailError && (
                        <p className="rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">
                          {emailError}
                        </p>
                      )}
                      <div className="flex justify-end gap-2">
                        <Button
                          type="button"
                          variant="outline"
                          disabled={emailSaving}
                          onClick={() => setEmailEditing(false)}
                        >
                          取消
                        </Button>
                        <Button type="submit" disabled={emailSaving}>
                          {emailSaving ? "绑定中..." : "确认绑定"}
                        </Button>
                      </div>
                    </form>
                  )}
                  <BindingRow icon={UserRound} label="登录账号" value={fieldFallback(user?.student_id)} />
                </CardContent>
              </Card>
            </div>
          </div>
        </section>
      </WorkspacePanel>

      <AvatarDialog
        open={avatarOpen}
        onOpenChange={setAvatarOpen}
        currentAvatarUrl={user?.avatar_url}
        fallback={fallback}
        onSave={updateAvatar}
        onClear={clearAvatar}
      />
    </WorkspaceFrame>
  );
}

function InfoRow({
  icon: Icon,
  label,
  value,
}: {
  icon: typeof IdCard;
  label: string;
  value: string;
}) {
  return (
    <div className="flex items-center gap-3 rounded-lg px-2 py-2 text-sm">
      <Icon className="size-4 text-muted-foreground" />
      <div className="min-w-0">
        <div className="text-xs text-muted-foreground">{label}</div>
        <div className="truncate">{value}</div>
      </div>
    </div>
  );
}

function BindingRow({
  icon: Icon,
  label,
  value,
  action,
}: {
  icon: typeof Mail;
  label: string;
  value: string;
  action?: ReactNode;
}) {
  return (
    <div className="flex items-center justify-between gap-3 rounded-lg border bg-muted/20 px-3 py-2">
      <div className="flex min-w-0 items-center gap-3">
        <Icon className="size-4 text-muted-foreground" />
        <div className="min-w-0">
          <div className="text-sm font-medium">{label}</div>
          <div className="truncate text-xs text-muted-foreground">{value}</div>
        </div>
      </div>
      <div className="flex shrink-0 items-center gap-2">
        <Badge>
          <CheckCircle2 className="size-3" />
          已绑定
        </Badge>
        {action}
      </div>
    </div>
  );
}
