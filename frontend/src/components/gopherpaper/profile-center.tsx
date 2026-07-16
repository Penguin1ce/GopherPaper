"use client";

import { useEffect, useState, type FormEvent, type ReactNode } from "react";
import {
  ArrowLeft,
  Camera,
  CheckCircle2,
  MessageSquareText,
  IdCard,
  KeyRound,
  LogOut,
  Mail,
  ShieldCheck,
  UserRound,
} from "lucide-react";
import Link from "next/link";

import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { Button, buttonVariants } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { PasswordInput } from "@/components/ui/password-input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
} from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
import { useApp } from "@/lib/gopherpaper/store";
import type {
  PreferenceAnswerStyle,
  PreferenceLanguage,
  PreferenceOutputFormat,
  UserPreference,
} from "@/lib/gopherpaper/types";
import { cn } from "@/lib/utils";
import { AvatarDialog } from "./avatar-dialog";
import { Empty } from "./app-ui";
import { WorkspaceFrame, WorkspacePanel } from "./workspace-frame";

type ProfileIcon = typeof UserRound;

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
    sendPasswordResetCode,
    resetPassword,
    preference,
    refreshPreferences,
    updatePreferences,
  } = useApp();
  const [name, setName] = useState("");
  const [prefForm, setPrefForm] = useState<UserPreference>(preference);
  const [avatarOpen, setAvatarOpen] = useState(false);
  const [emailEditing, setEmailEditing] = useState(false);
  const [passwordEditing, setPasswordEditing] = useState(false);
  const [nextEmail, setNextEmail] = useState("");
  const [emailCode, setEmailCode] = useState("");
  const [passwordCode, setPasswordCode] = useState("");
  const [nextPassword, setNextPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [emailSending, setEmailSending] = useState(false);
  const [emailSaving, setEmailSaving] = useState(false);
  const [passwordSending, setPasswordSending] = useState(false);
  const [passwordSaving, setPasswordSaving] = useState(false);
  const [saving, setSaving] = useState(false);
  const [preferenceSaving, setPreferenceSaving] = useState(false);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState("");
  const [emailError, setEmailError] = useState("");
  const [passwordError, setPasswordError] = useState("");
  const [preferenceError, setPreferenceError] = useState("");

  const fallback = (user?.name || user?.student_id || "G").slice(0, 1).toUpperCase();
  const displayName = fieldFallback(user?.name, user?.student_id || "GopherPaper 用户");
  const studentID = fieldFallback(user?.student_id);
  const email = fieldFallback(user?.email);
  const classID = fieldFallback(user?.class_id);

  useEffect(() => {
    setName(user?.name || "");
  }, [user?.name]);

  useEffect(() => {
    setPrefForm(preference);
  }, [preference]);

  useEffect(() => {
    if (!emailEditing) return;
    setNextEmail(user?.email || "");
    setEmailCode("");
    setEmailError("");
  }, [emailEditing, user?.email]);

  useEffect(() => {
    if (!passwordEditing) return;
    setPasswordCode("");
    setNextPassword("");
    setConfirmPassword("");
    setPasswordError("");
  }, [passwordEditing]);

  useEffect(() => {
    if (!authed) return;
    setRefreshing(true);
    Promise.all([refreshUser(), refreshPreferences()])
      .catch(() => {})
      .finally(() => setRefreshing(false));
  }, [authed, refreshPreferences, refreshUser]);

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
    const emailValue = nextEmail.trim();
    setEmailError("");
    if (!emailValue) {
      setEmailError("请输入新邮箱");
      return;
    }
    setEmailSending(true);
    try {
      await sendCode(emailValue);
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
    } catch (err) {
      setEmailError(err instanceof Error ? err.message : "邮箱更新失败");
    } finally {
      setEmailSaving(false);
    }
  }

  async function sendPasswordCode() {
    setPasswordError("");
    if (!user?.student_id || !user?.email) {
      setPasswordError("当前账号缺少学号或绑定邮箱，无法验证身份");
      return;
    }
    setPasswordSending(true);
    try {
      await sendPasswordResetCode({
        student_id: user.student_id,
        email: user.email,
      });
    } catch (err) {
      setPasswordError(err instanceof Error ? err.message : "验证码发送失败");
    } finally {
      setPasswordSending(false);
    }
  }

  async function submitPassword(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setPasswordSaving(true);
    setPasswordError("");
    try {
      if (!user?.student_id || !user?.email) {
        throw new Error("当前账号缺少学号或绑定邮箱，无法修改密码");
      }
      if (nextPassword.length < 6) {
        throw new Error("新密码至少需要 6 位");
      }
      if (nextPassword !== confirmPassword) {
        throw new Error("两次输入的新密码不一致");
      }
      await resetPassword({
        student_id: user.student_id,
        email: user.email,
        code: passwordCode.trim(),
        password: nextPassword,
      });
    } catch (err) {
      setPasswordError(err instanceof Error ? err.message : "密码修改失败");
    } finally {
      setPasswordSaving(false);
    }
  }

  async function submitPreference(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setPreferenceSaving(true);
    setPreferenceError("");
    try {
      await updatePreferences(prefForm);
    } catch (err) {
      setPreferenceError(err instanceof Error ? err.message : "AI 回答偏好保存失败");
    } finally {
      setPreferenceSaving(false);
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
    <WorkspaceFrame className="bg-muted/40">
      <WorkspacePanel as="aside" className="hidden w-80 flex-col lg:flex">
        <div className="relative overflow-hidden border-b p-5">
          <div className="absolute -right-16 -top-16 size-36 rounded-full bg-primary/10 blur-2xl" />
          <div className="absolute -bottom-14 left-10 size-28 rounded-full bg-sienna/10 blur-2xl" />

          <div className="relative flex items-center gap-3">
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
              <div className="truncate text-xs text-muted-foreground">资料、头像与账号安全</div>
            </div>
          </div>

          <div className="relative mt-6 rounded-3xl border bg-card/90 p-4 shadow-sm">
            <div className="flex items-center gap-3">
              <ProfileAvatar avatarUrl={user?.avatar_url} fallback={fallback} className="size-14 rounded-2xl" />
              <div className="min-w-0 flex-1">
                <div className="truncate text-base font-semibold">{displayName}</div>
                <div className="mt-1 truncate font-mono text-xs text-muted-foreground">{studentID}</div>
              </div>
            </div>
          </div>
        </div>

        <nav className="space-y-2 p-4">
          <SidebarLink href="#profile-card" icon={UserRound} label="个人名片" />
          <SidebarLink href="#profile-info" icon={IdCard} label="资料设置" />
          <SidebarLink href="#ai-preferences" icon={MessageSquareText} label="AI 回答偏好" />
          <SidebarLink href="#account-security" icon={ShieldCheck} label="账号安全" />
        </nav>

        <div className="mt-auto p-4">
          <div className="rounded-2xl border bg-muted/30 p-4">
            <div className="flex items-center gap-2 text-sm font-medium">
              <ShieldCheck className="size-4 text-primary" />
              已登录
            </div>
            <p className="mt-2 text-xs leading-relaxed text-muted-foreground">
              学号作为论文归属和知识库隔离键保持只读。邮箱修改成功后需要重新登录。
            </p>
          </div>
        </div>
      </WorkspacePanel>

      <WorkspacePanel className="min-w-0 flex-1 overflow-auto">
        <section className="mx-auto flex w-full max-w-6xl flex-col gap-5 p-4 lg:p-6">
          <header
            id="profile-card"
            className="relative overflow-hidden rounded-[2rem] border bg-card p-5 shadow-sm sm:p-6"
          >
            <div className="absolute -right-20 -top-24 size-56 rounded-full bg-primary/10 blur-3xl" />
            <div className="absolute -bottom-24 left-1/3 size-52 rounded-full bg-sienna/10 blur-3xl" />

            <div className="relative flex flex-col gap-5 xl:flex-row xl:items-center xl:justify-between">
              <div className="flex flex-col gap-4 sm:flex-row sm:items-center">
                <button
                  type="button"
                  className="group relative w-fit rounded-[1.75rem] outline-none focus-visible:ring-2 focus-visible:ring-ring/60"
                  onClick={() => setAvatarOpen(true)}
                  aria-label="调整头像"
                >
                  <ProfileAvatar
                    avatarUrl={user?.avatar_url}
                    fallback={fallback}
                    className="size-24 rounded-[1.75rem] ring-1 ring-border transition-transform group-hover:scale-[1.02]"
                  />
                  <span className="absolute -bottom-2 -right-2 grid size-9 place-items-center rounded-full border bg-primary text-primary-foreground shadow-sm">
                    <Camera className="size-4" />
                  </span>
                </button>

                <div className="min-w-0">
                  <div className="flex flex-wrap items-center gap-2">
                    <h1 className="truncate font-serif text-3xl font-semibold tracking-tight">
                      {displayName}
                    </h1>
                    {refreshing && <Badge variant="secondary">同步中</Badge>}
                  </div>
                  <div className="mt-2 flex flex-wrap gap-2 text-sm text-muted-foreground">
                    <span className="inline-flex items-center gap-1.5 rounded-full border bg-background/70 px-3 py-1">
                      <IdCard className="size-3.5" />
                      {studentID}
                    </span>
                    <span className="inline-flex items-center gap-1.5 rounded-full border bg-background/70 px-3 py-1">
                      <Mail className="size-3.5" />
                      {email}
                    </span>
                    <span className="inline-flex items-center gap-1.5 rounded-full border bg-primary/10 px-3 py-1 text-primary">
                      <CheckCircle2 className="size-3.5" />
                      邮箱已绑定
                    </span>
                  </div>
                </div>
              </div>

              <div className="flex flex-wrap gap-2">
                <Button type="button" onClick={() => setAvatarOpen(true)}>
                  <Camera className="size-4" />
                  调整头像
                </Button>
                <Link href="/" className={buttonVariants({ variant: "outline" })}>
                  <ArrowLeft className="size-4" />
                  返回工作台
                </Link>
                <Button
                  type="button"
                  variant="ghost"
                  onClick={() => void logout()}
                >
                  <LogOut className="size-4" />
                  退出
                </Button>
              </div>
            </div>
          </header>

          <div className="grid gap-5 xl:grid-cols-[minmax(0,1.05fr)_minmax(22rem,0.95fr)]">
            <form
              id="profile-info"
              className="rounded-[1.75rem] border bg-card p-5 shadow-sm sm:p-6"
              onSubmit={submit}
            >
              <SectionTitle
                title="资料设置"
                description="只把能修改的内容做成表单，身份字段保持只读展示。"
              />

              <div className="mt-6 space-y-5">
                <div className="space-y-2">
                  <Label htmlFor="profile-name">显示姓名</Label>
                  <Input
                    id="profile-name"
                    value={name}
                    maxLength={64}
                    placeholder="输入你的姓名或昵称"
                    onChange={(event) => setName(event.target.value)}
                  />
                </div>

                <div className="grid gap-3 sm:grid-cols-2">
                  <ReadOnlyField icon={IdCard} label="用户 ID / 学号" value={studentID} />
                  <ReadOnlyField icon={UserRound} label="班级" value={classID} />
                </div>

                <p className="rounded-2xl border bg-muted/30 px-4 py-3 text-xs leading-relaxed text-muted-foreground">
                  学号用于论文归属、RAG 隔离和登录凭证，当前版本不在资料页开放修改。
                </p>

                {error && (
                  <p className="rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">
                    {error}
                  </p>
                )}

                <div className="flex flex-col gap-2 border-t pt-5 sm:flex-row sm:justify-end">
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
              </div>
            </form>

            <section
              id="account-security"
              className="rounded-[1.75rem] border bg-card p-5 shadow-sm sm:p-6"
            >
              <SectionTitle
                title="账号安全"
                description="管理邮箱绑定与登录账号。修改邮箱后，当前登录态会失效。"
              />

              <div className="mt-6 space-y-4">
                <BindingRow
                  icon={Mail}
                  label="邮箱"
                  value={email}
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
                  <form className="space-y-3 rounded-2xl border bg-muted/20 p-4" onSubmit={submitEmail}>
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
                    <p className="text-xs leading-relaxed text-muted-foreground">
                      验证码会发送到新邮箱。绑定成功后需要重新登录。
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

                <BindingRow
                  icon={KeyRound}
                  label="登录密码"
                  value="通过绑定邮箱验证码修改"
                  statusLabel="邮箱验证"
                  action={
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      onClick={() => setPasswordEditing((value) => !value)}
                    >
                      {passwordEditing ? "取消" : "修改"}
                    </Button>
                  }
                />

                {passwordEditing && (
                  <form className="space-y-3 rounded-2xl border bg-muted/20 p-4" onSubmit={submitPassword}>
                    <div className="rounded-xl border bg-background/70 px-3 py-2 text-xs leading-relaxed text-muted-foreground">
                      验证码会发送到当前绑定邮箱：{email}。修改成功后需要重新登录。
                    </div>
                    <div className="grid gap-2 sm:grid-cols-[minmax(0,1fr)_auto]">
                      <div className="space-y-2">
                        <Label htmlFor="profile-password-code">邮箱验证码</Label>
                        <Input
                          id="profile-password-code"
                          value={passwordCode}
                          maxLength={6}
                          placeholder="6 位验证码"
                          onChange={(event) => setPasswordCode(event.target.value)}
                        />
                      </div>
                      <Button
                        type="button"
                        variant="outline"
                        className="self-end"
                        disabled={passwordSending || passwordSaving}
                        onClick={sendPasswordCode}
                      >
                        {passwordSending ? "发送中..." : "发送验证码"}
                      </Button>
                    </div>
                    <div className="grid gap-3 sm:grid-cols-2">
                      <div className="space-y-2">
                        <Label htmlFor="profile-new-password">新密码</Label>
                        <PasswordInput
                          id="profile-new-password"
                          value={nextPassword}
                          minLength={6}
                          placeholder="至少 6 位"
                          disabled={passwordSaving}
                          onChange={(event) => setNextPassword(event.target.value)}
                        />
                      </div>
                      <div className="space-y-2">
                        <Label htmlFor="profile-confirm-password">确认新密码</Label>
                        <PasswordInput
                          id="profile-confirm-password"
                          value={confirmPassword}
                          minLength={6}
                          placeholder="再次输入新密码"
                          disabled={passwordSaving}
                          onChange={(event) => setConfirmPassword(event.target.value)}
                        />
                      </div>
                    </div>
                    {passwordError && (
                      <p className="rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">
                        {passwordError}
                      </p>
                    )}
                    <div className="flex justify-end gap-2">
                      <Button
                        type="button"
                        variant="outline"
                        disabled={passwordSaving}
                        onClick={() => setPasswordEditing(false)}
                      >
                        取消
                      </Button>
                      <Button type="submit" disabled={passwordSaving}>
                        {passwordSaving ? "修改中..." : "确认修改"}
                      </Button>
                    </div>
                  </form>
                )}

                <BindingRow icon={UserRound} label="登录账号" value={studentID} />
              </div>
            </section>
          </div>

          <form
            id="ai-preferences"
            className="overflow-hidden rounded-[1.75rem] border bg-card shadow-sm"
            onSubmit={submitPreference}
          >
            <div className="grid lg:grid-cols-[minmax(18rem,0.82fr)_minmax(0,1.35fr)]">
              <div className="relative overflow-hidden border-b bg-muted/25 p-5 sm:p-6 lg:border-b-0 lg:border-r">
                <div className="absolute -right-20 -top-20 size-44 rounded-full bg-primary/10 blur-3xl" />
                <div className="relative">
                  <div className="flex items-center gap-3">
                    <div className="grid size-11 place-items-center rounded-2xl bg-primary/10 text-primary">
                      <MessageSquareText className="size-5" />
                    </div>
                    <div>
                      <h2 className="text-lg font-semibold tracking-tight">AI 回答偏好</h2>
                      <p className="mt-1 text-sm text-muted-foreground">控制回答方式，不改变科研可靠性规则。</p>
                    </div>
                  </div>

                  <div className="mt-6 space-y-3 rounded-2xl border bg-background/70 p-4 text-xs leading-relaxed text-muted-foreground">
                    <p>
                      <span className="font-medium text-foreground">可以自定义：</span>
                      称呼、回答详略、输出形式、语言习惯和补充表达要求。
                    </p>
                    <p>
                      <span className="font-medium text-foreground">不可覆盖：</span>
                      论文问答必须基于用户可见知识库，并保留可追溯出处。
                    </p>
                    <p>
                      保存后会在后续默认论文问答和小云雀对话中生效。
                    </p>
                  </div>
                </div>
              </div>

              <div className="p-5 sm:p-6">
                <div className="grid gap-6 xl:grid-cols-[minmax(0,0.95fr)_minmax(0,1.15fr)]">
                  <div className="space-y-5">
                    <div className="space-y-2">
                      <Label htmlFor="preference-nickname">希望 AI 如何称呼你</Label>
                      <Input
                        id="preference-nickname"
                        value={prefForm.nickname}
                        maxLength={64}
                        placeholder="例如：同学、小王、Kurumi"
                        onChange={(event) =>
                          setPrefForm((value) => ({ ...value, nickname: event.target.value }))
                        }
                      />
                    </div>

                    <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-1">
                      <PreferenceSelect
                        label="回答风格"
                        value={prefForm.answer_style}
                        onValueChange={(value) =>
                          setPrefForm((prev) => ({
                            ...prev,
                            answer_style: value as PreferenceAnswerStyle,
                          }))
                        }
                        options={[
                          ["concise", "简洁直接"],
                          ["detailed", "详细解释"],
                          ["academic", "学术严谨"],
                          ["beginner", "新手友好"],
                        ]}
                      />
                      <PreferenceSelect
                        label="输出形式"
                        value={prefForm.output_format}
                        onValueChange={(value) =>
                          setPrefForm((prev) => ({
                            ...prev,
                            output_format: value as PreferenceOutputFormat,
                          }))
                        }
                        options={[
                          ["conclusion_first", "先结论后解释"],
                          ["bullets", "多用要点"],
                          ["table", "适合时用表格"],
                          ["default", "默认段落"],
                        ]}
                      />
                      <PreferenceSelect
                        label="语言偏好"
                        value={prefForm.language}
                        onValueChange={(value) =>
                          setPrefForm((prev) => ({
                            ...prev,
                            language: value as PreferenceLanguage,
                          }))
                        }
                        options={[
                          ["auto", "跟随提问"],
                          ["zh", "总是中文"],
                          ["bilingual", "关键术语中英对照"],
                        ]}
                      />
                    </div>
                  </div>

                  <div className="space-y-2">
                    <div className="flex items-center justify-between gap-3">
                      <Label htmlFor="preference-custom">补充回答要求</Label>
                      <span className="text-xs text-muted-foreground">
                        {prefForm.custom_instruction.length}/500
                      </span>
                    </div>
                    <Textarea
                      id="preference-custom"
                      value={prefForm.custom_instruction}
                      maxLength={500}
                      rows={7}
                      placeholder="例如：解释公式时多给直观例子；回答论文方法时先给整体流程，再展开细节。"
                      className="min-h-48 resize-none rounded-2xl bg-background"
                      onChange={(event) =>
                        setPrefForm((value) => ({
                          ...value,
                          custom_instruction: event.target.value,
                        }))
                      }
                    />
                    <p className="text-xs leading-relaxed text-muted-foreground">
                      补充要求只影响表达方式；“必须基于知识库、保留出处、不能编造”仍由系统规则强制保证。
                    </p>
                  </div>
                </div>

                {preferenceError && (
                  <p className="mt-5 rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">
                    {preferenceError}
                  </p>
                )}

                <div className="mt-6 flex flex-col gap-2 border-t pt-5 sm:flex-row sm:justify-end">
                  <Button
                    type="button"
                    variant="outline"
                    onClick={() => setPrefForm(preference)}
                    disabled={preferenceSaving}
                  >
                    重置
                  </Button>
                  <Button type="submit" disabled={preferenceSaving}>
                    {preferenceSaving ? "保存中..." : "保存偏好"}
                  </Button>
                </div>
              </div>
            </div>
          </form>
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

function ProfileAvatar({
  avatarUrl,
  fallback,
  className,
}: {
  avatarUrl?: string;
  fallback: string;
  className?: string;
}) {
  return (
    <Avatar key={avatarUrl || fallback} className={className}>
      {avatarUrl && <AvatarImage src={avatarUrl} alt="用户头像" />}
      <AvatarFallback className="bg-primary font-serif text-2xl font-semibold text-primary-foreground">
        {fallback}
      </AvatarFallback>
    </Avatar>
  );
}

function SidebarLink({
  href,
  icon: Icon,
  label,
}: {
  href: string;
  icon: ProfileIcon;
  label: string;
}) {
  return (
    <a
      href={href}
      className="flex items-center gap-3 rounded-2xl px-3 py-2 text-sm text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
    >
      <Icon className="size-4" />
      {label}
    </a>
  );
}

function SectionTitle({ title, description }: { title: string; description: string }) {
  return (
    <div>
      <h2 className="text-lg font-semibold tracking-tight">{title}</h2>
      <p className="mt-1 text-sm leading-relaxed text-muted-foreground">{description}</p>
    </div>
  );
}

function ReadOnlyField({
  icon: Icon,
  label,
  value,
}: {
  icon: ProfileIcon;
  label: string;
  value: string;
}) {
  return (
    <div className="rounded-2xl border bg-muted/20 p-4">
      <div className="flex items-center gap-2 text-xs text-muted-foreground">
        <Icon className="size-3.5" />
        {label}
      </div>
      <div className="mt-2 truncate font-mono text-sm">{value}</div>
      <Badge variant="outline" className="mt-3">
        只读
      </Badge>
    </div>
  );
}

function PreferenceSelect({
  label,
  value,
  onValueChange,
  options,
}: {
  label: string;
  value: string;
  onValueChange: (value: string) => void;
  options: [string, string][];
}) {
  const selectedLabel =
    options.find(([optionValue]) => optionValue === value)?.[1] || value;
  return (
    <div className="space-y-2">
      <Label>{label}</Label>
      <Select value={value} onValueChange={(next) => next && onValueChange(next)}>
        <SelectTrigger className="h-10 w-full rounded-xl bg-background">
          <span className="truncate">{selectedLabel}</span>
        </SelectTrigger>
        <SelectContent>
          {options.map(([optionValue, optionLabel]) => (
            <SelectItem key={optionValue} value={optionValue}>
              {optionLabel}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  );
}

function BindingRow({
  icon: Icon,
  label,
  value,
  action,
  statusLabel = "已绑定",
}: {
  icon: ProfileIcon;
  label: string;
  value: string;
  action?: ReactNode;
  statusLabel?: string;
}) {
  return (
    <div className="flex items-center justify-between gap-3 rounded-2xl border bg-muted/20 px-4 py-3">
      <div className="flex min-w-0 items-center gap-3">
        <div className="grid size-10 shrink-0 place-items-center rounded-xl bg-background text-muted-foreground">
          <Icon className="size-4" />
        </div>
        <div className="min-w-0">
          <div className="text-sm font-medium">{label}</div>
          <div className="truncate text-xs text-muted-foreground">{value}</div>
        </div>
      </div>
      <div className="flex shrink-0 items-center gap-2">
        <Badge>
          <CheckCircle2 className="size-3" />
          {statusLabel}
        </Badge>
        {action}
      </div>
    </div>
  );
}
