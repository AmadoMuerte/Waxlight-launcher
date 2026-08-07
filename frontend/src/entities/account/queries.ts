import { useQuery } from "@tanstack/react-query";

import { ACCOUNTS_QUERY_KEY } from "../../shared/api/keys";
import { accountsApi } from "./api";
import type { Account } from "./model";

interface QueryOptions {
  refetchInterval?: number | false;
}

export function useAccountsQuery(options?: QueryOptions) {
  return useQuery({
    queryKey: ACCOUNTS_QUERY_KEY,
    queryFn: accountsApi.list,
    ...options,
  });
}

export type { Account };
