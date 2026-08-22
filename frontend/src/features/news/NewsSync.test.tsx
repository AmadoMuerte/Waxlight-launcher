// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { afterEach, beforeEach, expect, it, vi } from "vitest";

import { useNotificationStore } from "../../app/stores/notifications";
import type { NewsFeed, NewsItem } from "../../shared/api/types";
import i18n from "../../shared/i18n";
import { NewsSync } from "./NewsSync";

const newsApi = vi.hoisted(() => ({ sync: vi.fn() }));
vi.mock("../../shared/api/news", () => ({ newsApi }));

function item(id: string): NewsItem {
  return {
    id,
    title: `Article ${id}`,
    url: `https://www.vintagestory.at/blog.html/news/${id}/`,
    summary: "Summary",
    publishedAt: "2026-08-23T00:00:00Z",
    category: "news",
  };
}

function feed(newItems: NewsItem[]): NewsFeed {
  return {
    items: newItems,
    newItems,
    fetchedAt: "2026-08-23T00:00:00Z",
    unreadCount: newItems.length,
    refreshFailed: false,
  };
}

function renderSync(value: NewsFeed) {
  newsApi.sync.mockResolvedValue(value);
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>
        <NewsSync />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  void i18n.changeLanguage("en");
  vi.clearAllMocks();
  useNotificationStore.setState({ notifications: [] });
});

afterEach(() => cleanup());

it("does not notify for the initial historical baseline", async () => {
  renderSync(feed([]));
  await waitFor(() => expect(newsApi.sync).toHaveBeenCalled());
  expect(useNotificationStore.getState().notifications).toHaveLength(0);
});

it("creates one notification for one new article", async () => {
  renderSync(feed([item("B")]));
  await waitFor(() => expect(useNotificationStore.getState().notifications).toHaveLength(1));
  const notification = useNotificationStore.getState().notifications[0];
  expect(notification.message).toBe("Article B");
  expect(notification.type).toBe("info");
});

it("aggregates several new articles into one notification", async () => {
  renderSync(feed([item("D"), item("C"), item("B")]));
  await waitFor(() => expect(useNotificationStore.getState().notifications).toHaveLength(1));
  expect(useNotificationStore.getState().notifications[0].message).toBe(
    "3 new articles are available.",
  );
});
