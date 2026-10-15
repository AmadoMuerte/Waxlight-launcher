import { afterEach, beforeAll, beforeEach, expect, it, vi } from "bun:test";

import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import type { Instance } from "../../entities/instance/model";
import i18n from "../../shared/i18n";
import { ExportInstanceModal } from "./ExportInstanceModal";

const instancePackageApi = {
  selectExportPath: vi.fn(),
  export: vi.fn(),
};

void vi.mock("../../shared/api/instance-package", () => ({ instancePackageApi }));

const instance: Instance = {
  id: "instance-1",
  name: "Warm home",
  description: "A cozy base",
  gameVersionId: "1.20",
  gameClient: "vanilla",
  directory: "/instances/warm-home",
  status: "ready",
  launchArguments: [],
  environmentVariables: {},
  isPinned: false,
  createdAt: "2026-01-01T00:00:00Z",
  enabledModCount: 0,
  totalModCount: 0,
  playtimeSeconds: 0,
};

beforeAll(() => i18n.changeLanguage("en"));
beforeEach(() => vi.clearAllMocks());
afterEach(cleanup);

it("keeps the export form usable when destination selection is cancelled", async () => {
  instancePackageApi.selectExportPath.mockResolvedValue("");
  render(<ExportInstanceModal instance={instance} onClose={vi.fn()} onDone={vi.fn()} />);

  expect(screen.getByRole("textbox", { name: "Name" })).toBeTruthy();
  expect(screen.getByRole("textbox", { name: "Description" })).toBeTruthy();

  await userEvent.setup().click(screen.getByRole("button", { name: "Export" }));

  await waitFor(() => expect(instancePackageApi.selectExportPath).toHaveBeenCalledTimes(1));
  expect(screen.getByRole("button", { name: "Export" }).hasAttribute("disabled")).toBe(false);
  expect(instancePackageApi.export).not.toHaveBeenCalled();
});
