import { Bell, CircleCheck, CircleX, Info, TriangleAlert } from "lucide-react";
import { useTranslation } from "react-i18next";

import { type AppNotification, useNotificationStore } from "../../app/stores/notifications";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "../../shared/ui/dropdown-menu";
import { IconButton } from "../../shared/ui/icon-button";

const notificationIcons = {
  info: Info,
  success: CircleCheck,
  warning: TriangleAlert,
  error: CircleX,
} as const;

function NotificationItem({ notification }: { notification: AppNotification }) {
  const markRead = useNotificationStore((state) => state.markRead);
  const Icon = notificationIcons[notification.type];

  return (
    <DropdownMenuItem
      className="items-start gap-3 px-3 py-3"
      onSelect={() => {
        markRead(notification.id);
        notification.action?.run();
      }}
    >
      <Icon
        className={notification.read ? "mt-0.5 text-text-muted" : "mt-0.5 text-warning"}
        aria-hidden="true"
      />
      <span className="min-w-0 flex-1">
        <strong className="block text-[length:var(--fs-body)] text-text-primary">
          {notification.title}
        </strong>
        <span className="mt-0.5 block text-xs leading-5 text-text-muted">
          {notification.message}
        </span>
        {notification.action && (
          <span className="mt-1 block text-[length:var(--fs-label)] font-semibold text-accent">
            {notification.action.label}
          </span>
        )}
      </span>
      {!notification.read && (
        <span className="mt-1.5 size-1.5 shrink-0 rounded-full bg-accent" aria-hidden="true" />
      )}
    </DropdownMenuItem>
  );
}

export function NotificationCenter() {
  const { t } = useTranslation();
  const notifications = useNotificationStore((state) => state.notifications);
  const markAllRead = useNotificationStore((state) => state.markAllRead);
  const unreadCount = notifications.filter((notification) => !notification.read).length;

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <IconButton
          variant="ghost"
          className={unreadCount > 0 ? "notificationBell unread" : "notificationBell"}
          aria-label={t("notifications")}
        >
          <Bell className="size-5" />
          {unreadCount > 0 && (
            <span
              className="notificationBadge"
              aria-label={t("unread_notifications", { count: unreadCount })}
            >
              {unreadCount > 99 ? "99+" : unreadCount}
            </span>
          )}
        </IconButton>
      </DropdownMenuTrigger>

      <DropdownMenuContent
        align="end"
        sideOffset={8}
        className="w-[min(360px,calc(100vw-24px))] p-1.5"
      >
        <div className="flex items-center justify-between gap-3 px-2 py-1.5">
          <strong className="text-sm text-text-primary">{t("notifications")}</strong>
          {unreadCount > 0 && (
            <button type="button" className="linkButton text-xs" onClick={markAllRead}>
              {t("mark_all_read")}
            </button>
          )}
        </div>

        {notifications.length === 0 ? (
          <p className="px-3 py-6 text-center text-xs text-text-muted">{t("no_notifications")}</p>
        ) : (
          notifications.map((notification) => (
            <NotificationItem key={notification.id} notification={notification} />
          ))
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
