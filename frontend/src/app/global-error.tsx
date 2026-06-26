"use client";

export default function GlobalError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  return (
    <html lang="zh-CN">
      <body className="min-h-screen bg-neutral-950 text-neutral-50">
        <main className="mx-auto flex min-h-screen max-w-xl flex-col justify-center px-6">
          <p className="text-sm font-medium text-red-300">应用遇到错误</p>
          <h1 className="mt-3 text-2xl font-semibold">页面暂时无法显示</h1>
          <p className="mt-4 break-words text-sm leading-6 text-neutral-300">
            {error.message || "请重试当前操作。"}
          </p>
          <button
            type="button"
            onClick={reset}
            className="mt-8 w-fit rounded-md bg-neutral-50 px-4 py-2 text-sm font-medium text-neutral-950 transition hover:bg-neutral-200"
          >
            重试
          </button>
        </main>
      </body>
    </html>
  );
}
