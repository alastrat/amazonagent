import Link from "next/link";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { StatusPill } from "@/components/status-pill";
import { FunnelPipeline, type FunnelStats } from "@/components/funnel-pipeline";

export interface ScanJob {
  id: string;
  type: string;
  status: string;
  total_items: number;
  processed: number;
  qualified: number;
  eliminated: number;
  started_at: string;
  completed_at?: string;
  metadata?: Record<string, unknown>;
}

interface ScanProgressCardProps {
  scan: ScanJob;
  funnelStats?: FunnelStats;
}

function formatElapsed(startedAt: string, completedAt?: string): string {
  const start = new Date(startedAt).getTime();
  const end = completedAt ? new Date(completedAt).getTime() : Date.now();
  const seconds = Math.floor((end - start) / 1000);

  if (seconds < 60) return `${seconds}s`;
  const minutes = Math.floor(seconds / 60);
  const remainingSeconds = seconds % 60;
  return `${minutes}m ${remainingSeconds}s`;
}

export function ScanProgressCard({ scan, funnelStats }: ScanProgressCardProps) {
  const progressPct =
    scan.total_items > 0
      ? Math.round((scan.processed / scan.total_items) * 100)
      : 0;
  const isCompleted = scan.status === "completed";
  const isFailed = scan.status === "failed";
  const isTerminal = isCompleted || isFailed;

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium">
          {scan.type.replace(/_/g, " ")}
        </CardTitle>
        <StatusPill status={scan.status} />
      </CardHeader>
      <CardContent className="space-y-3">
        {/* Progress bar */}
        <div className="space-y-1">
          <div className="flex justify-between text-xs text-muted-foreground">
            <span>
              {scan.processed.toLocaleString()} / {scan.total_items.toLocaleString()} items
            </span>
            <span>{progressPct}%</span>
          </div>
          <div className="h-2 w-full rounded-full bg-muted">
            <div
              className={`h-2 rounded-full transition-all ${
                isFailed
                  ? "bg-red-500"
                  : isCompleted
                    ? "bg-green-500"
                    : "bg-blue-500"
              }`}
              style={{ width: `${progressPct}%` }}
            />
          </div>
        </div>

        {/* Elapsed time */}
        <div className="flex items-center justify-between text-xs text-muted-foreground">
          <span>
            {isTerminal ? "Duration" : "Elapsed"}:{" "}
            {formatElapsed(scan.started_at, scan.completed_at)}
          </span>
          {isCompleted && (
            <span>
              {scan.qualified.toLocaleString()} qualified / {scan.eliminated.toLocaleString()} eliminated
            </span>
          )}
        </div>

        {/* Funnel pipeline (shown when completed) */}
        {isCompleted && funnelStats && (
          <div className="space-y-2 border-t pt-3">
            <FunnelPipeline stats={funnelStats} />
            <div className="flex justify-end">
              <Link
                href={`/scans/${scan.id}`}
                className="text-sm font-medium text-primary hover:underline"
              >
                View Results &rarr;
              </Link>
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
