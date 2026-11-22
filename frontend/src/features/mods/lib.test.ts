import { describe, expect, it } from "bun:test";

import { formatGameVersions, parseModDBModURL } from "./lib";

describe("parseModDBModURL", () => {
  it("extracts the mod id from a ModDB mod page URL", () => {
    expect(parseModDBModURL("https://mods.vintagestory.at/show/mod/44133")).toBe("44133");
  });

  it("tolerates a trailing slash and query parameters", () => {
    expect(parseModDBModURL("https://mods.vintagestory.at/show/mod/44133/?tab=files")).toBe(
      "44133",
    );
  });

  it("extracts the slug from a short ModDB mod URL", () => {
    expect(parseModDBModURL("https://mods.vintagestory.at/configlib")).toBe("configlib");
    expect(parseModDBModURL("https://mods.vintagestory.at/xskills_fork/")).toBe("xskills_fork");
  });

  it("rejects other ModDB pages", () => {
    expect(parseModDBModURL("https://mods.vintagestory.at/list/mod")).toBeUndefined();
    expect(parseModDBModURL("https://mods.vintagestory.at/list")).toBeUndefined();
    expect(parseModDBModURL("https://mods.vintagestory.at/login")).toBeUndefined();
    expect(parseModDBModURL("https://mods.vintagestory.at/")).toBeUndefined();
    expect(parseModDBModURL("https://mods.vintagestory.at/show/mod/abc")).toBeUndefined();
  });

  it("rejects other hosts and non-https URLs", () => {
    expect(parseModDBModURL("https://www.vintagestory.at/show/mod/44133")).toBeUndefined();
    expect(parseModDBModURL("http://mods.vintagestory.at/show/mod/44133")).toBeUndefined();
    expect(parseModDBModURL("not a url")).toBeUndefined();
  });
});

describe("formatGameVersions", () => {
  it("returns an empty string for an empty list", () => {
    expect(formatGameVersions([])).toBe("");
  });

  it("keeps the full list when it fits within the limit", () => {
    expect(formatGameVersions(["1.19.8", "1.19.7", "1.19.6"])).toBe("1.19.8, 1.19.7, 1.19.6");
  });

  it("truncates a long list and counts the hidden versions", () => {
    expect(formatGameVersions(["1.20.0", "1.19.8", "1.19.7", "1.19.6", "1.19.5"])).toBe(
      "1.20.0, 1.19.8, 1.19.7, 1.19.6… +1",
    );
  });

  it("honors a custom limit", () => {
    expect(formatGameVersions(["1.20.0", "1.19.8", "1.19.7"], 2)).toBe("1.20.0, 1.19.8… +1");
  });
});
