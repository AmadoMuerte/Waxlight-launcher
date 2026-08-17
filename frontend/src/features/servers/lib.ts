import type { PublicServer } from "../../shared/api/types";
import { normalizeServerAddress } from "../../shared/lib/waxlight-links";

export function canRequestServerLaunch(server: PublicServer): boolean {
  return server.joinable || (server.accessRestricted && !server.address);
}

export function serverKey(server: { address: string; name: string }): string {
  return normalizeServerAddress(server.address) ?? `${server.address}\u0000${server.name}`;
}
