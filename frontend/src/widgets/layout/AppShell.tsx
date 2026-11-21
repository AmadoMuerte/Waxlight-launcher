import { Settings } from "lucide-react";
import type { PropsWithChildren } from "react";
import { useEffect, useRef } from "react";
import { useTranslation } from "react-i18next";
import { NavLink, useLocation } from "react-router";

import { AppToast } from "./AppToast";
import { NotificationCenter } from "./NotificationCenter";
import { Sidebar } from "./Sidebar";

const scrollPositions = new Map<string, number>();

export function AppShell({ children }: PropsWithChildren) {
  const { t } = useTranslation();
  const { pathname } = useLocation();
  const mainRef = useRef<HTMLElement>(null);

  // Positions are tracked on scroll events: by the time a navigation cleanup
  // could read scrollTop, the browser has already clamped it to the shorter
  // incoming page, so the real offset only exists in the last scroll event.
  useEffect(() => {
    const main = mainRef.current;
    if (!main) return undefined;
    main.scrollTop = scrollPositions.get(pathname) ?? 0;
    const save = () => scrollPositions.set(pathname, main.scrollTop);
    main.addEventListener("scroll", save, { passive: true });
    return () => main.removeEventListener("scroll", save);
  }, [pathname]);

  return (
    <div className="shell">
      <Sidebar />
      <main className="appMain" ref={mainRef}>
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
        {children}
      </main>
      <AppToast />
    </div>
  );
}
