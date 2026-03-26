import type {
  Campaign,
  Criteria,
  Deal,
  ScoringConfig,
  ScoringWeights,
  Thresholds,
  DashboardSummary,
  DomainEvent,
} from "./types";

const API_BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8081";

class ApiClient {
  // Default to dev token for local development — replaced by real Supabase auth in production
  private token: string | null = "dev-user-dev-tenant";

  setToken(token: string) {
    this.token = token;
  }

  private async fetch<T>(path: string, options?: RequestInit): Promise<T> {
    const headers: Record<string, string> = {
      "Content-Type": "application/json",
    };
    if (this.token) {
      headers["Authorization"] = `Bearer ${this.token}`;
    }

    const res = await fetch(`${API_BASE}${path}`, {
      ...options,
      headers: { ...headers, ...options?.headers },
    });

    if (!res.ok) {
      const body = await res.json().catch(() => ({}));
      throw new Error(body.error || `API error: ${res.status}`);
    }

    return res.json();
  }

  getCampaigns() {
    return this.fetch<Campaign[]>("/campaigns");
  }

  getCampaign(id: string) {
    return this.fetch<Campaign>(`/campaigns/${id}`);
  }

  createCampaign(data: { type: string; trigger_type: string; criteria: Criteria }) {
    return this.fetch<Campaign>("/campaigns", {
      method: "POST",
      body: JSON.stringify(data),
    });
  }

  getDeals(params?: Record<string, string>) {
    const qs = params ? "?" + new URLSearchParams(params).toString() : "";
    return this.fetch<{ deals: Deal[]; total: number }>(`/deals${qs}`);
  }

  getDeal(id: string) {
    return this.fetch<Deal>(`/deals/${id}`);
  }

  approveDeal(id: string) {
    return this.fetch<Deal>(`/deals/${id}/approve`, { method: "POST" });
  }

  rejectDeal(id: string, reason?: string) {
    return this.fetch<Deal>(`/deals/${id}/reject`, {
      method: "POST",
      body: JSON.stringify({ reason }),
    });
  }

  getScoringConfig() {
    return this.fetch<ScoringConfig>("/config/scoring");
  }

  updateScoringConfig(data: { weights: ScoringWeights; thresholds: Thresholds }) {
    return this.fetch<ScoringConfig>("/config/scoring", {
      method: "PUT",
      body: JSON.stringify(data),
    });
  }

  getDashboardSummary() {
    return this.fetch<DashboardSummary>("/dashboard/summary");
  }

  getEvents(params?: Record<string, string>) {
    const qs = params ? "?" + new URLSearchParams(params).toString() : "";
    return this.fetch<DomainEvent[]>(`/events${qs}`);
  }
}

export const apiClient = new ApiClient();
