const modIdPattern = /^[a-z0-9](?:[a-z0-9-]{0,62}[a-z0-9])?$/;

export const waxlightLinksBaseURL = "https://waxlight.by";

export function isValidWaxlightModID(value: unknown): value is string {
  return typeof value === "string" && modIdPattern.test(value);
}

export function modShareURL(modId: string): string | undefined {
  if (!isValidWaxlightModID(modId)) return undefined;
  return `${waxlightLinksBaseURL}/mod/${encodeURIComponent(modId)}`;
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
    return rawIPv6 ? address.toLowerCase() : parsed.host || undefined;
  } catch {
    return undefined;
  }
}

export function serverShareURL(address: string): string | undefined {
  const normalized = normalizeServerAddress(address);
  if (!normalized) return undefined;
  return `${waxlightLinksBaseURL}/server/${encodeURIComponent(normalized)}`;
}

export function deepLinkPath(target: unknown): string | undefined {
  if (!target || typeof target !== "object") {
    return undefined;
  }
  const type = Reflect.get(target, "type");
  const modId = Reflect.get(target, "modId");
  if (type === "mod" && isValidWaxlightModID(modId)) return `/mods/${encodeURIComponent(modId)}`;
  if (type === "server" && normalizeServerAddress(Reflect.get(target, "address")))
    return "/servers";
  return undefined;
}
