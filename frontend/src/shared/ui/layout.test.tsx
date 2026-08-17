// @vitest-environment jsdom

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { expect, it, vi } from "vitest";

import { Button } from "./button";
import { EmptyState } from "./empty";
import { ErrorState } from "./error-state";
import { LoadingState } from "./loading-state";
import { PageHeader } from "./page-header";
import { SegmentedControl } from "./segmented-control";
import { Tabs } from "./tabs";

function TabsExample() {
  const [value, setValue] = useState("one");
  return (
    <Tabs
      label="Views"
      value={value}
      options={[
        { value: "one", label: "One" },
        { value: "two", label: "Two", disabled: true },
        { value: "three", label: "Three" },
      ]}
      onValueChange={setValue}
    />
  );
}

function SegmentedExample() {
  const [value, setValue] = useState("grid");
  return (
    <SegmentedControl
      label="Layout"
      value={value}
      options={[
        { value: "grid", label: "Grid" },
        { value: "list", label: "List" },
      ]}
      onValueChange={setValue}
    />
  );
}

it("renders page header actions and shared states semantically", async () => {
  const retry = vi.fn();
  render(
    <>
      <PageHeader title="Library" actions={<Button>Import</Button>} />
      <EmptyState title="No instances" description="Create one." />
      <ErrorState
        title="Could not load"
        description="Try again."
        action={<Button onClick={retry}>Retry</Button>}
      />
      <LoadingState>Loading instances…</LoadingState>
    </>,
  );

  expect(screen.getByRole("heading", { level: 1, name: "Library" })).toBeTruthy();
  expect(screen.getByRole("button", { name: "Import" })).toBeTruthy();
  expect(screen.getByRole("alert")).toBeTruthy();
  expect(screen.getByText("Loading instances…")).toBeTruthy();
  await userEvent.click(screen.getByRole("button", { name: "Retry" }));
  expect(retry).toHaveBeenCalledOnce();
});

it("supports keyboard tab navigation and skips disabled tabs", async () => {
  render(<TabsExample />);
  const user = userEvent.setup();
  const one = screen.getByRole("tab", { name: "One" });
  one.focus();
  await user.keyboard("{ArrowRight}");
  expect(screen.getByRole("tab", { name: "Three" }).getAttribute("aria-selected")).toBe("true");
  await user.keyboard("{Home}");
  expect(one.getAttribute("aria-selected")).toBe("true");
});

it("exposes segmented selection with pressed buttons", async () => {
  render(<SegmentedExample />);
  await userEvent.click(screen.getByRole("button", { name: "List" }));
  expect(screen.getByRole("button", { name: "List" }).getAttribute("aria-pressed")).toBe("true");
});
