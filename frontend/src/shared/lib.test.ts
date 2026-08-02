import { describe, expect, it } from "vitest";

import { formatBytes, formatDuration } from "./lib";

describe("formatters", () => {
  it("formats durations", () => {
    expect(formatDuration(3_720)).toBe("1h 2m");
    expect(formatDuration(59)).toBe("0m");
  });

  it("formats byte sizes", () => {
    expect(formatBytes(1_024)).toBe("1.0 KB");
    expect(formatBytes(0)).toBe("0 B");
  });
});
