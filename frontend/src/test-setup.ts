/// <reference types="bun" />

import { beforeEach } from "bun:test";

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
// "parameter 1 is not of type 'Event'". Adopt the jsdom realm's constructors
// unconditionally: the preload only copies window properties that are missing
// from globalThis, so Bun's own Event/ErrorEvent would otherwise win.
if (typeof window !== "undefined") {
  const eventCtors = [
    "Event",
    "CustomEvent",
    "ErrorEvent",
    "PromiseRejectionEvent",
    "MouseEvent",
    "PointerEvent",
    "KeyboardEvent",
    "FocusEvent",
    "InputEvent",
    "UIEvent",
  ] as const;
  for (const name of eventCtors) {
    const ctor = Reflect.get(window, name);
    if (ctor) Reflect.set(globalThis, name, ctor);
  }
}

beforeEach(async () => {
  await i18n.changeLanguage("en");
});

export { i18n };
