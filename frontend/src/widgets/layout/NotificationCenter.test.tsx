// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, it, vi } from "vitest";

import { useNotificationStore } from "../../app/stores/notifications";
import { NotificationCenter } from "./NotificationCenter";

afterEach(() => {
  cleanup();
  useNotificationStore.setState({ notifications: [] });
});

it("runs notification actions and tracks unread notifications", async () => {
  const user = userEvent.setup();
  const run = vi.fn();
  useNotificationStore.getState().addNotification({
    id: "warning:1",
    type: "warning",
    title: "Important warning",
    message: "Review this warning",
    action: { label: "Review", run },
  });
  useNotificationStore.getState().addNotification({
    id: "news:1",
    type: "info",
    title: "Waxlight news",
    message: "A new article is available",
  });

  render(<NotificationCenter />);
  await user.click(screen.getByRole("button", { name: "Notifications" }));
  await user.click(await screen.findByRole("menuitem", { name: /Important warning/ }));

  expect(run).toHaveBeenCalledOnce();
  expect(
    useNotificationStore.getState().notifications.find(({ id }) => id === "warning:1")?.read,
  ).toBe(true);

  await user.click(screen.getByRole("button", { name: "Notifications" }));
  fireEvent.click(await screen.findByRole("button", { name: "Mark all as read" }));
  await waitFor(() =>
    expect(useNotificationStore.getState().notifications.every(({ read }) => read)).toBe(true),
  );
  expect(document.querySelector(".notificationBell")?.className).not.toContain("unread");
});
