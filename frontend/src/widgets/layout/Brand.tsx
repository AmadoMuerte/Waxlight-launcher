import { useTranslation } from "react-i18next";

import appIcon from "../../assets/appicon.png";

export function Brand() {
  const { t } = useTranslation();
  return (
    <div className="brand">
      <img src={appIcon} alt="waxlight" className="brand-icon" />
      <div>
        <strong>Waxlight</strong>
        <span>{t("launcher_uppercase")}</span>
      </div>
    </div>
  );
}
