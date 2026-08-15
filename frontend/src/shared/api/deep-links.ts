import { call } from "./bridge";

export interface DeepLinkTarget {
  type: string;
  modId?: string;
  address?: string;
}

export const deepLinksApi = {
  consumePending: () => call<DeepLinkTarget[]>("DeepLinkController", "ConsumePendingDeepLinks"),
};
