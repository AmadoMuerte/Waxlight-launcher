import { call } from "./bridge";
import type { Statistics } from "./types";

export const statisticsApi = {
  overview: () => call<Statistics>("StatisticsController", "GetOverviewStatistics"),
};
