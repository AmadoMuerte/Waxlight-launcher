import { create } from "zustand";

export type ToastType = "ok" | "error";

interface ToastState {
  message?: string;
  type?: ToastType;
  notify: (message: string, type?: ToastType, duration?: number) => void;
  dismiss: () => void;
}

let toastTimer: number | undefined;

export const useToastStore = create<ToastState>((set) => ({
  notify: (message, type = "ok", duration = 3_800) => {
    if (toastTimer !== undefined) {
      window.clearTimeout(toastTimer);
    }
    set({ message, type });
    toastTimer = window.setTimeout(() => {
      set({ message: undefined });
    }, duration);
  },
  dismiss: () => {
    if (toastTimer !== undefined) {
      window.clearTimeout(toastTimer);
    }
    toastTimer = undefined;
    set({ message: undefined });
  },
}));
