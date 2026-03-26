const statusColors: Record<string, string> = {
  pending: "bg-gray-100 text-gray-700",
  running: "bg-blue-100 text-blue-700",
  completed: "bg-green-100 text-green-700",
  failed: "bg-red-100 text-red-700",
  discovered: "bg-gray-100 text-gray-700",
  analyzing: "bg-blue-100 text-blue-700",
  needs_review: "bg-amber-100 text-amber-700",
  approved: "bg-green-100 text-green-700",
  rejected: "bg-red-100 text-red-700",
  sourcing: "bg-purple-100 text-purple-700",
  live: "bg-emerald-100 text-emerald-700",
  archived: "bg-gray-100 text-gray-500",
};

export function StatusPill({ status }: { status: string }) {
  const color = statusColors[status] || "bg-gray-100 text-gray-700";
  return (
    <span className={`inline-flex rounded-full px-2 py-0.5 text-xs font-medium ${color}`}>
      {status.replace(/_/g, " ")}
    </span>
  );
}
