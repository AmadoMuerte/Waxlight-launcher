import { CircleAlert } from "lucide-react";
import type { ComponentProps } from "react";

import { EmptyState } from "./empty";

export function ErrorState({ icon, ...props }: ComponentProps<typeof EmptyState>) {
  return (
    <EmptyState
      role="alert"
      className="errorState"
      icon={icon ?? <CircleAlert size={30} aria-hidden="true" />}
      {...props}
    />
  );
}
