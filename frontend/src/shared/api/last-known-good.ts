import { call } from "./bridge";
import type { LastKnownGood } from "./types";

export const lastKnownGoodApi = {
  get: (instanceId: string) =>
    call<LastKnownGood>("LastKnownGoodController", "GetInstanceLastKnownGood", instanceId),
};
