import { Sidebar } from "./Sidebar";
import { PaperPane } from "./PaperPane";
import { RightPane } from "./RightPane";

export function Workspace() {
  return (
    <section className="workspace-view">
      <Sidebar />
      <PaperPane />
      <RightPane />
    </section>
  );
}
