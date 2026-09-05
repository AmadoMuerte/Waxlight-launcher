export const ACCOUNTS_QUERY_KEY = ["accounts"] as const;
export const INSTANCES_QUERY_KEY = ["instances"] as const;
export const GAME_VERSIONS_QUERY_KEY = ["game-versions"] as const;
export const AVAILABLE_GAME_VERSIONS_QUERY_KEY = ["game-versions", "available"] as const;
export const OPERATIONS_QUERY_KEY = ["operations"] as const;
export const STATISTICS_QUERY_KEY = ["statistics"] as const;
export const SETTINGS_QUERY_KEY = ["settings"] as const;
export const OPTIMUM_STATUS_QUERY_KEY = ["settings", "optimum"] as const;
export const FAVORITE_SERVERS_QUERY_KEY = ["favorite-servers"] as const;
export const PUBLIC_SERVERS_QUERY_KEY = ["public-servers"] as const;
export const PUBLIC_SERVER_DETAILS_QUERY_KEY = (id: string) => ["public-servers", id] as const;
export const NEWS_QUERY_KEY = ["news"] as const;

export const DOWNLOADED_MODS_QUERY_KEY = ["mods", "downloaded"] as const;
export const MOD_TAGS_QUERY_KEY = ["mods", "tags"] as const;
export const MOD_DETAILS_QUERY_KEY = (modId: string) => ["mods", "details", modId] as const;

export const SNAPSHOTS_QUERY_KEY = (instanceId: string) => ["snapshots", instanceId] as const;

export const LAST_KNOWN_GOOD_QUERY_KEY = (instanceId: string) =>
  ["last-known-good", instanceId] as const;

export const WATCHED_QUERY_KEYS = [
  ACCOUNTS_QUERY_KEY,
  INSTANCES_QUERY_KEY,
  GAME_VERSIONS_QUERY_KEY,
  OPERATIONS_QUERY_KEY,
  STATISTICS_QUERY_KEY,
  SETTINGS_QUERY_KEY,
] as const;
