"use client";

import Link from "next/link";
import { useCampaigns } from "@/hooks/use-campaigns";
import { PageHeader } from "@/components/page-header";
import { StatusPill } from "@/components/status-pill";
import { EmptyState } from "@/components/empty-state";
import { Button } from "@/components/ui/button";

export default function CampaignsPage() {
  const { data: campaigns, isLoading } = useCampaigns();

  if (isLoading) return <div className="p-4">Loading...</div>;

  return (
    <div className="space-y-6">
      <PageHeader
        title="Campaigns"
        description="Research campaigns and discovery runs"
        action={
          <Link href="/campaigns/new">
            <Button>New Campaign</Button>
          </Link>
        }
      />

      {!campaigns || campaigns.length === 0 ? (
        <EmptyState
          title="No campaigns yet"
          description="Create your first campaign to start discovering profitable products."
          action={
            <Link href="/campaigns/new">
              <Button>Create Campaign</Button>
            </Link>
          }
        />
      ) : (
        <div className="rounded-lg border">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b bg-muted/50">
                <th className="px-4 py-2 text-left font-medium">Type</th>
                <th className="px-4 py-2 text-left font-medium">Keywords</th>
                <th className="px-4 py-2 text-left font-medium">Status</th>
                <th className="px-4 py-2 text-left font-medium">Created</th>
              </tr>
            </thead>
            <tbody>
              {campaigns.map((c) => (
                <tr key={c.id} className="border-b last:border-0">
                  <td className="px-4 py-2">
                    <Link href={`/campaigns/${c.id}`} className="text-primary hover:underline">
                      {c.type}
                    </Link>
                  </td>
                  <td className="px-4 py-2">{c.criteria.keywords?.join(", ") || "\u2014"}</td>
                  <td className="px-4 py-2"><StatusPill status={c.status} /></td>
                  <td className="px-4 py-2 text-muted-foreground">{new Date(c.created_at).toLocaleDateString()}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
