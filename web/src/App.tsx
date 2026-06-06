import { AuthView } from "./components/AuthView";
import { Workspace } from "./components/Workspace";
import { ToastStack } from "./components/ui";
import { useApp } from "./store";

export function App() {
  const { authed } = useApp();
  return (
    <div className="app-shell">
      {authed ? <Workspace /> : <AuthView />}
      <ToastStack />
    </div>
  );
}
