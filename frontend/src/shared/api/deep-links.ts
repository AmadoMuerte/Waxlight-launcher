import { call } from "./bridge";

export interface DeepLinkTarget {
  type: string;
  modId: string;
}

export const deepLinksApi = {
  consumePending: () => call<DeepLinkTarget[]>("DeepLinkController", "ConsumePendingDeepLinks"),
};
