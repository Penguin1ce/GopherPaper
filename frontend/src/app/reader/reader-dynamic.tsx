"use client";

import dynamic from "next/dynamic";

const ReaderClient = dynamic(
  () => import("./reader-client").then((mod) => mod.ReaderClient),
  { ssr: false },
);

export function ReaderDynamic() {
  return <ReaderClient />;
}
