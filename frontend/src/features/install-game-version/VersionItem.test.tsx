// @vitest-environment jsdom

import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, it, vi } from "vitest";

import type { AvailableGameVersion } from "../../entities/game-version/model";
import { VersionItem } from "./VersionItem";

const version: AvailableGameVersion = {
  id: "1.22.6",
  name: "1.22.6",
  channel: "stable",
  platform: "linux",
  architecture: "amd64",
  downloadSize: 590_500_000,
  latest: true,
  installed: false,
};

afterEach(() => cleanup());

it("renders the metadata, channel and download action", () => {
  render(<VersionItem version={version} installed={false} onInstall={vi.fn()} />);
  expect(screen.getByText("1.22.6")).toBeTruthy();
  expect(screen.getByText("Latest")).toBeTruthy();
  expect(screen.getByText(/linux · amd64 ·/)).toBeTruthy();
  expect(screen.getByText("Stable")).toBeTruthy();
  expect(screen.getByRole("button", { name: "Download" })).toBeTruthy();
});

it("labels and disables installed versions", () => {
  render(<VersionItem version={version} installed onInstall={vi.fn()} />);
  const button = screen.getByRole("button", { name: "Installed" }) as HTMLButtonElement;
  expect(button.disabled).toBe(true);
});

it("renders pre-release channels without danger styling", () => {
  render(
    <VersionItem
      version={{ ...version, id: "1.23.0-pre.1", channel: "unstable", latest: false }}
      installed={false}
      onInstall={vi.fn()}
    />,
  );
  expect(screen.getByText("Preview")).toBeTruthy();
  const pill = screen.getByText("Preview");
  expect(pill.className).not.toContain("status-failed");
});

it("shows a busy state on the primary action", () => {
  render(<VersionItem version={version} installed={false} busy onInstall={vi.fn()} />);
  const button = screen.getByRole("button", { name: "Download" }) as HTMLButtonElement;
  expect(button.disabled).toBe(true);
  expect(button.getAttribute("aria-busy")).toBe("true");
});

it("triggers the install action", async () => {
  const onInstall = vi.fn();
  const user = userEvent.setup();
  render(<VersionItem version={version} installed={false} onInstall={onInstall} />);
  await user.click(screen.getByRole("button", { name: "Download" }));
  expect(onInstall).toHaveBeenCalledTimes(1);
});

it("truncates long metadata safely", () => {
  render(
    <VersionItem
      version={{
        ...version,
        id: "1.22.6-pre.1",
        name: "A very long translated pre-release version name that must never break the row actions",
        channel: "unstable",
        latest: false,
      }}
      installed={false}
      onInstall={vi.fn()}
    />,
  );
  expect(
    screen.getByText(
      "A very long translated pre-release version name that must never break the row actions",
    ),
  ).toBeTruthy();
  expect(screen.getByRole("button", { name: "Download" })).toBeTruthy();
});
