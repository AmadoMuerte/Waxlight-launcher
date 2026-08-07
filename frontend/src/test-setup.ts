import { beforeEach } from "vitest";

import i18n from "./shared/i18n";

if (typeof HTMLElement !== "undefined") {
  Object.defineProperties(HTMLElement.prototype, {
    hasPointerCapture: { value: () => false },
    setPointerCapture: { value: () => {} },
    releasePointerCapture: { value: () => {} },
    scrollIntoView: { value: () => {} },
  });
}

beforeEach(async () => {
  await i18n.changeLanguage("en");
});

export { i18n };
