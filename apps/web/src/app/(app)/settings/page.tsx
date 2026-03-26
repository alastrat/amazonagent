"use client";

import { useQuery } from "@tanstack/react-query";
import { apiClient } from "@/lib/api-client";
import { queryKeys } from "@/lib/query-keys";
import { PageHeader } from "@/components/page-header";

export default function SettingsPage() {
  const { data, isLoading, error } = useQuery({
    queryKey: queryKeys.scoring,
    queryFn: () => apiClient.getScoringConfig(),
  });

  return (
    <div className="space-y-6">
      <PageHeader title="Settings" description="Manage your tenant configuration" />

      <div className="space-y-4">
        <h2 className="text-lg font-medium">Scoring Configuration</h2>

        {isLoading && <p className="text-sm text-muted-foreground">Loading scoring config...</p>}

        {error && (
          <p className="text-sm text-destructive">
            Failed to load scoring config.
          </p>
        )}

        {data && (
          <div className="rounded-lg border">
            <div className="border-b px-4 py-3">
              <p className="text-sm font-medium">
                Weights{" "}
                <span className="ml-2 text-xs font-normal text-muted-foreground">
                  Version {data.version}
                </span>
              </p>
            </div>
            <ul className="divide-y text-sm">
              <li className="flex items-center justify-between px-4 py-3">
                <span className="text-muted-foreground">Demand</span>
                <span className="font-mono font-medium">{data.weights.demand}</span>
              </li>
              <li className="flex items-center justify-between px-4 py-3">
                <span className="text-muted-foreground">Competition</span>
                <span className="font-mono font-medium">{data.weights.competition}</span>
              </li>
              <li className="flex items-center justify-between px-4 py-3">
                <span className="text-muted-foreground">Margin</span>
                <span className="font-mono font-medium">{data.weights.margin}</span>
              </li>
              <li className="flex items-center justify-between px-4 py-3">
                <span className="text-muted-foreground">Risk</span>
                <span className="font-mono font-medium">{data.weights.risk}</span>
              </li>
              <li className="flex items-center justify-between px-4 py-3">
                <span className="text-muted-foreground">Sourcing</span>
                <span className="font-mono font-medium">{data.weights.sourcing}</span>
              </li>
            </ul>

            <div className="border-t px-4 py-3">
              <p className="text-sm font-medium">Thresholds</p>
            </div>
            <ul className="divide-y text-sm">
              <li className="flex items-center justify-between px-4 py-3">
                <span className="text-muted-foreground">Minimum overall score</span>
                <span className="font-mono font-medium">{data.thresholds.min_overall}</span>
              </li>
              <li className="flex items-center justify-between px-4 py-3">
                <span className="text-muted-foreground">Minimum per-dimension score</span>
                <span className="font-mono font-medium">{data.thresholds.min_per_dimension}</span>
              </li>
            </ul>
          </div>
        )}
      </div>

      <p className="text-sm text-muted-foreground">More settings coming soon.</p>
    </div>
  );
}
