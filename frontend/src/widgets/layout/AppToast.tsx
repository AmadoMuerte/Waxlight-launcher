import { Toast, ToastIcon, ToastProvider, ToastViewport } from "@/shared/ui/toast";

import { useToastStore } from "../../app/stores/toast";

export function AppToast() {
  const message = useToastStore((state) => state.message);
  const type = useToastStore((state) => state.type);
  const dismiss = useToastStore((state) => state.dismiss);

  return (
    <ToastProvider swipeDirection="right">
      {message && (
        <Toast
          key={`toast-${Date.now()}`}
          variant={type ?? "ok"}
          open={!!message}
          onOpenChange={(open) => {
            if (!open) dismiss();
          }}
          duration={3800}
        >
          <ToastIcon variant={type ?? "ok"} />
          <span className="min-w-0 overflow-hidden text-ellipsis">{message}</span>
        </Toast>
      )}
      <ToastViewport />
    </ToastProvider>
  );
}
