import i18nModule from "i18next";
import { initReactI18next } from "react-i18next";

import {
  defaultLanguage,
  normalizeLanguage,
  supportedLanguages,
  type LanguageCode,
} from "./languages";

const localeModules = import.meta.glob<Record<string, string>>("./locales/*.json", {
  eager: true,
  import: "default",
});

const resources = Object.fromEntries(
  supportedLanguages.map(({ code }) => {
    const translation = localeModules[`./locales/${code}.json`];
    if (!translation) {
      throw new Error(`Missing locale resource for language: ${code}`);
    }
    return [code, { translation }];
  }),
);

type MissingKeyReporter = (languages: readonly string[], namespace: string, key: string) => void;

// missingKeyReporter is late-bound so this module never depends on the
// logging pipeline at import time. The app entry wires it; the launcher log
// then receives missing-translation warnings instead of the browser console.
let missingKeyReporter: MissingKeyReporter = () => {};

export function setMissingKeyReporter(reporter: MissingKeyReporter) {
  missingKeyReporter = reporter;
}

void i18nModule.use(initReactI18next).init({
  resources,

  lng: defaultLanguage,
  fallbackLng: defaultLanguage,

  supportedLngs: supportedLanguages.map((language) => language.code),
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
      missingKeyReporter(languages, namespace, key);
    }
  },
});

export async function changeAppLanguage(
  language: string | null | undefined,
): Promise<LanguageCode> {
  const normalized = normalizeLanguage(language);

  if (i18nModule.resolvedLanguage !== normalized) {
    await i18nModule.changeLanguage(normalized);
  }

  document.documentElement.lang = normalized;

  return normalized;
}

export default i18nModule;
