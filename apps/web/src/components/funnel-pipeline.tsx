export interface FunnelStats {
  input_count: number;
  t0_deduped: number;
  t1_margin_killed: number;
  t2_brand_killed: number;
  t3_enrich_killed: number;
  survivor_count: number;
}

interface Stage {
  label: string;
  count: number;
  dropped: number;
  highlight?: boolean;
}

function buildStages(stats: FunnelStats): Stage[] {
  const afterDedup = stats.input_count - stats.t0_deduped;
  const afterMargin = afterDedup - stats.t1_margin_killed;
  const afterBrand = afterMargin - stats.t2_brand_killed;

  return [
    { label: "Input", count: stats.input_count, dropped: 0 },
    { label: "Deduped", count: afterDedup, dropped: stats.t0_deduped },
    { label: "Margin Pass", count: afterMargin, dropped: stats.t1_margin_killed },
    { label: "Brand Pass", count: afterBrand, dropped: stats.t2_brand_killed },
    {
      label: "Survivors",
      count: stats.survivor_count,
      dropped: stats.t3_enrich_killed,
      highlight: true,
    },
  ];
}

function FunnelStage({ stage, maxCount }: { stage: Stage; maxCount: number }) {
  const fillPct = maxCount > 0 ? (stage.count / maxCount) * 100 : 0;

  return (
    <div className="flex min-w-0 flex-1 flex-col gap-1">
      <span className="text-xs text-muted-foreground">{stage.label}</span>
      <span
        className={`text-lg font-bold ${stage.highlight ? "text-emerald-600" : ""}`}
      >
        {stage.count.toLocaleString()}
      </span>
      <div className="h-2 w-full rounded-full bg-muted">
        <div
          className={`h-2 rounded-full transition-all ${
            stage.highlight ? "bg-emerald-500" : "bg-primary"
          }`}
          style={{ width: `${fillPct}%` }}
        />
      </div>
      {stage.dropped > 0 && (
        <span className="text-xs font-medium text-red-600">
          -{stage.dropped.toLocaleString()} eliminated
        </span>
      )}
    </div>
  );
}

function Chevron() {
  return (
    <div className="flex shrink-0 items-center px-1 text-muted-foreground/50">
      <svg width="16" height="24" viewBox="0 0 16 24" fill="none">
        <path
          d="M2 2L12 12L2 22"
          stroke="currentColor"
          strokeWidth="2"
          strokeLinecap="round"
          strokeLinejoin="round"
        />
      </svg>
    </div>
  );
}

interface FunnelPipelineProps {
  stats: FunnelStats;
  className?: string;
}

export function FunnelPipeline({ stats, className }: FunnelPipelineProps) {
  const stages = buildStages(stats);
  const maxCount = stats.input_count;

  return (
    <div
      className={`flex flex-col gap-3 md:flex-row md:items-start md:gap-0 ${className ?? ""}`}
    >
      {stages.map((stage, i) => (
        <div key={stage.label} className="flex min-w-0 flex-1 items-start">
          <FunnelStage stage={stage} maxCount={maxCount} />
          {i < stages.length - 1 && (
            <div className="hidden md:flex">
              <Chevron />
            </div>
          )}
        </div>
      ))}
    </div>
  );
}
