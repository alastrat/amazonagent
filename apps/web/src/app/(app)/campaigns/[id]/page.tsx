"use client";

import { use } from "react";
import Link from "next/link";
import { useCampaign } from "@/hooks/use-campaigns";
import { useDeals } from "@/hooks/use-deals";
import { PageHeader } from "@/components/page-header";
import { StatusPill } from "@/components/status-pill";
import { ScoreBadge } from "@/components/score-badge";
import { EmptyState } from "@/components/empty-state";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { MetricCard } from "@/components/metric-card";

export default function CampaignDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = use(params);
  const { data: campaign, isLoading: loadingCampaign } = useCampaign(id);
  const { data: dealsData, isLoading: loadingDeals } = useDeals({
    campaign_id: id,
  });

  if (loadingCampaign) return <div className="p-4">Loading...</div>;
  if (!campaign) return <div className="p-4">Campaign not found</div>;

  const deals = dealsData?.deals || [];
  const avgScore =
    deals.length > 0
      ? deals.reduce((sum, d) => sum + d.scores.overall, 0) / deals.length
      : 0;

  return (
    <div className="space-y-6">
      <PageHeader
        title={`Campaign: ${campaign.type}`}
        description={`Created ${new Date(campaign.created_at).toLocaleString()} via ${campaign.trigger_type}`}
        action={<StatusPill status={campaign.status} />}
      />

      <div className="grid gap-4 md:grid-cols-4">
        <MetricCard title="Status" value={campaign.status} />
        <MetricCard title="Deals Found" value={deals.length} />
        <MetricCard
          title="Avg Score"
          value={avgScore > 0 ? avgScore.toFixed(1) : "—"}
        />
        <MetricCard title="Marketplace" value={campaign.criteria.marketplace} />
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Criteria</CardTitle>
        </CardHeader>
        <CardContent>
          <dl className="grid grid-cols-2 gap-x-4 gap-y-2 text-sm">
            <dt className="text-muted-foreground">Keywords</dt>
            <dd>{campaign.criteria.keywords?.join(", ") || "—"}</dd>
            {campaign.criteria.min_margin_pct && (
              <>
                <dt className="text-muted-foreground">Min Margin</dt>
                <dd>{campaign.criteria.min_margin_pct}%</dd>
              </>
            )}
            {campaign.criteria.min_monthly_revenue && (
              <>
                <dt className="text-muted-foreground">Min Monthly Revenue</dt>
                <dd>${campaign.criteria.min_monthly_revenue.toLocaleString()}</dd>
              </>
            )}
            {campaign.criteria.max_wholesale_cost && (
              <>
                <dt className="text-muted-foreground">Max Wholesale Cost</dt>
                <dd>${campaign.criteria.max_wholesale_cost}</dd>
              </>
            )}
            {campaign.criteria.preferred_brands &&
              campaign.criteria.preferred_brands.length > 0 && (
                <>
                  <dt className="text-muted-foreground">Preferred Brands</dt>
                  <dd>{campaign.criteria.preferred_brands.join(", ")}</dd>
                </>
              )}
          </dl>
        </CardContent>
      </Card>

      <div>
        <h2 className="mb-3 text-lg font-medium">Deals from this Campaign</h2>
        {loadingDeals ? (
          <div>Loading deals...</div>
        ) : deals.length === 0 ? (
          <EmptyState
            title="No deals yet"
            description={
              campaign.status === "running"
                ? "Pipeline is running — deals will appear shortly."
                : campaign.status === "pending"
                  ? "Campaign is queued — waiting for pipeline to start."
                  : "No deals were found matching the criteria."
            }
          />
        ) : (
          <div className="rounded-lg border">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b bg-muted/50">
                  <th className="px-4 py-2 text-left font-medium">ASIN</th>
                  <th className="px-4 py-2 text-left font-medium">Title</th>
                  <th className="px-4 py-2 text-left font-medium">Brand</th>
                  <th className="px-4 py-2 text-left font-medium">Score</th>
                  <th className="px-4 py-2 text-left font-medium">Margin</th>
                  <th className="px-4 py-2 text-left font-medium">Status</th>
                </tr>
              </thead>
              <tbody>
                {deals.map((deal) => (
                  <tr
                    key={deal.id}
                    className="border-b last:border-0 hover:bg-muted/30"
                  >
                    <td className="px-4 py-2">
                      <Link
                        href={`/deals/${deal.id}`}
                        className="font-mono text-xs text-primary hover:underline"
                      >
                        {deal.asin}
                      </Link>
                    </td>
                    <td className="px-4 py-2">{deal.title}</td>
                    <td className="px-4 py-2 text-muted-foreground">
                      {deal.brand}
                    </td>
                    <td className="px-4 py-2">
                      <ScoreBadge score={Math.round(deal.scores.overall)} />
                    </td>
                    <td className="px-4 py-2">
                      <ScoreBadge score={deal.scores.margin} label="Margin" />
                    </td>
                    <td className="px-4 py-2">
                      <StatusPill status={deal.status} />
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  );
}
