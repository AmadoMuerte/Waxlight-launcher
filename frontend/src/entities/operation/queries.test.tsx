// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook } from "@testing-library/react";
import type { PropsWithChildren } from "react";
import { afterEach, expect, it, vi } from "vitest";

import { useOperationsQuery } from "./queries";

const list = vi.hoisted(() => vi.fn());

vi.mock("../../shared/api/operations", () => ({
  operationsApi: { list },
}));

afterEach(() => {
  vi.useRealTimers();
  list.mockReset();
});

it("stops polling after an operation query error", async () => {
  vi.useFakeTimers();
  list.mockRejectedValue(new Error("database unavailable"));
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const wrapper = ({ children }: PropsWithChildren) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );

  renderHook(() => useOperationsQuery({ refetchInterval: 1_000 }), { wrapper });
  await act(async () => {});
  expect(list).toHaveBeenCalledTimes(1);

  await act(() => vi.advanceTimersByTimeAsync(5_000));
  expect(list).toHaveBeenCalledTimes(1);
  queryClient.clear();
});
