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
