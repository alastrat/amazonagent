import type {
  Campaign,
  Criteria,
  Deal,
  ScoringConfig,
  ScoringWeights,
  Thresholds,
  DashboardSummary,
  DomainEvent,
  DiscoveryConfig,
  DiscoveredProduct,
  BrandIntelligence,
  CatalogStats,
  ScanJob,
  UploadFunnelResponse,
  SellerProfile,
  EligibilityFingerprint,
  StrategyVersion,
  DiscoverySuggestion,
  CreditAccount,
  CreditTransaction,
  AmazonSellerAccount,
  ConnectSellerAccountRequest,
  AssessmentGraph,
  AssessmentOutcome,
} from "./types";

const API_BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8081";

class ApiClient {
  private token: string | null = null;

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

  private async fetchMultipart<T>(path: string, body: FormData): Promise<T> {
    const headers: Record<string, string> = {};
    if (this.token) {
      headers["Authorization"] = `Bearer ${this.token}`;
    }

    const res = await fetch(`${API_BASE}${path}`, {
      method: "POST",
      headers,
      body,
    });

    if (!res.ok) {
      const resBody = await res.json().catch(() => ({}));
      throw new Error(resBody.error || `API error: ${res.status}`);
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

  getDiscovery() {
    return this.fetch<DiscoveryConfig>("/discovery");
  }

  updateDiscovery(data: Partial<DiscoveryConfig>) {
    return this.fetch<DiscoveryConfig>("/discovery", {
      method: "PUT",
      body: JSON.stringify(data),
    });
  }

  getCatalogProducts(params?: Record<string, string>) {
    const qs = params ? "?" + new URLSearchParams(params).toString() : "";
    return this.fetch<{ products: DiscoveredProduct[]; total: number }>(`/catalog/products${qs}`);
  }

  getCatalogStats() {
    return this.fetch<CatalogStats>("/catalog/stats");
  }

  getBrands(params?: Record<string, string>) {
    const qs = params ? "?" + new URLSearchParams(params).toString() : "";
    return this.fetch<{ brands: BrandIntelligence[]; total: number }>(`/catalog/brands${qs}`);
  }

  getBrandProducts(brandId: string, params?: Record<string, string>) {
    const qs = params ? "?" + new URLSearchParams(params).toString() : "";
    return this.fetch<{ products: DiscoveredProduct[]; total: number }>(`/catalog/brands/${brandId}/products${qs}`);
  }

  evaluateProducts(asins: string[]) {
    return this.fetch<ScanJob>("/scans/category", {
      method: "POST",
      body: JSON.stringify({ asins }),
    });
  }

  uploadPriceList(file: File, distributor: string) {
    const form = new FormData();
    form.append("file", file);
    form.append("distributor", distributor);
    return this.fetchMultipart<UploadFunnelResponse>("/pricelist/upload-funnel", form);
  }

  getScans() {
    return this.fetch<ScanJob[]>("/scans");
  }

  getScan(id: string) {
    return this.fetch<ScanJob>(`/scans/${id}`);
  }

  // --- Seller Account ---

  connectSellerAccount(credentials: ConnectSellerAccountRequest) {
    return this.fetch<AmazonSellerAccount>("/seller-account/connect", {
      method: "POST",
      body: JSON.stringify(credentials),
    });
  }

  getSellerAccount() {
    return this.fetch<AmazonSellerAccount>("/seller-account");
  }

  disconnectSellerAccount() {
    return this.fetch<{ status: string }>("/seller-account/disconnect", {
      method: "DELETE",
    });
  }

  // --- Assessment ---

  startAssessment(data?: { account_age_days?: number; active_listings?: number; stated_capital?: number }) {
    return this.fetch<SellerProfile>("/assessment/start", {
      method: "POST",
      body: JSON.stringify(data ?? {}),
    });
  }

  getAssessmentStatus() {
    return this.fetch<{ status: string; archetype: string }>("/assessment/status");
  }

  getAssessmentProfile() {
    return this.fetch<{ profile: SellerProfile; fingerprint: EligibilityFingerprint }>("/assessment/profile");
  }

  getAssessmentGraph() {
    return this.fetch<{ graph: AssessmentGraph; status: string; outcome?: AssessmentOutcome }>("/assessment/graph");
  }

  // --- Strategy ---

  getActiveStrategy() {
    return this.fetch<StrategyVersion>("/strategy");
  }

  getStrategyVersions() {
    return this.fetch<StrategyVersion[]>("/strategy/versions");
  }

  getStrategyVersion(id: string) {
    return this.fetch<StrategyVersion>(`/strategy/versions/${id}`);
  }

  activateStrategyVersion(id: string) {
    return this.fetch<{ status: string }>(`/strategy/versions/${id}/activate`, {
      method: "POST",
    });
  }

  rollbackStrategyVersion(id: string) {
    return this.fetch<StrategyVersion>(`/strategy/versions/${id}/rollback`, {
      method: "POST",
    });
  }

  // --- Suggestions ---

  getPendingSuggestions() {
    return this.fetch<DiscoverySuggestion[]>("/suggestions");
  }

  getAllSuggestions() {
    return this.fetch<DiscoverySuggestion[]>("/suggestions/all");
  }

  acceptSuggestion(id: string) {
    return this.fetch<{ status: string }>(`/suggestions/${id}/accept`, {
      method: "POST",
    });
  }

  dismissSuggestion(id: string) {
    return this.fetch<{ status: string }>(`/suggestions/${id}/dismiss`, {
      method: "POST",
    });
  }

  // --- Credits ---

  getCredits() {
    return this.fetch<CreditAccount>("/credits");
  }

  getCreditTransactions() {
    return this.fetch<CreditTransaction[]>("/credits/transactions");
  }
}

export const apiClient = new ApiClient();
