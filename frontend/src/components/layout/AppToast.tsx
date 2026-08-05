import { Toast, ToastIcon, ToastProvider, ToastViewport } from "@/components/ui/toast";

type ToastType = "ok" | "error";

interface ToastState {
  message: string;
  type: ToastType;
}

interface AppToastProps {
  toast?: ToastState;
  onDismiss: () => void;
}

export function AppToast({ toast, onDismiss }: AppToastProps) {
  return (
    <ToastProvider swipeDirection="right">
      {toast && (
        <Toast
          key={`toast-${Date.now()}`}
          variant={toast.type}
          open={!!toast}
          onOpenChange={(open) => {
            if (!open) onDismiss();
          }}
          duration={3800}
        >
          <ToastIcon variant={toast.type} />
          <span className="min-w-0 overflow-hidden text-ellipsis">{toast.message}</span>
        </Toast>
      )}
      <ToastViewport />
    </ToastProvider>
  );
}
