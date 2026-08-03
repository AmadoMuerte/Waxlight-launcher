export const supportedLanguages = [
  { code: "en", nativeName: "English" },
  { code: "ru", nativeName: "Русский" },
] as const;

export type LanguageCode = (typeof supportedLanguages)[number]["code"];
export const defaultLanguage: LanguageCode = "en";

export function isSupportedLanguage(
  value: string | null | undefined,
): value is LanguageCode {
  return supportedLanguages.some(({ code }) => code === value);
}

export function normalizeLanguage(
  value: string | null | undefined,
): LanguageCode {
  if (!value) return defaultLanguage;
  const normalized = value.trim().toLowerCase().split(/[-_]/)[0];
  return isSupportedLanguage(normalized) ? normalized : defaultLanguage;
}
