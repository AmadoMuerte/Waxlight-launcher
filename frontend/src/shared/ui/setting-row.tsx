import type { HTMLAttributes, ReactNode } from "react";

import { cn } from "@/shared/lib/utils";

type SettingRowProps = HTMLAttributes<HTMLDivElement> & {
  title?: ReactNode;
  description?: ReactNode;
  warning?: ReactNode;
  column?: boolean;
  disabled?: boolean;
};

export function SettingRow({
  title,
  description,
  warning,
  column = false,
  disabled = false,
  className,
  children,
  ...props
}: SettingRowProps) {
  return (
    <div
      className={cn(
        "settingRow",
        column && "settingRowColumn",
        disabled && "settingRowDisabled",
        className,
      )}
      {...props}
    >
      {(title || description || warning) && (
        <div className="settingRowText">
          {title && <span className="settingRowTitle">{title}</span>}
          {description && <small className="settingRowDescription">{description}</small>}
          {warning && <small className="settingRowWarning">{warning}</small>}
        </div>
      )}
      <div className="settingRowControl">{children}</div>
    </div>
  );
}
