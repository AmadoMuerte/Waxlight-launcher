import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { useSearchParams } from "react-router-dom";

import { useToastStore } from "../../app/stores/toast";
import { accountsApi } from "../../entities/account/api";
import type { Account } from "../../entities/account/model";
import { useAccountsQuery } from "../../entities/account/queries";
import { LoginModal, authErrorMessages, isValidEmail } from "../../features/auth/LoginModal";
import { errorMessage } from "../../shared/api/bridge";
import { ACCOUNTS_QUERY_KEY } from "../../shared/api/keys";
import { Button } from "../../shared/ui/button";
import { ConfirmDialog } from "../../shared/ui/confirm-dialog";
import { Empty } from "../../shared/ui/empty";
import { PageHeader } from "../../shared/ui/page-header";
import { StatusPill } from "../../shared/ui/status-pill";

export { authErrorMessages, isValidEmail };

export function AccountsPage() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const notify = useToastStore((state) => state.notify);
  const { data: accounts = [] } = useAccountsQuery();
  const [loginAccount, setLoginAccount] = useState<Account | null>();
  const [searchParams, setSearchParams] = useSearchParams();
  const [removeConfirm, setRemoveConfirm] = useState<{
    open: boolean;
    title: string;
    message?: string;
    onConfirm: () => void;
  }>({ open: false, title: "", onConfirm: () => {} });

  useEffect(() => {
    if (searchParams.get("add") === "1") {
      setLoginAccount(null);
      setSearchParams({}, { replace: true });
    }
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

  function selectAccount(accountID: string) {
    selectMutation.mutate(accountID);
  }

  function removeAccount(account: Account) {
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
    <>
      <PageHeader
        eyebrow="Vintage Story"
        title={t("accounts")}
        description={t("accounts_description")}
        action={<Button onClick={() => setLoginAccount(null)}>{t("add_account")}</Button>}
      />

      <div className="notice pageNotice">
        <b>{t("credentials_never_stored")}</b>
        <span>{t("session_credentials_notice")}</span>
      </div>

      {accounts.length === 0 ? (
        <Empty
          icon="♙"
          title={t("no_saved_accounts")}
          description={t("official_account_description")}
          action={<Button onClick={() => setLoginAccount(null)}>{t("sign_in")}</Button>}
        />
      ) : (
        <div className="accountGrid">
          {accounts.map((account) => (
            <article className="accountCard" key={account.id}>
              <div className="avatar">{account.displayName.slice(0, 1).toUpperCase()}</div>

              <div className="accountIdentity">
                <strong>{account.displayName}</strong>
                <small>{account.email}</small>
                <StatusPill status={account.status} />
              </div>

              <div className="accountActions">
                {account.isDefault ? (
                  <span className="defaultMark">{t("selected_status")}</span>
                ) : (
                  <Button variant="secondary" onClick={() => selectAccount(account.id)}>
                    {t("select")}
                  </Button>
                )}

                {account.status === "expired" || account.status === "needs_reauth" ? (
                  <Button variant="secondary" onClick={() => setLoginAccount(account)}>
                    {t("sign_in_again")}
                  </Button>
                ) : (
                  <Button variant="ghost" onClick={() => validateAccount(account)}>
                    {t("validate")}
                  </Button>
                )}

                <Button variant="ghost" onClick={() => removeAccount(account)}>
                  {t("remove_from_launcher")}
                </Button>
              </div>
            </article>
          ))}
        </div>
      )}

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
    </>
  );
}
