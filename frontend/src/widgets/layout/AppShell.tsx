import { Settings } from "lucide-react";
import type { PropsWithChildren } from "react";
import { useTranslation } from "react-i18next";
import { NavLink } from "react-router";

import { AppToast } from "./AppToast";
import { NotificationCenter } from "./NotificationCenter";
import { Sidebar } from "./Sidebar";

export function AppShell({ children }: PropsWithChildren) {
  const { t } = useTranslation();

  return (
    <div className="shell">
      <Sidebar />
      <div className="appActions">
        <NotificationCenter />
        <NavLink
          to="/settings"
          className={({ isActive }) => `iconButton md ghost${isActive ? " active" : ""}`}
          aria-label={t("settings")}
        >
          <Settings className="size-5" />
        </NavLink>
      </div>
      <main className="appMain">{children}</main>
      <AppToast />
    </div>
  );
}
