import type { PropsWithChildren } from "react";

import { AppToast } from "./AppToast";
import { Sidebar } from "./Sidebar";

export function AppShell({ children }: PropsWithChildren) {
  return (
    <div className="shell">
      <Sidebar />
      <main className="appMain">{children}</main>
      <AppToast />
    </div>
  );
}
