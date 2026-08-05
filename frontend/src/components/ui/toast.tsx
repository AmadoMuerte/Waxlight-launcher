import * as ToastPrimitive from "@radix-ui/react-toast";
import { X } from "lucide-react";
import * as React from "react";

import { cn } from "@/lib/utils";

function ToastProvider(props: React.ComponentProps<typeof ToastPrimitive.Provider>) {
  return <ToastPrimitive.Provider data-slot="toast-provider" {...props} />;
}

function ToastViewport(props: React.ComponentProps<typeof ToastPrimitive.Viewport>) {
  return (
    <ToastPrimitive.Viewport
      data-slot="toast-viewport"
      className="fixed right-6 bottom-6 z-[40] flex w-[420px] max-w-[calc(100vw-48px)] flex-col gap-2.5 outline-none"
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
        "group pointer-events-auto relative flex w-full max-w-[420px] items-center gap-2.5 overflow-hidden rounded-[9px] p-3 pr-8 text-[13px] text-[var(--text-primary)] shadow-[0_15px_50px_#0009] transition-all",
        variant === "ok" && "border border-[#385239] bg-[#20251f]",
        variant === "error" && "border border-[#633735] bg-[#332020]",
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
      className="inline-flex h-8 shrink-0 items-center justify-center rounded-md border border-[var(--border-subtle)] bg-transparent px-3 text-[12px] font-semibold text-[#d6d0ca] transition-colors hover:bg-white/[0.04] focus:outline-2 focus:outline-[var(--amber)] disabled:pointer-events-none disabled:opacity-50"
      {...props}
    />
  );
}

function ToastClose(props: React.ComponentProps<typeof ToastPrimitive.Close>) {
  return (
    <ToastPrimitive.Close
      data-slot="toast-close"
      className="absolute right-1.5 top-1.5 flex size-6 items-center justify-center rounded-md text-[#77716d] opacity-0 transition-opacity hover:text-[var(--text-primary)] focus:opacity-100 focus:outline-2 focus:outline-[var(--amber)] group-hover:opacity-100"
      {...props}
    >
      <X className="size-3" />
    </ToastPrimitive.Close>
  );
}

function ToastIcon({ variant }: { variant?: ToastVariant }) {
  return (
    <span className={variant === "error" ? "text-[#ff988e]" : "text-[#8ed19a]"}>
      {variant === "error" ? "!" : "✓"}
    </span>
  );
}

export { Toast, ToastAction, ToastClose, ToastIcon, ToastProvider, ToastViewport };
