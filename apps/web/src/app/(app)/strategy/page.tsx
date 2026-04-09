"use client";

import { useActiveStrategy, useStrategyVersions, useActivateStrategyVersion as useActivateVersion, useRollbackStrategyVersion as useRollbackVersion } from "@/hooks/use-strategy";
import { PageHeader } from "@/components/page-header";
import { StatusPill } from "@/components/status-pill";
import { EmptyState } from "@/components/empty-state";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import Link from "next/link";

function ProgressPct({ value }: { value: number }) {
  const clamped = Math.min(100, Math.max(0, value));
  return (
    <div className="flex items-center gap-2">
      <div className="h-2 flex-1 overflow-hidden rounded-full bg-muted">
        <div
          className="h-full rounded-full bg-primary transition-all"
          style={{ width: `${clamped}%` }}
        />
      </div>
      <span className="text-xs font-medium text-muted-foreground">{clamped.toFixed(0)}%</span>
    </div>
  );
}

function daysRemaining(timeframe: string): number {
  const target = new Date(timeframe);
  const now = new Date();
  const diff = target.getTime() - now.getTime();
  return Math.max(0, Math.ceil(diff / (1000 * 60 * 60 * 24)));
}

export default function StrategyPage() {
  const { data: active, isLoading: activeLoading } = useActiveStrategy();
  const { data: versions, isLoading: versionsLoading } = useStrategyVersions();
  const activateVersion = useActivateVersion();
  const rollbackVersion = useRollbackVersion();

  const isLoading = activeLoading || versionsLoading;

  if (isLoading) return <div className="p-4">Loading...</div>;

  return (
    <div className="space-y-6">
      <PageHeader
        title="Strategy"
        description="Manage your sourcing strategy and goals"
        action={
          <Link href="/onboarding">
            <Button variant="outline">New Assessment</Button>
          </Link>
        }
      />

      {/* Active Strategy Card */}
      {active ? (
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-3">
              <span>Active Strategy v{active.version_number}</span>
              <StatusPill status={active.status} />
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            {active.goals && active.goals.length > 0 ? (
              active.goals.map((goal: any, i: number) => (
                <div key={i} className="space-y-2 rounded-lg border p-3">
                  <div className="flex items-center justify-between">
                    <span className="text-sm font-medium">{goal.type}</span>
                    <span className="text-xs text-muted-foreground">
                      {daysRemaining(goal.timeframe)} days remaining
                    </span>
                  </div>
                  <div className="text-xs text-muted-foreground">
                    Target: ${goal.target_amount?.toLocaleString()}
                  </div>
                  <ProgressPct value={goal.progress ?? 0} />
                </div>
              ))
            ) : (
              <p className="text-sm text-muted-foreground">No goals defined for this strategy.</p>
            )}
          </CardContent>
        </Card>
      ) : (
        <EmptyState
          title="No active strategy"
          description="Complete onboarding to create your first sourcing strategy."
          action={
            <Link href="/onboarding">
              <Button>Get Started</Button>
            </Link>
          }
        />
      )}

      {/* Version History Table */}
      {versions && versions.length > 0 && (
        <div>
          <h2 className="mb-3 text-lg font-medium">Version History</h2>
          <div className="rounded-lg border">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b bg-muted/50">
                  <th className="px-4 py-2 text-left font-medium">Version</th>
                  <th className="px-4 py-2 text-left font-medium">Status</th>
                  <th className="px-4 py-2 text-left font-medium">Change Reason</th>
                  <th className="px-4 py-2 text-left font-medium">Created By</th>
                  <th className="px-4 py-2 text-left font-medium">Created At</th>
                  <th className="px-4 py-2 text-left font-medium">Actions</th>
                </tr>
              </thead>
              <tbody>
                {versions.map((v: any) => (
                  <tr key={v.id} className="border-b last:border-0 hover:bg-muted/30">
                    <td className="px-4 py-2 font-mono text-xs">v{v.version}</td>
                    <td className="px-4 py-2">
                      <StatusPill status={v.status} />
                    </td>
                    <td className="px-4 py-2 text-muted-foreground">{v.change_reason ?? "-"}</td>
                    <td className="px-4 py-2 text-muted-foreground">{v.created_by ?? "-"}</td>
                    <td className="px-4 py-2 text-muted-foreground">
                      {new Date(v.created_at).toLocaleDateString()}
                    </td>
                    <td className="px-4 py-2">
                      {v.status !== "active" && (
                        <div className="flex gap-2">
                          <Button
                            size="xs"
                            variant="outline"
                            onClick={() => activateVersion.mutate(v.id)}
                            disabled={activateVersion.isPending}
                          >
                            Activate
                          </Button>
                          <Button
                            size="xs"
                            variant="ghost"
                            onClick={() => rollbackVersion.mutate(v.id)}
                            disabled={rollbackVersion.isPending}
                          >
                            Rollback
                          </Button>
                        </div>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  );
}
