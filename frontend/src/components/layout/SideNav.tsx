import { useTranslation } from "react-i18next";
import { NavLink } from "react-router-dom";

import type { Operation } from "../../shared/api";

const navigation = [
  { to: "/library", icon: "▦", labelKey: "library" },
  { to: "/mods", icon: "◇", labelKey: "mods" },
  { to: "/versions", icon: "⬡", labelKey: "game_versions" },
  { to: "/operations", icon: "⇣", labelKey: "operations" },
  { to: "/accounts", icon: "♙", labelKey: "accounts" },
  { to: "/statistics", icon: "◷", labelKey: "statistics" },
  { to: "/settings", icon: "⚙", labelKey: "settings" },
] as const;

interface SideNavProps {
  operations: Operation[];
}

export function SideNav({ operations }: SideNavProps) {
  const { t } = useTranslation();
  return (
    <nav>
      {navigation.map((item) => (
        <NavLink
          key={item.to}
          to={item.to}
          className={({ isActive }) => (isActive ? "active" : "")}
        >
          <i>{item.icon}</i>
          <span>{t(item.labelKey)}</span>
          {item.to === "/operations" &&
            operations.some((operation) => operation.status === "running") && (
              <b className="navPulse" />
            )}
        </NavLink>
      ))}
    </nav>
  );
}
