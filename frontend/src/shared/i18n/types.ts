import en from "./locales/en.json";

export type TranslationResources = typeof en;
export type TranslationKey = Exclude<keyof TranslationResources, "_glossary">;
