export const queryKeys = {
  dashboard: ["dashboard"] as const,
  campaigns: {
    all: ["campaigns"] as const,
    detail: (id: string) => ["campaigns", id] as const,
  },
  deals: {
    all: ["deals"] as const,
    list: (params: Record<string, string>) => ["deals", params] as const,
    detail: (id: string) => ["deals", id] as const,
  },
  scoring: ["scoring"] as const,
  discovery: ["discovery"] as const,
  events: (params: Record<string, string>) => ["events", params] as const,
};
