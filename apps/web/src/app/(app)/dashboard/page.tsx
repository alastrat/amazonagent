"use client";

import { useQuery } from "@tanstack/react-query";
import { apiClient } from "@/lib/api-client";
import { queryKeys } from "@/lib/query-keys";
import { MetricCard } from "@/components/metric-card";
import { PageHeader } from "@/components/page-header";
import { StatusPill } from "@/components/status-pill";
import { ScoreBadge } from "@/components/score-badge";
import { PriceListUploadDialog } from "@/components/price-list-upload-dialog";
import { Button } from "@/components/ui/button";

export default function DashboardPage() {
  const { data, isLoading } = useQuery({
    queryKey: queryKeys.dashboard,
    queryFn: () => apiClient.getDashboardSummary(),
  });

  if (isLoading) return <div className="p-4">Loading...</div>;

  return (
    <div className="space-y-6">
      <PageHeader
        title="Dashboard"
        description="Overview of your sourcing pipeline"
        action={
          <PriceListUploadDialog>
            <Button>Upload Price List</Button>
          </PriceListUploadDialog>
        }
      />

      <div className="grid gap-4 md:grid-cols-3">
        <MetricCard title="Pending Review" value={data?.deals_pending_review ?? 0} description="Deals awaiting your decision" />
        <MetricCard title="Approved Deals" value={data?.deals_approved ?? 0} description="Ready for sourcing" />
        <MetricCard title="Active Campaigns" value={data?.active_campaigns ?? 0} description="Currently running" />
      </div>

      <div>
        <h2 className="mb-3 text-lg font-medium">Recent Deals</h2>
        {data?.recent_deals && data.recent_deals.length > 0 ? (
          <div className="rounded-lg border">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b bg-muted/50">
                  <th className="px-4 py-2 text-left font-medium">ASIN</th>
                  <th className="px-4 py-2 text-left font-medium">Title</th>
                  <th className="px-4 py-2 text-left font-medium">Score</th>
                  <th className="px-4 py-2 text-left font-medium">Status</th>
                </tr>
              </thead>
              <tbody>
                {data.recent_deals.map((deal) => (
                  <tr key={deal.id} className="border-b last:border-0">
                    <td className="px-4 py-2 font-mono text-xs">{deal.asin}</td>
                    <td className="px-4 py-2">{deal.title}</td>
                    <td className="px-4 py-2"><ScoreBadge score={Math.round(deal.scores.overall)} /></td>
                    <td className="px-4 py-2"><StatusPill status={deal.status} /></td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : (
          <p className="text-sm text-muted-foreground">No deals yet. Create a campaign to get started.</p>
        )}
      </div>
    </div>
  );
}
