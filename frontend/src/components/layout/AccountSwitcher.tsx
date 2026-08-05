import { useCallback } from "react";
import { useTranslation } from "react-i18next";
import { NavLink } from "react-router-dom";

import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";

import { accountsApi, type Account } from "../../shared/api";
import { errorMessage } from "../../shared/api/bridge";

type Notify = (message: string, type?: "ok" | "error") => void;

interface AccountSwitcherProps {
  accounts: Account[];
  refresh: () => Promise<void>;
  notify: Notify;
}

export function AccountSwitcher({ accounts, refresh, notify }: AccountSwitcherProps) {
  const { t } = useTranslation();
  const defaultAccount = accounts.find((account) => account.isDefault);

  const handleSelect = useCallback(
    async (accountId: string) => {
      try {
        await accountsApi.setDefault(accountId);
        await refresh();
      } catch (error) {
        notify(errorMessage(error), "error");
      }
    },
    [refresh, notify],
  );

  return (
    <DropdownMenu>
      <DropdownMenuTrigger className="mt-[5px] flex select-none items-center gap-2.5 rounded-[10px] border border-[#2e2a2d] bg-[#171619] px-2.5 py-2.5 outline-none transition-colors hover:border-[#4a4035] focus-visible:ring-[3px] focus-visible:ring-[var(--amber)]/15">
        <span className="miniAvatar">
          {(defaultAccount?.displayName ?? "?").slice(0, 1).toUpperCase()}
        </span>
        <span className="min-w-0 text-left">
          <small className="block overflow-hidden text-ellipsis whitespace-nowrap text-[10px] uppercase tracking-wider text-[#77716d]">
            {t("account")}
          </small>
          <strong className="mt-0.5 block overflow-hidden text-ellipsis whitespace-nowrap text-[12px] text-[#d9d2ca]">
            {defaultAccount?.displayName ?? t("account_not_selected")}
          </strong>
        </span>
      </DropdownMenuTrigger>

      <DropdownMenuContent side="bottom" align="start" sideOffset={7} className="w-[240px]">
        {accounts.map((account) => (
          <DropdownMenuItem key={account.id} onSelect={() => void handleSelect(account.id)}>
            <span className="flex-0-0 w-3 text-[var(--amber)]">{account.isDefault ? "✓" : ""}</span>
            <span className="min-w-0">
              <strong className="block overflow-hidden text-ellipsis whitespace-nowrap text-[12px] text-[#eee8e1]">
                {account.displayName}
              </strong>
              <small className="mt-0.5 block overflow-hidden text-ellipsis whitespace-nowrap text-[11px] text-[#716b68]">
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
