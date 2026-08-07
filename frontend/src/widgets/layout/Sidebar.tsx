import { AccountSwitcher } from "./AccountSwitcher";
import { Brand } from "./Brand";
import { SidebarFooter } from "./SidebarFooter";
import { SideNav } from "./SideNav";

export function Sidebar() {
  return (
    <aside className="sidebar">
      <Brand />
      <SideNav />
      <AccountSwitcher />
      <SidebarFooter />
    </aside>
  );
}
