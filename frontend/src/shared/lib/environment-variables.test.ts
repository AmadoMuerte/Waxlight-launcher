import { describe, expect, it } from "bun:test";

import {
  environmentVariablesEqual,
  formatEnvironmentVariables,
  parseEnvironmentVariables,
} from "./environment-variables";

describe("environment variables", () => {
  it("round-trips sorted variables", () => {
    const values = { ZETA: "last", ALPHA: "first" };
    expect(formatEnvironmentVariables(values)).toBe("ALPHA=first\nZETA=last");
    expect(parseEnvironmentVariables(formatEnvironmentVariables(values))).toEqual({ values });
  });

  it("keeps spaces and equals signs in values", () => {
    expect(parseEnvironmentVariables("KEY=a=b c")).toEqual({ values: { KEY: "a=b c" } });
  });

  it("reports the invalid line", () => {
    expect(parseEnvironmentVariables("VALID=1\nINVALID")).toEqual({
      values: {},
      invalidLine: 2,
    });
  });

  it("compares variables independent of order", () => {
    expect(environmentVariablesEqual({ A: "1", B: "2" }, { B: "2", A: "1" })).toBe(true);
    expect(environmentVariablesEqual({ A: "1" }, { A: "2" })).toBe(false);
  });
});
