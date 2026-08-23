import type { ReactNode } from "react";

import { Dialog, DialogContent, DialogHeader, DialogTitle } from "./dialog";

export function Modal({
  title,
  onClose,
  children,
  className = "",
  closable = true,
}: {
  title: string;
  onClose: () => void;
  children: ReactNode;
  className?: string;
  closable?: boolean;
}) {
  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open && closable) onClose();
      }}
    >
      <DialogContent className={className} aria-label={title} closable={closable}>
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
        </DialogHeader>
        {children}
      </DialogContent>
    </Dialog>
  );
}
