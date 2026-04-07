export type CampaignType = "discovery_run" | "manual" | "experiment";
export type CampaignStatus = "pending" | "running" | "completed" | "failed";
export type TriggerType = "chat" | "dashboard" | "scheduler" | "spreadsheet";

export type DealStatus =
  | "discovered"
  | "analyzing"
  | "needs_review"
  | "approved"
  | "rejected"
  | "sourcing"
  | "procuring"
  | "listing"
  | "live"
  | "monitoring"
  | "reorder"
  | "archived";

export interface Criteria {
  keywords: string[];
  min_monthly_revenue?: number;
  min_margin_pct?: number;
  max_wholesale_cost?: number;
  max_moq?: number;
  preferred_brands?: string[];
  marketplace: string;
}

export interface Campaign {
  id: string;
  tenant_id: string;
  type: CampaignType;
  criteria: Criteria;
  scoring_config_id: string;
  experiment_id?: string;
  source_file?: string;
  status: CampaignStatus;
  created_by: string;
  trigger_type: TriggerType;
  created_at: string;
  completed_at?: string;
}

export interface DealScores {
  demand: number;
  competition: number;
  margin: number;
  risk: number;
  sourcing_feasibility: number;
  overall: number;
}

export interface AgentEvidence {
  reasoning: string;
  data: Record<string, unknown>;
}

export interface Evidence {
  demand: AgentEvidence;
  competition: AgentEvidence;
  margin: AgentEvidence;
  risk: AgentEvidence;
  sourcing: AgentEvidence;
}

export interface Deal {
  id: string;
  tenant_id: string;
  campaign_id: string;
  asin: string;
  title: string;
  brand: string;
  category: string;
  status: DealStatus;
  scores: DealScores;
  evidence: Evidence;
  reviewer_verdict: string;
  iteration_count: number;
  created_at: string;
  updated_at: string;
}

export interface ScoringWeights {
  demand: number;
  competition: number;
  margin: number;
  risk: number;
  sourcing: number;
}

export interface Thresholds {
  min_overall: number;
  min_per_dimension: number;
}

export interface ScoringConfig {
  id: string;
  tenant_id: string;
  version: number;
  weights: ScoringWeights;
  thresholds: Thresholds;
  created_by: string;
  active: boolean;
  created_at: string;
}

export interface DashboardSummary {
  deals_pending_review: number;
  deals_approved: number;
  active_campaigns: number;
  recent_deals: Deal[];
}

export interface DomainEvent {
  id: string;
  tenant_id: string;
  event_type: string;
  entity_type: string;
  entity_id: string;
  payload: Record<string, unknown>;
  correlation_id: string;
  actor_id: string;
  timestamp: string;
}

export interface DiscoveredProduct {
  id: string;
  tenant_id: string;
  asin: string;
  title: string;
  brand_id: string;
  category: string;
  browse_node_id?: string;
  estimated_price?: number;
  buy_box_price?: number;
  bsr_rank?: number;
  seller_count?: number;
  estimated_margin_pct?: number;
  real_margin_pct?: number;
  eligibility_status: string;
  data_quality: number;
  refresh_priority: number;
  source: string;
  first_seen_at: string;
  last_seen_at: string;
  price_updated_at?: string;
}

export interface BrandIntelligence {
  tenant_id: string;
  brand_id: string;
  brand_name: string;
  category: string;
  product_count: number;
  high_margin_count: number;
  avg_margin: number;
  avg_sellers: number;
  avg_bsr: number;
}

export interface CatalogStats {
  total_products: number;
  total_brands: number;
  avg_margin: number;
  eligible_count: number;
}

export interface FunnelStats {
  input_count: number;
  t0_deduped: number;
  t1_margin_killed: number;
  t2_brand_killed: number;
  t3_enrich_killed: number;
  survivor_count: number;
}

export interface ScanJob {
  id: string;
  tenant_id: string;
  type: string;
  status: string;
  total_items: number;
  processed: number;
  qualified: number;
  eliminated: number;
  started_at: string;
  completed_at?: string;
  metadata?: Record<string, any>;
}

export interface UploadFunnelResponse {
  scan_id: string;
  funnel: FunnelStats;
}

export type DiscoveryCadence = "nightly" | "twice_daily" | "weekly";

export interface DiscoveryConfig {
  id: string;
  tenant_id: string;
  categories: string[];
  baseline_criteria: Criteria;
  scoring_config_id: string;
  cadence: DiscoveryCadence;
  enabled: boolean;
  last_run_at?: string;
  next_run_at?: string;
}
