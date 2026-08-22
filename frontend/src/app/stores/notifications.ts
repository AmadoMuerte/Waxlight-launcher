import { create } from "zustand";

export type NotificationType = "info" | "success" | "warning" | "error";

export interface NotificationAction {
  label: string;
  run: () => void;
}

export interface AppNotification {
  id: string;
  type: NotificationType;
  title: string;
  message: string;
  createdAt: string;
  read: boolean;
  action?: NotificationAction;
  metadata?: Record<string, unknown>;
}

export type NewNotification = Omit<AppNotification, "createdAt" | "read">;

interface NotificationState {
  notifications: AppNotification[];
  addNotification: (notification: NewNotification) => void;
  removeNotification: (id: string) => void;
  markRead: (id: string) => void;
  markAllRead: () => void;
}

export const useNotificationStore = create<NotificationState>((set) => ({
  notifications: [],
  addNotification: (notification) =>
    set((state) => {
      const existing = state.notifications.find((item) => item.id === notification.id);
      const next = {
        ...notification,
        createdAt: existing?.createdAt ?? new Date().toISOString(),
        read: existing?.read ?? false,
      };
      return {
        notifications: [next, ...state.notifications.filter((item) => item.id !== notification.id)],
      };
    }),
  removeNotification: (id) =>
    set((state) => ({
      notifications: state.notifications.filter((notification) => notification.id !== id),
    })),
  markRead: (id) =>
    set((state) => ({
      notifications: state.notifications.map((notification) =>
        notification.id === id ? { ...notification, read: true } : notification,
      ),
    })),
  markAllRead: () =>
    set((state) => ({
      notifications: state.notifications.map((notification) => ({ ...notification, read: true })),
    })),
}));
