import { useTranslation } from "react-i18next";

import type { Account } from "../../entities/account/model";
import { cn } from "../../shared/lib/utils";
import { Button } from "../../shared/ui/button";
import { Card } from "../../shared/ui/card";
import { StatusPill } from "../../shared/ui/status-pill";

interface AccountCardProps {
  account: Account;
  busy?: boolean;
  onSelect: () => void;
  onSignInAgain: () => void;
  onValidate: () => void;
  onRemove: () => void;
}

export function AccountCard({
  account,
  busy = false,
  onSelect,
  onSignInAgain,
  onValidate,
  onRemove,
}: AccountCardProps) {
  const { t } = useTranslation();
  const needsAuth = account.status === "expired" || account.status === "needs_reauth";

  return (
    <Card
      className={cn("accountCard flex flex-col gap-4 p-5", account.isDefault && "border-accent/40")}
    >
      <div className="flex min-w-0 items-center gap-4">
        <div
          aria-hidden="true"
          className="grid h-12 w-12 shrink-0 place-items-center rounded-full bg-accent-muted font-display text-xl text-accent"
        >
          {account.displayName.slice(0, 1).toUpperCase()}
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex min-w-0 items-center gap-2">
            <strong className="min-w-0 truncate text-[15px] font-semibold text-text-primary">
              {account.displayName}
            </strong>
            {account.isDefault && (
              <span className="shrink-0 rounded-full border border-border-default bg-accent-muted px-2.5 py-0.5 text-[11px] font-semibold text-accent">
                {t("selected_status")}
              </span>
            )}
          </div>
          <small className="mt-0.5 block truncate text-[13px] text-text-muted">
            {account.email}
          </small>
          <div className="mt-2">
            <StatusPill status={account.status} />
          </div>
        </div>
      </div>
      <div className="flex flex-wrap items-center justify-end gap-2 border-t border-border-subtle pt-4">
        {!account.isDefault && (
          <Button variant="secondary" disabled={busy} onClick={onSelect}>
            {t("select")}
          </Button>
        )}
        {needsAuth ? (
          <Button variant="secondary" disabled={busy} onClick={onSignInAgain}>
            {t("sign_in_again")}
          </Button>
        ) : (
          <Button variant="ghost" disabled={busy} onClick={onValidate}>
            {t("validate")}
          </Button>
        )}
        <Button variant="danger" disabled={busy} onClick={onRemove}>
          {t("remove_from_launcher")}
        </Button>
      </div>
    </Card>
  );
}
