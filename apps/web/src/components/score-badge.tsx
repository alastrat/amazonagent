interface ScoreBadgeProps {
  score: number;
  label?: string;
}

export function ScoreBadge({ score, label }: ScoreBadgeProps) {
  const color =
    score >= 8
      ? "bg-green-100 text-green-800"
      : score >= 6
        ? "bg-yellow-100 text-yellow-800"
        : "bg-red-100 text-red-800";

  return (
    <span className={`inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs font-medium ${color}`}>
      {label && <span className="text-muted-foreground">{label}</span>}
      {score}/10
    </span>
  );
}
