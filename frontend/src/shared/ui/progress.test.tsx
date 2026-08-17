// @vitest-environment jsdom

import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, expect, it } from "vitest";

import { Progress } from "./progress";

function indicator() {
  return document.querySelector(".progressIndicator") as HTMLElement | null;
}

afterEach(cleanup);

it("exposes determinate progress with semantic attributes", () => {
  render(<Progress value={63} />);

  const bar = screen.getByRole("progressbar");
  expect(bar.getAttribute("aria-valuemin")).toBe("0");
  expect(bar.getAttribute("aria-valuemax")).toBe("100");
  expect(bar.getAttribute("aria-valuenow")).toBe("63");
  expect(bar.getAttribute("aria-valuetext")).toBe("63%");
  expect(indicator()?.style.width).toBe("63%");
});

it("supports a custom max scale", () => {
  render(<Progress max={1} value={0.25} />);

  const bar = screen.getByRole("progressbar");
  expect(bar.getAttribute("aria-valuemax")).toBe("1");
  expect(bar.getAttribute("aria-valuenow")).toBe("0.25");
  expect(indicator()?.style.width).toBe("25%");
});

it("clamps out-of-range values instead of overflowing the bar", () => {
  const { rerender } = render(<Progress value={150} />);
  expect(screen.getByRole("progressbar").getAttribute("aria-valuenow")).toBe("100");
  expect(indicator()?.style.width).toBe("100%");

  rerender(<Progress value={-20} />);
  expect(screen.getByRole("progressbar").getAttribute("aria-valuenow")).toBe("0");
  expect(indicator()?.style.width).toBe("0%");
});

it("renders indeterminate progress without a numeric value", () => {
  render(<Progress indeterminate />);

  const bar = screen.getByRole("progressbar");
  expect(bar.getAttribute("aria-valuenow")).toBeNull();
  expect(bar.getAttribute("data-state")).toBe("indeterminate");
  expect(document.querySelector(".progressIndicatorIndeterminate")).toBeTruthy();
});
