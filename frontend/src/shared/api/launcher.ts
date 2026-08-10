import { call } from "./bridge";
import type { LaunchValidation } from "./types";

export const launcherApi = {
  validate: (instanceId: string, accountId?: string) =>
    call<LaunchValidation>("LaunchController", "ValidateLaunch", {
      instanceId,
      accountId,
    }),
  launch: (instanceId: string, accountId?: string) =>
    call("LaunchController", "LaunchInstance", { instanceId, accountId }),
  launchServer: (instanceId: string, address: string, accountId?: string) =>
    call("LaunchController", "LaunchServer", { instanceId, address, accountId }),
  stop: (instanceId: string) => call<void>("LaunchController", "StopInstance", instanceId),
  running: () => call<string[]>("LaunchController", "GetRunningInstances"),
};
