import type { PublicServer } from "../../shared/api/types";

export function canRequestServerLaunch(server: PublicServer): boolean {
  return server.joinable || (server.accessRestricted && !server.address);
}

export function normalizeServerAddress(value: unknown): string | undefined {
  if (typeof value !== "string") return undefined;
  const address = value.trim();
  if (!address || /[\s/\\?#@]/.test(address)) return undefined;

  try {
    const rawIPv6 =
      address.includes(":") && !address.startsWith("[") && address.split(":").length > 2;
    const authority = rawIPv6 ? `[${address}]` : address;
    const parsed = new URL(`vintagestoryjoin://${authority}`);
    if (
      parsed.username ||
      parsed.password ||
      (parsed.pathname && parsed.pathname !== "/") ||
      parsed.search ||
      parsed.hash
    ) {
      return undefined;
    }
    return rawIPv6 ? address.toLowerCase() : parsed.host.toLowerCase() || undefined;
  } catch {
    return undefined;
  }
}

export function serverKey(server: { address: string; name: string }): string {
  return normalizeServerAddress(server.address) ?? `${server.address}\u0000${server.name}`;
}
