import { useTranslation } from "react-i18next";

export function SidebarFooter() {
  const { t } = useTranslation();
  return (
    <div className="sidebarFoot">
      <div className="warmLine" />
      <span>{t("unofficial_launcher")}</span>
      <small>{t("for_vintage_story")}</small>
    </div>
  );
}
