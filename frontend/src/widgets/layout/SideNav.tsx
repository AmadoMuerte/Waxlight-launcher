import { useTranslation } from "react-i18next";
import { NavLink } from "react-router";

import { useOperationsQuery } from "../../entities/operation/queries";

const navigation = [
  { to: "/library", icon: "▦", labelKey: "library" },
  { to: "/mods", icon: "◇", labelKey: "mods" },
  { to: "/versions", icon: "⬡", labelKey: "game_versions" },
  { to: "/operations", icon: "⇣", labelKey: "operations" },
  { to: "/accounts", icon: "♙", labelKey: "accounts" },
  { to: "/statistics", icon: "◷", labelKey: "statistics" },
  { to: "/settings", icon: "⚙", labelKey: "settings" },
] as const;

export function SideNav() {
  const { t } = useTranslation();
  const { data: operations = [] } = useOperationsQuery();

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
