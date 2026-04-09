"use client";

import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { apiClient } from "@/lib/api-client";
import { queryKeys } from "@/lib/query-keys";
import { useAssessmentStatus } from "@/hooks/use-assessment";
import { useCredits } from "@/hooks/use-credits";
import { usePendingSuggestions } from "@/hooks/use-suggestions";
import { MetricCard } from "@/components/metric-card";
import { PageHeader } from "@/components/page-header";
import { StatusPill } from "@/components/status-pill";
import { ScoreBadge } from "@/components/score-badge";
import { PriceListUploadDialog } from "@/components/price-list-upload-dialog";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";

export default function DashboardPage() {
  const { data, isLoading } = useQuery({
    queryKey: queryKeys.dashboard,
    queryFn: () => apiClient.getDashboardSummary(),
  });
  const { data: assessment } = useAssessmentStatus(false);
  const { data: credits } = useCredits();
  const { data: suggestions } = usePendingSuggestions();

  const pendingCount = suggestions?.filter((s: any) => s.status === "pending").length ?? 0;
  const hasAssessment = assessment?.status === "completed";

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

      {!hasAssessment && (
        <Card>
          <CardContent className="flex items-center justify-between py-4">
            <div>
              <p className="font-medium">Welcome! Complete your onboarding to get started.</p>
              <p className="text-sm text-muted-foreground">
                We will assess your account and build a custom sourcing strategy.
              </p>
            </div>
            <Link href="/onboarding">
              <Button>Get Started</Button>
            </Link>
          </CardContent>
        </Card>
      )}

      <div className="grid gap-4 md:grid-cols-3">
        <MetricCard title="Pending Review" value={data?.deals_pending_review ?? 0} description="Deals awaiting your decision" />
        <MetricCard title="Approved Deals" value={data?.deals_approved ?? 0} description="Ready for sourcing" />
        <MetricCard title="Active Campaigns" value={data?.active_campaigns ?? 0} description="Currently running" />
      </div>

      <div className="grid gap-4 md:grid-cols-3">
        {credits && (
          <Card>
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">Credit Balance</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">{credits.remaining}</div>
              <p className="text-xs text-muted-foreground">
                {credits.used}/{credits.monthly_limit} used ({credits.tier} tier)
              </p>
              {credits.reset_at && (
                <p className="text-xs text-muted-foreground">
                  Resets {new Date(credits.reset_at).toLocaleDateString()}
                </p>
              )}
            </CardContent>
          </Card>
        )}
        <MetricCard
          title="Pending Suggestions"
          value={pendingCount}
          description="Products awaiting your review"
        />
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
