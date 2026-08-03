import i18n from "i18next";
import { initReactI18next } from "react-i18next";

import en from "./locales/en.json";
import ru from "./locales/ru.json";
import {
  defaultLanguage,
  normalizeLanguage,
  supportedLanguages,
  type LanguageCode,
} from "./languages";

void i18n.use(initReactI18next).init({
  resources: {
    en: {
      translation: en,
    },
    ru: {
      translation: ru,
    },
  },

  lng: defaultLanguage,
  fallbackLng: defaultLanguage,

  supportedLngs: supportedLanguages.map(
    (language) => language.code,
  ),
  load: "languageOnly",
  cleanCode: true,
  nonExplicitSupportedLngs: true,

  interpolation: {
    escapeValue: false,
  },

  returnNull: false,
  returnEmptyString: false,

  react: {
    useSuspense: false,
  },

  debug: import.meta.env.DEV,
  saveMissing: import.meta.env.DEV,
  missingKeyHandler: (languages, namespace, key) => {
    if (import.meta.env.DEV) {
      console.warn(`[i18n] Missing translation: ${namespace}:${key}`, languages);
    }
  },
});

export async function changeAppLanguage(
  language: string | null | undefined,
): Promise<LanguageCode> {
  const normalized = normalizeLanguage(language);

  if (i18n.resolvedLanguage !== normalized) {
    await i18n.changeLanguage(normalized);
  }

  document.documentElement.lang = normalized;

  return normalized;
}

export default i18n;
