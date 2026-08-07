import type { FormEvent, ReactNode } from "react";

export function SubmitForm({
  onSubmit,
  children,
  className = "",
  noValidate = false,
}: {
  onSubmit: () => Promise<void>;
  children: ReactNode;
  className?: string;
  noValidate?: boolean;
}) {
  async function handleSubmit(event: FormEvent) {
    event.preventDefault();
    await onSubmit();
  }

  return (
    <form className={className} noValidate={noValidate} onSubmit={handleSubmit}>
      {children}
    </form>
  );
}
