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

// --- Concierge types ---

export type SellerArchetype = "greenhorn" | "ra_to_wholesale" | "expanding_pro" | "capital_rich";
export type AssessmentStatus = "pending" | "running" | "completed" | "failed";

export interface SellerProfile {
  id: string;
  tenant_id: string;
  archetype: SellerArchetype;
  account_age_days: number;
  active_listings: number;
  stated_capital: number;
  assessment_status: AssessmentStatus;
  assessed_at?: string;
}

export interface CategoryEligibility {
  category: string;
  probe_count: number;
  open_count: number;
  gated_count: number;
  open_rate: number;
}

export interface EligibilityFingerprint {
  id: string;
  tenant_id: string;
  categories: CategoryEligibility[];
  brand_results: Record<string, unknown>[];
  total_probes: number;
  total_eligible: number;
  total_restricted: number;
  overall_open_rate: number;
  confidence: number;
}

export type StrategyStatus = "draft" | "active" | "rolled_back" | "archived";

export interface StrategyGoal {
  id: string;
  type: "revenue" | "profit";
  target_amount: number;
  currency: string;
  timeframe_start: string;
  timeframe_end: string;
  target_categories: string[];
  current_progress: number;
}

export interface StrategyVersion {
  id: string;
  tenant_id: string;
  version_number: number;
  goals: StrategyGoal[];
  search_params: Record<string, unknown>;
  status: StrategyStatus;
  parent_version_id?: string;
  change_reason?: string;
  created_by: string;
  created_at: string;
  activated_at?: string;
}

export type SuggestionStatus = "pending" | "accepted" | "dismissed";

export interface DiscoverySuggestion {
  id: string;
  tenant_id: string;
  strategy_version_id: string;
  asin: string;
  title: string;
  brand: string;
  category: string;
  buy_box_price: number;
  estimated_margin_pct: number;
  bsr_rank: number;
  seller_count: number;
  reason: string;
  status: SuggestionStatus;
  deal_id?: string;
  created_at: string;
}

export interface CreditAccount {
  tier: string;
  monthly_limit: number;
  used: number;
  remaining: number;
  reset_at: string;
}

export interface CreditTransaction {
  id: string;
  tenant_id: string;
  amount: number;
  balance_after: number;
  description: string;
  created_at: string;
}

// --- Amazon Seller Account ---

export type SellerAccountStatus = "pending" | "valid" | "invalid" | "expired";

export interface AmazonSellerAccount {
  id: string;
  tenant_id: string;
  sp_api_client_id: string;
  seller_id: string;
  marketplace_id: string;
  status: SellerAccountStatus;
  error_message?: string;
  connected_at?: string;
  last_verified?: string;
  created_at: string;
  updated_at: string;
}

export interface ConnectSellerAccountRequest {
  sp_api_client_id: string;
  sp_api_client_secret: string;
  sp_api_refresh_token: string;
  seller_id: string;
  marketplace_id?: string;
}

// --- Assessment Graph ---

export type AssessmentGraphNodeType = "root" | "category" | "brand" | "product";
export type AssessmentGraphNodeStatus = "not_scanned" | "scanning" | "scanned" | "skipped";

export interface AssessmentGraphNode {
  id: string;
  type: AssessmentGraphNodeType;
  label: string;
  status: AssessmentGraphNodeStatus;
  eligible?: boolean;
  open_rate?: number;
  price?: number;
  margin?: number;
  category?: string;
  brand?: string;
}

export interface AssessmentGraphEdge {
  source: string;
  target: string;
}

export interface AssessmentGraphStats {
  categories_scanned: number;
  categories_total: number;
  eligible_products: number;
  ungatable_products: number;
  restricted_products: number;
  open_brands: number;
  restricted_brands: number;
  qualified_products?: number;
}

export interface AssessmentGraph {
  nodes: AssessmentGraphNode[];
  edges: AssessmentGraphEdge[];
  stats: AssessmentGraphStats;
}

// --- Assessment Outcome ---

export interface CategorySummary {
  category: string;
  eligible_count: number;
  qualified_count: number;
  avg_margin_pct: number;
  open_rate: number;
}

export interface ProductRecommendation {
  asin: string;
  title: string;
  brand: string;
  category: string;
  buy_box_price: number;
  est_margin_pct: number;
  seller_count: number;
  bsr_rank: number;
}

export interface UngatingStep {
  order: number;
  category: string;
  action: string;
  difficulty: string;
  est_days: number;
  impact: string;
}

export interface UngatingRoadmap {
  restricted_categories: { category: string; open_rate: number; difficulty: string }[];
  recommended_path: UngatingStep[];
  estimated_timeline: string;
}

export interface OpportunityResult {
  categories: CategorySummary[];
  products: ProductRecommendation[];
  strategy: StrategyGoal[];
}

export interface AssessmentOutcome {
  has_opportunities: boolean;
  qualified_count: number;
  eligible_categories: number;
  open_brands: number;
  opportunity?: OpportunityResult;
  ungating?: UngatingRoadmap;
}

// --- Assessment Tree (hierarchical drill-down) ---

export type EligibilityStatus = "eligible" | "ungatable" | "restricted";

export interface TreeNode {
  id: string;
  name: string;
  type?: "root" | "category" | "subcategory" | "brand" | "product";
  open_rate?: number;
  eligible_count?: number;
  ungatable_count?: number;
  total_count?: number;
  eligible?: boolean;
  eligibility_status?: EligibilityStatus;
  children?: TreeNode[];
  value?: number;
  asin?: string;
  price?: number;
  est_margin_pct?: number;
  seller_count?: number;
}

export interface ProductDetail {
  asin: string;
  title: string;
  brand: string;
  category: string;
  subcategory?: string;
  price: number;
  est_margin_pct: number;
  seller_count: number;
  eligible: boolean;
  eligibility_status?: EligibilityStatus;
  approval_url?: string;
}

// --- End concierge types ---

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
