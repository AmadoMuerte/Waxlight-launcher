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

// Radix components construct events with the module-scope global. In jsdom the
// Node-provided globals create events whose prototype chain does not match the
// jsdom window, so dispatching them (e.g. the focus-scope unmount timer) throws
// "parameter 1 is not of type 'Event'". Align the globals with the jsdom realm.
if (typeof window !== "undefined") {
  globalThis.CustomEvent = window.CustomEvent;
  globalThis.Event = window.Event;
}

beforeEach(async () => {
  await i18n.changeLanguage("en");
});

export { i18n };
