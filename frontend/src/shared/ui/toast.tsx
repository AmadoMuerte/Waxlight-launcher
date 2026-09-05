import * as ToastPrimitive from "@radix-ui/react-toast";
import { Check, CircleAlert, X } from "lucide-react";
import * as React from "react";

import { cn } from "@/shared/lib/utils";

function ToastProvider(props: React.ComponentProps<typeof ToastPrimitive.Provider>) {
  return <ToastPrimitive.Provider data-slot="toast-provider" {...props} />;
}

function ToastViewport(props: React.ComponentProps<typeof ToastPrimitive.Viewport>) {
  return (
    <ToastPrimitive.Viewport
      data-slot="toast-viewport"
      className="fixed right-6 bottom-6 z-[70] flex w-[var(--toast-w)] max-w-[calc(100vw-48px)] flex-col gap-2.5 outline-none"
      {...props}
    />
  );
}

type ToastVariant = "ok" | "error";

interface ToastProps extends React.ComponentProps<typeof ToastPrimitive.Root> {
  variant?: ToastVariant;
}

function Toast({ className, variant = "ok", ...props }: ToastProps) {
  return (
    <ToastPrimitive.Root
      data-slot="toast"
      className={cn(
        "group pointer-events-auto relative flex w-full max-w-[var(--toast-w)] items-center gap-2.5 overflow-hidden rounded-lg border p-3 pr-8 text-text-primary shadow-elevated transition-all",
        variant === "ok" && "border-success-border bg-success-surface",
        variant === "error" && "border-danger-border bg-danger-surface",
        "data-[swipe=cancel]:translate-x-0 data-[swipe=end]:translate-x-[var(--radix-toast-swipe-end-x)] data-[swipe=move]:translate-x-[var(--radix-toast-swipe-move-x)] data-[swipe=move]:transition-none",
        "data-[state=open]:animate-in data-[state=closed]:animate-out data-[swipe=end]:animate-out data-[state=closed]:fade-out-80 data-[state=closed]:slide-out-to-right-full data-[state=open]:slide-in-from-bottom-full",
        className,
      )}
      {...props}
    />
  );
}

function ToastAction(props: React.ComponentProps<typeof ToastPrimitive.Action>) {
  return (
    <ToastPrimitive.Action
      data-slot="toast-action"
      className="inline-flex h-8 shrink-0 items-center justify-center rounded-md border border-border-subtle bg-transparent px-3 font-semibold text-text-secondary transition-colors hover:bg-surface-ghost-hover focus:outline-2 focus:outline-accent disabled:pointer-events-none disabled:opacity-50"
      style={{ fontSize: "var(--fs-small)" }}
      {...props}
    />
  );
}

function ToastClose(props: React.ComponentProps<typeof ToastPrimitive.Close>) {
  return (
    <ToastPrimitive.Close
      data-slot="toast-close"
      className="absolute right-1.5 top-1.5 flex size-6 items-center justify-center rounded-md text-text-disabled opacity-0 transition-opacity hover:text-text-primary focus:opacity-100 focus:outline-2 focus:outline-accent group-hover:opacity-100"
      {...props}
    >
      <X className="size-3" />
    </ToastPrimitive.Close>
  );
}

function ToastIcon({ variant }: { variant?: ToastVariant }) {
  const Icon = variant === "error" ? CircleAlert : Check;
  return (
    <Icon
      size={16}
      className={cn("size-4", variant === "error" ? "text-danger-foreground" : "text-success")}
      aria-hidden="true"
    />
  );
}

export { Toast, ToastAction, ToastClose, ToastIcon, ToastProvider, ToastViewport };
