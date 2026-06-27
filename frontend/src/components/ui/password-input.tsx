"use client";

import { Eye, EyeOff } from "lucide-react";
import * as React from "react";

import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";

type PasswordInputProps = Omit<React.ComponentProps<typeof Input>, "type"> & {
  hideLabel?: string;
  revealLabel?: string;
};

function PasswordInput({
  className,
  disabled,
  hideLabel = "隐藏密码",
  revealLabel = "显示密码",
  ...props
}: PasswordInputProps) {
  const [visible, setVisible] = React.useState(false);
  const Icon = visible ? EyeOff : Eye;

  return (
    <div className="gp-password-input relative">
      <Input
        {...props}
        disabled={disabled}
        type={visible ? "text" : "password"}
        className={cn("pr-10", className)}
      />
      <button
        type="button"
        aria-label={visible ? hideLabel : revealLabel}
        aria-pressed={visible}
        className="absolute right-1.5 top-1/2 flex size-7 -translate-y-1/2 items-center justify-center rounded-md text-muted-foreground outline-none transition-colors hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring/50 disabled:pointer-events-none disabled:opacity-50"
        disabled={disabled}
        onClick={() => setVisible((current) => !current)}
      >
        <Icon className="size-4" />
      </button>
    </div>
  );
}

export { PasswordInput };
