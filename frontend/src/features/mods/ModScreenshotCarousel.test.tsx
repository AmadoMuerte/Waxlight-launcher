// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

import i18n from "../../shared/i18n";
import { ModScreenshotCarousel } from "./ModScreenshotCarousel";

const screenshots = [
  { url: "https://cdn.example.test/one.png", caption: "First view" },
  { url: "https://cdn.example.test/two.png", caption: "Second view" },
];

beforeAll(() => i18n.changeLanguage("en"));
afterEach(cleanup);

describe("ModScreenshotCarousel", () => {
  it("navigates screenshots and opens the selected image", () => {
    const onOpen = vi.fn();
    render(<ModScreenshotCarousel screenshots={screenshots} modName="Test Mod" onOpen={onOpen} />);

    expect(screen.getByRole("img", { name: "First view" }).getAttribute("src")).toBe(
      screenshots[0].url,
    );

    fireEvent.click(screen.getByRole("button", { name: "Next screenshot" }));
    expect(screen.getByRole("img", { name: "Second view" }).getAttribute("src")).toBe(
      screenshots[1].url,
    );
    expect(screen.getByRole("button", { name: "Second view" }).getAttribute("aria-pressed")).toBe(
      "true",
    );

    fireEvent.click(screen.getByRole("button", { name: /Second view.*Open/ }));
    expect(onOpen).toHaveBeenCalledWith(1);
  });
});
