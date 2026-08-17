// @vitest-environment jsdom

import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { StatisticsPage } from "./StatisticsPage";

const statisticsQuery = vi.hoisted(() => ({ useStatisticsQuery: vi.fn() }));
const instancesQuery = vi.hoisted(() => ({ useInstancesQuery: vi.fn() }));

vi.mock("../../entities/statistics/queries", () => statisticsQuery);
vi.mock("../../entities/instance/queries", () => instancesQuery);

const baseStatistics = {
  totalPlaytimeSeconds: 0,
  launchCount: 0,
  averageSessionSeconds: 0,
  recentSessions: [],
};

function queryResult(overrides: Record<string, unknown>) {
  return {
    data: undefined,
    isPending: false,
    isError: false,
    error: null,
    refetch: vi.fn(),
    ...overrides,
  };
}

describe("StatisticsPage", () => {
  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
  });

  beforeEach(() => {
    instancesQuery.useInstancesQuery.mockReturnValue(queryResult({ data: [] }));
  });

  it("renders the primary summary metrics with formatted values", () => {
    statisticsQuery.useStatisticsQuery.mockReturnValue(
      queryResult({
        data: {
          totalPlaytimeSeconds: 3600 * 124 + 60 * 32,
          launchCount: 1248,
          averageSessionSeconds: 3900,
          recentSessions: [],
        },
      }),
    );

    render(<StatisticsPage />);

    expect(screen.getByText("Total playtime")).toBeTruthy();
    expect(screen.getByText("124h 32m")).toBeTruthy();
    expect(screen.getByText("1,248")).toBeTruthy();
    expect(screen.getByText("1h 5m")).toBeTruthy();
  });

  it("shows a sensible zero state for new users", () => {
    statisticsQuery.useStatisticsQuery.mockReturnValue(queryResult({ data: baseStatistics }));

    render(<StatisticsPage />);

    expect(screen.getAllByText("0m").length).toBeGreaterThan(0);
    expect(screen.getByText("No session history yet")).toBeTruthy();
  });

  it("lists recent sessions with instance names and crash state", () => {
    statisticsQuery.useStatisticsQuery.mockReturnValue(
      queryResult({
        data: {
          ...baseStatistics,
          recentSessions: [
            {
              id: "s1",
              instanceId: "inst-1",
              versionId: "1.20",
              startedAt: "2026-08-16T19:30:00Z",
              durationSeconds: 5400,
              crashed: false,
              recovered: false,
            },
            {
              id: "s2",
              instanceId: "inst-2",
              versionId: "1.20",
              startedAt: "2026-08-15T18:00:00Z",
              durationSeconds: 1200,
              crashed: true,
              recovered: false,
            },
          ],
        },
      }),
    );
    instancesQuery.useInstancesQuery.mockReturnValue(
      queryResult({
        data: [
          { id: "inst-1", name: "A Warm Home" },
          { id: "inst-2", name: "Broken World" },
        ],
      }),
    );

    render(<StatisticsPage />);

    expect(screen.getByText("A Warm Home")).toBeTruthy();
    expect(screen.getByText("1h 30m")).toBeTruthy();
    expect(screen.getByText("Broken World")).toBeTruthy();
    expect(screen.getByText("Crashed")).toBeTruthy();
  });

  it("falls back to a removed-instance label", () => {
    statisticsQuery.useStatisticsQuery.mockReturnValue(
      queryResult({
        data: {
          ...baseStatistics,
          recentSessions: [
            {
              id: "s1",
              instanceId: "gone",
              versionId: "1.20",
              startedAt: "2026-08-16T19:30:00Z",
              durationSeconds: 600,
              crashed: false,
              recovered: false,
            },
          ],
        },
      }),
    );

    render(<StatisticsPage />);

    expect(screen.getByText("Removed instance")).toBeTruthy();
  });

  it("shows a loading state while statistics are pending", () => {
    statisticsQuery.useStatisticsQuery.mockReturnValue(queryResult({ isPending: true }));

    render(<StatisticsPage />);

    expect(screen.getByText("Loading statistics")).toBeTruthy();
  });

  it("shows an error state with a working retry action", async () => {
    const user = userEvent.setup();
    const refetch = vi.fn().mockResolvedValue(undefined);
    statisticsQuery.useStatisticsQuery.mockReturnValue(
      queryResult({ isError: true, error: new Error("boom"), refetch }),
    );

    render(<StatisticsPage />);

    expect(screen.getByText("Could not load statistics")).toBeTruthy();
    await user.click(screen.getByRole("button", { name: "Retry" }));
    expect(refetch).toHaveBeenCalledTimes(1);
  });

  it("shows the most played instance when the backend reports one", () => {
    statisticsQuery.useStatisticsQuery.mockReturnValue(
      queryResult({
        data: { ...baseStatistics, mostPlayedInstanceId: "inst-1" },
      }),
    );
    instancesQuery.useInstancesQuery.mockReturnValue(
      queryResult({ data: [{ id: "inst-1", name: "A Warm Home" }] }),
    );

    render(<StatisticsPage />);

    expect(screen.getByText("Most played instance")).toBeTruthy();
    expect(screen.getByText("A Warm Home")).toBeTruthy();
  });
});
