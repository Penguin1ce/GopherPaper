import type { Metadata } from "next";
import localFont from "next/font/local";
import { TooltipProvider } from "@/components/ui/tooltip";
import "./globals.css";

// 字体自托管(woff2 落仓库 src/app/fonts),不走 next/font/google ——
// 构建不依赖 fonts.gstatic.com(国内网络拉不到会让 Turbopack 解析失败)。
// 单一 grotesque(Manrope)统管标题与正文,靠字重拉层级;CJK 走 PingFang/微软雅黑回退。
const sans = localFont({
  src: "./fonts/Manrope.woff2",
  weight: "200 800",
  variable: "--font-sans-latin",
  display: "swap",
});

// 技术感小标签/引用 chip 走等宽。
const mono = localFont({
  src: "./fonts/JetBrainsMono.woff2",
  weight: "100 800",
  variable: "--font-mono-latin",
  display: "swap",
});

export const metadata: Metadata = {
  title: "GopherPaper",
  description: "科研文献智能解析与知识服务系统",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html
      lang="zh-CN"
      className={`${sans.variable} ${mono.variable} h-full antialiased`}
      suppressHydrationWarning
    >
      <body className="flex min-h-full flex-col">
        <TooltipProvider>{children}</TooltipProvider>
      </body>
    </html>
  );
}
