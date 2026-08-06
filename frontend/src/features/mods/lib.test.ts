// @vitest-environment jsdom

import { describe, expect, it } from "vitest";

import { formatGameVersions } from "./lib";

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
