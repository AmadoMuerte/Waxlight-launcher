import type { ReactNode } from "react";

import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogTitle,
} from "../../shared/ui/dialog";

// ChangesDialog is the shared dialog shell of the Last Known Good feature. It
// renders the same title, body and footer structure in every dialog that
// presents configuration changes, so the recovery prompt and the informational
// changes dialog stay visually identical. The caller supplies the content:
// title, optional description, body children and footer actions.
export function ChangesDialog({
  open,
  onClose,
  icon,
  iconVariant = "muted",
  title,
  description,
  children,
  footer,
}: {
  open: boolean;
  onClose: () => void;
  icon?: ReactNode;
  iconVariant?: "warning" | "muted";
  title: string;
  description?: string;
  children: ReactNode;
  footer: ReactNode;
}) {
  return (
    <Dialog
      open={open}
      onOpenChange={(nextOpen) => {
        if (!nextOpen) {
          onClose();
        }
      }}
    >
      <DialogContent className="recoveryDialog">
        <DialogTitle className="recoveryDialogTitle">
          {icon && (
            <span className={`recoveryDialogTitleIcon ${iconVariant}`} aria-hidden="true">
              {icon}
            </span>
          )}
          <span>{title}</span>
        </DialogTitle>
        {description && (
          <DialogDescription className="recoveryDialogDescription">{description}</DialogDescription>
        )}
        <div className="recoveryBody">{children}</div>
        <DialogFooter className="recoveryDialogFooter">{footer}</DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
