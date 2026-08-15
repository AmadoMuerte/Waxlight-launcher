const modIdPattern = /^[a-z0-9](?:[a-z0-9-]{0,62}[a-z0-9])?$/;

export const waxlightLinksBaseURL = "https://waxlight.by";

export function isValidWaxlightModID(value: unknown): value is string {
  return typeof value === "string" && modIdPattern.test(value);
}

export function modShareURL(modId: string): string | undefined {
  if (!isValidWaxlightModID(modId)) return undefined;
  return `${waxlightLinksBaseURL}/mod/${encodeURIComponent(modId)}`;
}

export function deepLinkPath(target: unknown): string | undefined {
  if (!target || typeof target !== "object") {
    return undefined;
  }
  const type = Reflect.get(target, "type");
  const modId = Reflect.get(target, "modId");
  if (type !== "mod" || !isValidWaxlightModID(modId)) return undefined;
  return `/mods/${encodeURIComponent(modId)}`;
}
