"use client";

import Link from "next/link";
import { useState } from "react";
import { useDeals } from "@/hooks/use-deals";
import { PageHeader } from "@/components/page-header";
import { StatusPill } from "@/components/status-pill";
import { ScoreBadge } from "@/components/score-badge";
import { EmptyState } from "@/components/empty-state";
import { Input } from "@/components/ui/input";

export default function DealsPage() {
  const [search, setSearch] = useState("");
  const [status, setStatus] = useState("");
  const params: Record<string, string> = {};
  if (search) params.search = search;
  if (status) params.status = status;

  const { data, isLoading } = useDeals(params);

  return (
    <div className="space-y-6">
      <PageHeader title="Deal Explorer" description={`${data?.total ?? 0} deals found`} />

      <div className="flex gap-3">
        <Input
          placeholder="Search by title, brand, or ASIN..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="max-w-sm"
        />
        <select
          value={status}
          onChange={(e) => setStatus(e.target.value)}
          className="flex h-9 rounded-md border bg-transparent px-3 py-1 text-sm"
        >
          <option value="">All statuses</option>
          <option value="needs_review">Needs Review</option>
          <option value="approved">Approved</option>
          <option value="rejected">Rejected</option>
        </select>
      </div>

      {isLoading ? (
        <div>Loading...</div>
      ) : !data?.deals || data.deals.length === 0 ? (
        <EmptyState title="No deals found" description="Adjust your filters or run a campaign to generate deals." />
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
              {data.deals.map((deal) => (
                <tr key={deal.id} className="border-b last:border-0 hover:bg-muted/30">
                  <td className="px-4 py-2">
                    <Link href={`/deals/${deal.id}`} className="font-mono text-xs text-primary hover:underline">
                      {deal.asin}
                    </Link>
                  </td>
                  <td className="px-4 py-2">{deal.title}</td>
                  <td className="px-4 py-2 text-muted-foreground">{deal.brand}</td>
                  <td className="px-4 py-2"><ScoreBadge score={Math.round(deal.scores.overall)} /></td>
                  <td className="px-4 py-2"><ScoreBadge score={deal.scores.margin} label="Margin" /></td>
                  <td className="px-4 py-2"><StatusPill status={deal.status} /></td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
