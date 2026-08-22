import { useEffect } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router";

import { useNotificationStore } from "../../app/stores/notifications";
import { useNewsQuery } from "../../entities/news/queries";

export function NewsSync() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { data } = useNewsQuery();
  const addNotification = useNotificationStore((state) => state.addNotification);

  useEffect(() => {
    const items = data?.newItems ?? [];
    if (items.length === 0) {
      return;
    }
    const latest = items[0];
    addNotification({
      id: `news:${latest.id}`,
      type: "info",
      title: t("news_notification_title"),
      message:
        items.length === 1 ? latest.title : t("news_notification_count", { count: items.length }),
      action: { label: t("view_news"), run: () => void navigate("/news") },
      metadata: { source: "news", latestItemId: latest.id, count: items.length },
    });
  }, [addNotification, data, navigate, t]);

  return null;
}
