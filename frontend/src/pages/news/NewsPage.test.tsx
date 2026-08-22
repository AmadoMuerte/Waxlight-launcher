// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { afterEach, beforeEach, expect, it, vi } from "vitest";

import { NEWS_QUERY_KEY } from "../../shared/api/keys";
import type { NewsFeed, NewsItem } from "../../shared/api/types";
import { NewsPage } from "./NewsPage";

const newsApi = vi.hoisted(() => ({
  sync: vi.fn(),
  markSeen: vi.fn(),
  openArticle: vi.fn(),
}));
vi.mock("../../shared/api/news", () => ({ newsApi }));

function item(id: string, title: string, publishedAt: string): NewsItem {
  return {
    id,
    title,
    url: `https://www.vintagestory.at/blog.html/news/${id}/`,
    summary: `${title} summary`,
    imageUrl: `https://media.vintagestory.at/${id}.png`,
    publishedAt,
    category: "release",
  };
}

const items = [
  item("B", "Newest article", "2026-08-23T00:00:00Z"),
  item("A", "Older article", "2026-08-22T00:00:00Z"),
];

function feed(overrides: Partial<NewsFeed> = {}): NewsFeed {
  return {
    items,
    newItems: [],
    fetchedAt: "2026-08-23T00:00:00Z",
    unreadCount: 2,
    refreshFailed: false,
    ...overrides,
  };
}

function renderPage() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>
        <NewsPage />
      </MemoryRouter>
    </QueryClientProvider>,
  );
  return queryClient;
}

beforeEach(() => {
  vi.clearAllMocks();
  newsApi.markSeen.mockResolvedValue(undefined);
  newsApi.openArticle.mockResolvedValue(undefined);
});

afterEach(() => cleanup());

it("renders entries newest-first, opens the original article, and marks the feed seen", async () => {
  newsApi.sync.mockResolvedValue(feed());
  const user = userEvent.setup();
  renderPage();

  const headings = await screen.findAllByRole("heading", { level: 2 });
  expect(headings.map((heading) => heading.textContent)).toEqual([
    "Newest article",
    "Older article",
  ]);
  await waitFor(() => expect(newsApi.markSeen).toHaveBeenCalledWith(["B", "A"]));
  await user.click(screen.getAllByRole("button", { name: /Read more/ })[0]);
  expect(newsApi.openArticle).toHaveBeenCalledWith(items[0].url);
});

it("shows cached articles with a non-fatal refresh status", async () => {
  newsApi.sync.mockResolvedValue(feed({ refreshFailed: true }));
  renderPage();
  expect(await screen.findByText("Newest article")).toBeTruthy();
  expect(screen.getByText("Could not refresh news.")).toBeTruthy();
  expect(screen.getByText("Showing the last downloaded articles.")).toBeTruthy();
});

it("shows loading and then an error when no cache exists", async () => {
  let reject!: (error: Error) => void;
  newsApi.sync.mockReturnValue(new Promise((_, rejectPromise) => (reject = rejectPromise)));
  renderPage();
  expect(screen.getByText("Loading Vintage Story news…")).toBeTruthy();
  reject(new Error("offline"));
  expect(await screen.findByText("Unable to load Vintage Story news")).toBeTruthy();
});

it("keeps existing cards visible during manual refresh", async () => {
  let resolveRefresh!: (value: NewsFeed) => void;
  newsApi.sync
    .mockResolvedValueOnce(feed())
    .mockReturnValueOnce(new Promise((resolve) => (resolveRefresh = resolve)));
  const user = userEvent.setup();
  renderPage();
  await screen.findByText("Newest article");
  await user.click(screen.getByRole("button", { name: "Refresh" }));
  expect(screen.getByText("Newest article")).toBeTruthy();
  resolveRefresh(feed());
});

it("keeps cached cards and warns when a background refresh fails", async () => {
  newsApi.sync.mockResolvedValueOnce(feed());
  const queryClient = renderPage();
  await screen.findByText("Newest article");
  newsApi.sync.mockRejectedValueOnce(new Error("offline"));
  await queryClient.invalidateQueries({ queryKey: NEWS_QUERY_KEY });
  expect(screen.getByText("Newest article")).toBeTruthy();
  expect(await screen.findByText("Could not refresh news.")).toBeTruthy();
});
