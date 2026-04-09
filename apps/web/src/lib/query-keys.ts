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
  catalog: {
    all: ["catalog"] as const,
    products: (params: Record<string, string>) => ["catalog", "products", params] as const,
    stats: ["catalog", "stats"] as const,
  },
  brands: {
    all: ["brands"] as const,
    list: (params: Record<string, string>) => ["brands", params] as const,
    detail: (id: string) => ["brands", id] as const,
    products: (id: string, params: Record<string, string>) => ["brands", id, "products", params] as const,
  },
  scans: {
    all: ["scans"] as const,
    detail: (id: string) => ["scans", id] as const,
  },
  assessment: {
    status: ["assessment", "status"] as const,
    profile: ["assessment", "profile"] as const,
  },
  strategy: {
    active: ["strategy"] as const,
    versions: ["strategy", "versions"] as const,
    detail: (id: string) => ["strategy", "versions", id] as const,
  },
  suggestions: {
    all: ["suggestions", "all"] as const,
    pending: ["suggestions", "pending"] as const,
  },
  credits: {
    account: ["credits"] as const,
    transactions: ["credits", "transactions"] as const,
  },
};
