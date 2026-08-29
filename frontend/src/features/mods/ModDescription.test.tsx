// @vitest-environment jsdom

import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { ModDescription } from "./ModDescription";

describe("ModDescription", () => {
  it("renders supported ModDB HTML and opens HTTPS links through the app", () => {
    const onOpenExternal = vi.fn();

    render(
      <ModDescription
        description={
          '<h2>Installation</h2><p>Use <strong>this mod</strong>.</p><ul><li>First step</li></ul><a href="https://example.com/docs">Read more</a><img src="https://example.com/cover.png" alt="Cover">'
        }
        fallback="Summary"
        onOpenExternal={onOpenExternal}
      />,
    );

    expect(screen.getByRole("heading", { name: "Installation" })).not.toBeNull();
    expect(screen.getByText("this mod").tagName).toBe("STRONG");
    expect(screen.getByText("First step").closest("li")).not.toBeNull();
    expect(screen.getByRole("img", { name: "Cover" }).getAttribute("src")).toBe(
      "https://example.com/cover.png",
    );

    fireEvent.click(screen.getByRole("link", { name: "Read more" }));
    expect(onOpenExternal).toHaveBeenCalledWith("https://example.com/docs");
  });

  it("removes executable content and non-HTTPS resources", () => {
    const { container } = render(
      <ModDescription
        description={
          '<script>alert("xss")</script><a href="javascript:alert(1)">Unsafe link</a><img src="javascript:alert(1)" onerror="alert(1)"><p onclick="alert(1)">Safe text</p>'
        }
        fallback="Summary"
        onOpenExternal={vi.fn()}
      />,
    );

    expect(container.querySelector("script")).toBeNull();
    expect(screen.queryByRole("link", { name: "Unsafe link" })).toBeNull();
    expect(container.querySelector("img")).toBeNull();
    expect(container.querySelector("[onclick]")).toBeNull();
    expect(screen.getByText("Safe text")).not.toBeNull();
  });
});
