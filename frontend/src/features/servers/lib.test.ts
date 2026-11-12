import { describe, expect, it } from "bun:test";

import { normalizeServerAddress, serverKey } from "./lib";

describe("normalizeServerAddress", () => {
  it("normalizes host case, ports, and raw IPv6 addresses", () => {
    expect(normalizeServerAddress("Play.Example.com:42420")).toBe("play.example.com:42420");
    expect(normalizeServerAddress("2001:DB8::1")).toBe("2001:db8::1");
    expect(normalizeServerAddress("[2001:DB8::1]:42420")).toBe("[2001:db8::1]:42420");
  });

  it("rejects unsafe or malformed addresses", () => {
    expect(normalizeServerAddress("../../etc/passwd")).toBeUndefined();
    expect(normalizeServerAddress("play.example.com/path")).toBeUndefined();
    expect(normalizeServerAddress("javascript:alert(1)")).toBeUndefined();
  });

  it("falls back to the raw address and name for unparseable entries", () => {
    expect(serverKey({ address: "not a host", name: "Broken" })).toBe("not a host\u0000Broken");
  });
});
