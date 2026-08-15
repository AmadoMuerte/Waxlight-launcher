import { describe, expect, it } from "vitest";

import {
  deepLinkPath,
  modShareURL,
  normalizeServerAddress,
  serverShareURL,
} from "./waxlight-links";

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

  it("maps a server target to the existing servers route", () => {
    expect(deepLinkPath({ type: "server", address: "play.example.com:42420" })).toBe("/servers");
  });

  it("round-trips encoded server addresses through the public share URL", () => {
    const url = serverShareURL("[2001:DB8::1]:42420");
    expect(url).toBe("https://waxlight.by/server/%5B2001%3Adb8%3A%3A1%5D%3A42420");
    expect(
      normalizeServerAddress(decodeURIComponent(new URL(url!).pathname.slice("/server/".length))),
    ).toBe("[2001:db8::1]:42420");
    expect(normalizeServerAddress("2001:DB8::1")).toBe("2001:db8::1");
  });
});
