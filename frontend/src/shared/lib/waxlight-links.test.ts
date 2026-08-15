import { describe, expect, it } from "vitest";

import { deepLinkPath, modShareURL } from "./waxlight-links";

describe("Waxlight Links", () => {
  it("maps a validated mod target to the existing details route", () => {
    expect(deepLinkPath({ type: "mod", modId: "optimum" })).toBe("/mods/optimum");
  });

  it("does not navigate unknown or invalid targets", () => {
    expect(deepLinkPath({ type: "server", modId: "optimum" })).toBeUndefined();
    expect(deepLinkPath({ type: "mod", modId: "UPPERCASE" })).toBeUndefined();
  });

  it("builds the public share URL", () => {
    expect(modShareURL("optimum")).toBe("https://waxlight.by/mod/optimum");
  });
});
