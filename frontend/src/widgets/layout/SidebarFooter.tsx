import { useTranslation } from "react-i18next";

import { BrowserOpenURL } from "../../wailsjs/runtime/runtime";

const DISCORD_URL = "https://discord.gg/CrRHvg9UVw";

export function SidebarFooter() {
  const { t } = useTranslation();

  function openDiscord(event: React.MouseEvent<HTMLAnchorElement>) {
    event.preventDefault();
    try {
      BrowserOpenURL(DISCORD_URL);
    } catch {
      window.open(DISCORD_URL, "_blank", "noopener,noreferrer");
    }
  }

  return (
    <div className="sidebarFoot">
      <a
        className="discordLink"
        href={DISCORD_URL}
        target="_blank"
        rel="noreferrer"
        onClick={openDiscord}
      >
        <span className="discordLinkIcon">
          <svg aria-hidden="true" viewBox="0 0 24 24">
            <path d="M19.54 4.34A16.1 16.1 0 0 0 15.5 3l-.5 1.03a15 15 0 0 0-6 0L8.5 3a16.1 16.1 0 0 0-4.04 1.34C1.9 8.17 1.2 11.9 1.55 15.58A16.27 16.27 0 0 0 6.5 18l1.2-1.63a9.64 9.64 0 0 1-1.89-.91l.46-.36c3.64 1.67 7.83 1.67 11.46 0l.46.36c-.6.36-1.23.67-1.89.91l1.2 1.63a16.18 16.18 0 0 0 4.95-2.42c.41-4.27-.7-7.96-2.91-11.24ZM8.68 13.32c-1.02 0-1.86-.93-1.86-2.07s.82-2.07 1.86-2.07c1.04 0 1.87.94 1.86 2.07 0 1.14-.82 2.07-1.86 2.07Zm6.64 0c-1.02 0-1.86-.93-1.86-2.07s.82-2.07 1.86-2.07c1.04 0 1.87.94 1.86 2.07 0 1.14-.82 2.07-1.86 2.07Z" />
          </svg>
        </span>
        <span>{t("official_waxlight_discord")}</span>
      </a>
    </div>
  );
}
