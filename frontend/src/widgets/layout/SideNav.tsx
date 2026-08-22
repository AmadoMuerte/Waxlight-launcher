import {
  ChartNoAxesColumn,
  Layers3,
  Library,
  ListTodo,
  Newspaper,
  PanelsTopLeft,
  Puzzle,
  Server,
} from "lucide-react";
import { useTranslation } from "react-i18next";

import { useNewsQuery } from "../../entities/news/queries";
import { useOperationsQuery } from "../../entities/operation/queries";
import { NavItem } from "./NavItem";

const navigation = [
  { to: "/library", icon: Library, labelKey: "library" },
  { to: "/mods", icon: Puzzle, labelKey: "mods" },
  { to: "/versions", icon: Layers3, labelKey: "game_versions" },
  { to: "/servers", icon: Server, labelKey: "servers" },
  { to: "/news", icon: Newspaper, labelKey: "news" },
  { to: "/operations", icon: ListTodo, labelKey: "operations" },
  { to: "/statistics", icon: ChartNoAxesColumn, labelKey: "statistics" },
] as const;

export function SideNav() {
  const { t } = useTranslation();
  const { data: operations = [] } = useOperationsQuery();
  const { data: news } = useNewsQuery();

  return (
    <nav className="sideNav">
      {navigation.map((item) => (
        <NavItem
          key={item.to}
          to={item.to}
          icon={item.icon}
          label={t(item.labelKey)}
          indicator={
            (item.to === "/operations" &&
              operations.some((operation) => operation.status === "running")) ||
            (item.to === "/news" && (news?.unreadCount ?? 0) > 0)
          }
          indicatorLabel={
            item.to === "/news" && (news?.unreadCount ?? 0) > 0
              ? t("unread_news", { count: news?.unreadCount })
              : undefined
          }
        />
      ))}
      {import.meta.env.DEV && <NavItem to="/dev/ui" icon={PanelsTopLeft} label="UI Lab" />}
    </nav>
  );
}
