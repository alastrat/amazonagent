import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";

const eligibilityColors: Record<string, string> = {
  eligible: "bg-green-100 text-green-800 border-green-200",
  restricted: "bg-red-100 text-red-800 border-red-200",
  gated: "bg-yellow-100 text-yellow-800 border-yellow-200",
  unknown: "bg-gray-100 text-gray-700 border-gray-200",
};

interface EligibilityBadgeProps {
  status: string;
  className?: string;
}

export function EligibilityBadge({ status, className }: EligibilityBadgeProps) {
  const color = eligibilityColors[status] || eligibilityColors.unknown;
  return (
    <Badge
      variant="outline"
      className={cn(color, className)}
    >
      {status.replace(/_/g, " ")}
    </Badge>
  );
}
