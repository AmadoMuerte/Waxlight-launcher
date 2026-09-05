import { useMutation, useQueryClient } from "@tanstack/react-query";
import { User } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { useSearchParams } from "react-router";

import { useToastStore } from "../../app/stores/toast";
import { accountsApi } from "../../entities/account/api";
import type { Account } from "../../entities/account/model";
import { useAccountsQuery } from "../../entities/account/queries";
import { useSettingsQuery } from "../../entities/settings/queries";
import { AccountCard } from "../../features/accounts/AccountCard";
import { LoginModal, authErrorMessages, isValidEmail } from "../../features/auth/LoginModal";
import { errorMessage } from "../../shared/api/bridge";
import { ACCOUNTS_QUERY_KEY } from "../../shared/api/keys";
import { Button } from "../../shared/ui/button";
import { ConfirmDialog } from "../../shared/ui/confirm-dialog";
import { Empty } from "../../shared/ui/empty";
import { Page, PageContent } from "../../shared/ui/page";
import { PageHeader } from "../../shared/ui/page-header";

export { authErrorMessages, isValidEmail };

export function AccountsPage() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const notify = useToastStore((state) => state.notify);
  const { data: accounts = [] } = useAccountsQuery();
  const { data: settings } = useSettingsQuery();
  const [loginAccount, setLoginAccount] = useState<Account | null>();
  const [searchParams, setSearchParams] = useSearchParams();
  const addRequestConsumed = useRef(false);
  const [removeConfirm, setRemoveConfirm] = useState<{
    open: boolean;
    title: string;
    message?: string;
    onConfirm: () => void;
  }>({ open: false, title: "", onConfirm: () => {} });

  useEffect(() => {
    if (searchParams.get("add") !== "1") {
      addRequestConsumed.current = false;
      return;
    }
    if (addRequestConsumed.current) return;
    addRequestConsumed.current = true;
    const nextParams = new URLSearchParams(searchParams);
    nextParams.delete("add");
    setSearchParams(nextParams, { replace: true });
    setLoginAccount(null);
  }, [searchParams, setSearchParams]);

  const selectMutation = useMutation({
    mutationFn: (accountID: string) => accountsApi.setDefault(accountID),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ACCOUNTS_QUERY_KEY });
      notify(t("account_selected"));
    },
    onError: (error) => {
      notify(errorMessage(error), "error");
    },
  });

  const removeMutation = useMutation({
    mutationFn: (accountID: string) => accountsApi.remove(accountID),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ACCOUNTS_QUERY_KEY });
      notify(t("account_removed"));
    },
    onError: (error) => {
      notify(errorMessage(error), "error");
    },
  });

  const validateMutation = useMutation({
    mutationFn: (accountID: string) => accountsApi.validate(accountID),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ACCOUNTS_QUERY_KEY });
      notify(t("account_session_valid"));
    },
    onError: async (error) => {
      await queryClient.invalidateQueries({ queryKey: ACCOUNTS_QUERY_KEY });
      notify(errorMessage(error), "error");
    },
  });

  const busyAccountID = selectMutation.isPending
    ? selectMutation.variables
    : removeMutation.isPending
      ? removeMutation.variables
      : validateMutation.isPending
        ? validateMutation.variables
        : undefined;

  function selectAccount(accountID: string) {
    selectMutation.mutate(accountID);
  }

  function removeAccount(account: Account) {
    if (settings?.confirmDeletion === false) {
      removeMutation.mutate(account.id);
      return;
    }
    setRemoveConfirm({
      open: true,
      title: t("remove_account_confirmation", { name: account.displayName }),
      message: t("remove_account_confirmation_message"),
      onConfirm: () => {
        removeMutation.mutate(account.id);
      },
    });
  }

  function validateAccount(account: Account) {
    validateMutation.mutate(account.id);
  }

  return (
    <Page>
      <PageHeader
        eyebrow="Vintage Story"
        title={t("accounts")}
        description={t("accounts_description")}
        actions={<Button onClick={() => setLoginAccount(null)}>{t("add_account")}</Button>}
      />

      <PageContent>
        <div className="notice mb-5">
          <b>{t("credentials_never_stored")}</b>
          <span>{t("session_credentials_notice")}</span>
        </div>

        {accounts.length === 0 ? (
          <Empty
            icon={<User size={24} aria-hidden="true" />}
            title={t("no_saved_accounts")}
            description={t("official_account_description")}
            action={<Button onClick={() => setLoginAccount(null)}>{t("sign_in")}</Button>}
          />
        ) : (
          <div className="grid grid-cols-[repeat(auto-fill,minmax(min(300px,100%),1fr))] gap-4">
            {accounts.map((account) => (
              <AccountCard
                key={account.id}
                account={account}
                busy={busyAccountID === account.id}
                onSelect={() => selectAccount(account.id)}
                onSignInAgain={() => setLoginAccount(account)}
                onValidate={() => validateAccount(account)}
                onRemove={() => removeAccount(account)}
              />
            ))}
          </div>
        )}
      </PageContent>

      {loginAccount !== undefined && (
        <LoginModal
          account={loginAccount ?? undefined}
          onClose={() => setLoginAccount(undefined)}
          onDone={async () => {
            setLoginAccount(undefined);
            await queryClient.invalidateQueries({ queryKey: ACCOUNTS_QUERY_KEY });
            notify(t("signed_in_successfully"));
          }}
        />
      )}

      <ConfirmDialog
        open={removeConfirm.open}
        title={removeConfirm.title}
        message={removeConfirm.message}
        destructive
        onConfirm={() => {
          setRemoveConfirm((s) => ({ ...s, open: false }));
          removeConfirm.onConfirm();
        }}
        onCancel={() => setRemoveConfirm((s) => ({ ...s, open: false }))}
      />
    </Page>
  );
}
