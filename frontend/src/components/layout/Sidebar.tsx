import type { Account, Operation } from "../../shared/api";
import { AccountSwitcher } from "./AccountSwitcher";
import { Brand } from "./Brand";
import { SidebarFooter } from "./SidebarFooter";
import { SideNav } from "./SideNav";

type Notify = (message: string, type?: "ok" | "error") => void;

interface SidebarProps {
  accounts: Account[];
  operations: Operation[];
  refresh: () => Promise<void>;
  notify: Notify;
}

export function Sidebar({ accounts, operations, refresh, notify }: SidebarProps) {
  return (
    <aside className="sidebar">
      <Brand />
      <SideNav operations={operations} />
      <AccountSwitcher accounts={accounts} refresh={refresh} notify={notify} />
      <SidebarFooter />
    </aside>
  );
}
