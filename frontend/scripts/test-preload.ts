import fs from "node:fs";
import { dirname, join } from "node:path";

import { JSDOM } from "jsdom";

// vitest always exposes import.meta.env with DEV: true; bun does not define
// DEV at all. App sources are rewritten at load time (plugin below) to read
// this shim instead, keeping dev-gated code (UI Lab route/nav) testable.
Object.assign(globalThis, {
  viteEnv: { DEV: true, PROD: false, MODE: "test", SSR: false },
});

const ENV = "globalThis.viteEnv";

// Bun 1.4.4's onLoad must return an object; returning undefined is an error,
// so callers below always pass the source through when there is nothing to
// rewrite. "tsx" for JSX files, "ts" otherwise.
function loaderFor(path: string): "ts" | "tsx" {
  return path.endsWith(".tsx") ? "tsx" : "ts";
}

function rewriteEnv(source: string): string {
  return source.replaceAll("import.meta.env", ENV);
}

Bun.plugin({
  name: "vite-compat",
  setup(build) {
    // The single eager import.meta.glob call in the i18n module becomes
    // static JSON imports: bun test has no Vite transforms. The type argument
    // may contain ">>" (Record<string, string>), so stop at the first "("
    // instead of trying to balance angle brackets.
    build.onLoad({ filter: /src\/shared\/i18n\/index\.ts$/ }, async (args) => {
      const source = await Bun.file(args.path).text();
      const globbed = /import\.meta\.glob[^(]*\([^)]*\)/;
      if (!globbed.test(source)) {
        return { contents: rewriteEnv(source), loader: loaderFor(args.path) };
      }
      const locales = fs
        .readdirSync(join(dirname(args.path), "locales"))
        .filter((file) => file.endsWith(".json"))
        .toSorted();
      const imports = locales
        .map((file, index) => `import __locale_${index} from "./locales/${file}";`)
        .join("\n");
      const entries = locales
        .map((file, index) => `"./locales/${file}": __locale_${index}`)
        .join(",\n");
      return {
        contents: rewriteEnv(`${imports}\n${source.replace(globbed, `{\n  ${entries},\n}`)}`),
        loader: loaderFor(args.path),
      };
    });

    // Every other app source reads the env shim instead of import.meta.env.
    build.onLoad({ filter: /frontend\/src\/.*\.[jt]sx?$/ }, async (args) => {
      const source = await Bun.file(args.path).text();
      const contents =
        args.path.includes("/wailsjs/") || !source.includes("import.meta.env")
          ? source
          : rewriteEnv(source);
      return { contents, loader: loaderFor(args.path) };
    });
  },
});

// Single-realm jsdom preload: install one JSDOM instance globally before any
// test module loads, so App/test code and jsdom share the same realm
// (Event/CustomEvent instanceof checks match).
const dom = new JSDOM("<!DOCTYPE html>", { url: "http://localhost/", pretendToBeVisual: true });
const { window } = dom;

for (const key of Object.getOwnPropertyNames(window)) {
  if (key in globalThis) continue;
  try {
    // ponytail: direct assignment instead of defineProperty; a few jsdom
    // accessors throw on access and are skipped — upgrade to explicit
    // per-property copies if a test needs one of them.
    Reflect.set(globalThis, key, Reflect.get(window, key));
  } catch {
    // skip getters that throw outside a real browser
  }
}

// Under vitest+jsdom `globalThis === window`, so tests that inject a global
// (e.g. `globalThis.runtime = { EventsOnMultiple }`) are visible to Wails
// runtime code that reads `window.runtime`. Under bun the jsdom window is a
// detached realm, so make `window` a Proxy over it that falls back to
// globalThis for anything the jsdom target lacks. Property reads hit the jsdom
// realm first (document, location, addEventListener, Event ...), unknown reads
// and `in` checks defer to globalThis, and writes stay on the jsdom window.
const selfKeys = new Set(["window", "self", "globalThis", "top", "parent", "frames"]);
const windowProxy = new Proxy(window, {
  get(target, key) {
    if (typeof key === "string" && selfKeys.has(key)) return windowProxy;
    if (Reflect.has(target, key)) return Reflect.get(target, key, target);
    return Reflect.get(globalThis, key);
  },
  has(target, key) {
    return Reflect.has(target, key) || key in globalThis;
  },
  set(target, key, value) {
    return Reflect.set(target, key, value, target);
  },
});

Object.assign(globalThis, { window: windowProxy, document: window.document });

await import("../src/test-setup.ts");
