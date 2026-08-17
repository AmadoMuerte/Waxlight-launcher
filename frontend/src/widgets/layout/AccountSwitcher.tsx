import { useQueryClient } from "@tanstack/react-query";
import { Check } from "lucide-react";
import { useTranslation } from "react-i18next";
import { NavLink } from "react-router";

import { useToastStore } from "../../app/stores/toast";
import { useAccountsQuery } from "../../entities/account/queries";
import { accountsApi } from "../../shared/api/accounts";
import { errorMessage } from "../../shared/api/bridge";
import { ACCOUNTS_QUERY_KEY } from "../../shared/api/keys";
import type { Account } from "../../shared/api/types";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "../../shared/ui/dropdown-menu";

export function AccountSwitcher() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const notify = useToastStore((state) => state.notify);
  const { data: accounts = [] } = useAccountsQuery();
  const defaultAccount = accounts.find((account) => account.isDefault);

  async function handleSelect(account: Account) {
    try {
      await accountsApi.setDefault(account.id);
      await queryClient.invalidateQueries({ queryKey: ACCOUNTS_QUERY_KEY });
    } catch (error) {
      notify(errorMessage(error), "error");
    }
  }

  return (
    <DropdownMenu>
      <DropdownMenuTrigger className="sidebarAccountTrigger">
        <span className="miniAvatar">
          {(defaultAccount?.displayName ?? "?").slice(0, 1).toUpperCase()}
        </span>
        <span className="min-w-0 text-left">
          <small className="block overflow-hidden text-ellipsis whitespace-nowrap text-[10px] tracking-wider text-text-disabled uppercase">
            {t("account")}
          </small>
          <strong className="mt-0.5 block overflow-hidden text-ellipsis whitespace-nowrap text-[12px] text-text-primary">
            {defaultAccount?.displayName ?? t("account_not_selected")}
          </strong>
        </span>
      </DropdownMenuTrigger>

      <DropdownMenuContent side="bottom" align="start" sideOffset={7} className="w-[240px]">
        {accounts.map((account) => (
          <DropdownMenuItem key={account.id} onSelect={() => void handleSelect(account)}>
            <span className="grid w-3.5 shrink-0 place-items-center text-accent">
              {account.isDefault && <Check size={13} strokeWidth={3} aria-hidden="true" />}
            </span>
            <span className="min-w-0">
              <strong className="block overflow-hidden text-ellipsis whitespace-nowrap text-[12px] text-text-primary">
                {account.displayName}
              </strong>
              <small className="mt-0.5 block overflow-hidden text-ellipsis whitespace-nowrap text-[11px] text-text-muted">
                {account.email}
              </small>
            </span>
          </DropdownMenuItem>
        ))}

        <DropdownMenuSeparator />

        <DropdownMenuItem asChild>
          <NavLink to="/accounts?add=1">{t("add_account")}</NavLink>
        </DropdownMenuItem>
        <DropdownMenuItem asChild>
          <NavLink to="/accounts">{t("manage_accounts")}</NavLink>
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
