import config from "../../../languages.json";

export type LanguageCode = string;

export const supportedLanguages = config.languages;
export const defaultLanguage = config.defaultLanguage;

const languageAliases = new Map(
  supportedLanguages.flatMap(({ code, aliases = [] }) =>
    aliases.map((alias) => [alias, code] as const),
  ),
);
const languageCodes = new Set(supportedLanguages.map(({ code }) => code));

export function isSupportedLanguage(value: string | null | undefined): value is LanguageCode {
  return value !== undefined && value !== null && languageCodes.has(value);
}

export function normalizeLanguage(value: string | null | undefined): LanguageCode {
  if (!value) return defaultLanguage;
  try {
    const code = new Intl.Locale(value.trim().replaceAll("_", "-")).language;
    return languageCodes.has(code) ? code : (languageAliases.get(code) ?? defaultLanguage);
  } catch {
    return defaultLanguage;
  }
}
