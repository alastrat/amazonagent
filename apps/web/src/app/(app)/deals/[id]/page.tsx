"use client";

import { use } from "react";
import { useDeal, useApproveDeal, useRejectDeal } from "@/hooks/use-deals";
import { PageHeader } from "@/components/page-header";
import { ScoreBadge } from "@/components/score-badge";
import { StatusPill } from "@/components/status-pill";
import { EvidencePanel } from "@/components/evidence-panel";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";

export default function DealDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const { data: deal, isLoading } = useDeal(id);
  const approve = useApproveDeal();
  const reject = useRejectDeal();

  if (isLoading) return <div className="p-4">Loading...</div>;
  if (!deal) return <div className="p-4">Deal not found</div>;

  return (
    <div className="space-y-6">
      <PageHeader
        title={deal.title}
        description={`${deal.asin} \u00b7 ${deal.brand} \u00b7 ${deal.category}`}
        action={
          deal.status === "needs_review" ? (
            <div className="flex gap-2">
              <Button
                variant="outline"
                onClick={() => reject.mutate({ id: deal.id, reason: "Not a good fit" })}
                disabled={reject.isPending}
              >
                Reject
              </Button>
              <Button
                onClick={() => approve.mutate(deal.id)}
                disabled={approve.isPending}
              >
                Approve
              </Button>
            </div>
          ) : (
            <StatusPill status={deal.status} />
          )
        }
      />

      <div className="grid gap-4 md:grid-cols-5">
        <Card>
          <CardHeader className="pb-2"><CardTitle className="text-sm">Demand</CardTitle></CardHeader>
          <CardContent><ScoreBadge score={deal.scores.demand} /></CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2"><CardTitle className="text-sm">Competition</CardTitle></CardHeader>
          <CardContent><ScoreBadge score={deal.scores.competition} /></CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2"><CardTitle className="text-sm">Margin</CardTitle></CardHeader>
          <CardContent><ScoreBadge score={deal.scores.margin} /></CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2"><CardTitle className="text-sm">Risk</CardTitle></CardHeader>
          <CardContent><ScoreBadge score={deal.scores.risk} /></CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2"><CardTitle className="text-sm">Sourcing</CardTitle></CardHeader>
          <CardContent><ScoreBadge score={deal.scores.sourcing_feasibility} /></CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader><CardTitle>Overall Score</CardTitle></CardHeader>
        <CardContent>
          <div className="text-3xl font-bold">{deal.scores.overall.toFixed(1)}/10</div>
          <p className="mt-1 text-sm text-muted-foreground">
            Reviewer verdict: {deal.reviewer_verdict} (iteration {deal.iteration_count})
          </p>
        </CardContent>
      </Card>

      <div>
        <h2 className="mb-3 text-lg font-medium">Agent Evidence</h2>
        <EvidencePanel evidence={deal.evidence} />
      </div>
    </div>
  );
}
