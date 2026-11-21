import { afterEach, expect, it, vi } from "bun:test";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes, useNavigate } from "react-router";

import i18n from "../../shared/i18n";
import { AppShell } from "./AppShell";

void vi.mock("../../entities/operation/queries", () => ({
  useOperationsQuery: () => ({ data: [] }),
}));
void vi.mock("../../entities/news/queries", () => ({
  useNewsQuery: () => ({ data: { unreadCount: 0 } }),
}));
void vi.mock("../../entities/account/queries", () => ({
  useAccountsQuery: () => ({ data: [] }),
}));

afterEach(() => cleanup());

function Jump({ to }: { to: string }) {
  const navigate = useNavigate();
  return (
    <button type="button" onClick={() => void navigate(to)}>
      go
    </button>
  );
}

function renderShell(initial: string, listPath: string, detailsPath: string) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[initial]}>
        <AppShell>
          <Routes>
            <Route path={listPath} element={<Jump to={detailsPath} />} />
            <Route path={detailsPath} element={<Jump to={listPath} />} />
          </Routes>
        </AppShell>
      </MemoryRouter>
    </QueryClientProvider>,
  );
  return document.querySelector(".appMain") as HTMLElement;
}

it("resets scroll on forward navigation and restores it when going back", () => {
  void i18n.changeLanguage("en");
  const main = renderShell("/scroll-mods", "/scroll-mods", "/scroll-mods/1");

  main.scrollTop = 240;
  fireEvent.scroll(main);
  fireEvent.click(screen.getByText("go"));
  expect(main.scrollTop).toBe(0);

  fireEvent.click(screen.getByText("go"));
  expect(main.scrollTop).toBe(240);
});

it("starts a fresh route at the top", () => {
  void i18n.changeLanguage("en");
  const main = renderShell("/fresh-a", "/fresh-a", "/fresh-b");

  main.scrollTop = 500;
  fireEvent.scroll(main);
  fireEvent.click(screen.getByText("go"));
  expect(main.scrollTop).toBe(0);
});

it("keeps the saved position when the shorter incoming page clamps the scroller", () => {
  void i18n.changeLanguage("en");
  const main = renderShell("/clamp-mods", "/clamp-mods", "/clamp-mods/1");

  main.scrollTop = 240;
  fireEvent.scroll(main);
  fireEvent.click(screen.getByText("go"));
  // The browser clamps scrollTop while the short loading state is shown and
  // fires a scroll event for it; this must not overwrite the saved offset.
  fireEvent.scroll(main);

  fireEvent.click(screen.getByText("go"));
  expect(main.scrollTop).toBe(240);
});
