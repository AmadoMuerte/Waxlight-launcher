import { useTranslation } from "react-i18next";

interface ErrorBannerProps {
  message: string;
  onRetry: () => Promise<void>;
}

export function ErrorBanner({ message, onRetry }: ErrorBannerProps) {
  const { t } = useTranslation();
  return (
    <div className="backendError">
      <span>!</span>
      <div>
        <strong>{t("could_not_connect_to_core")}</strong>
        <p>{message}</p>
      </div>
      <button onClick={() => void onRetry()}>{t("retry")}</button>
    </div>
  );
}
