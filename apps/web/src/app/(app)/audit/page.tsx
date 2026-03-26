"use client";

import { useQuery } from "@tanstack/react-query";
import { apiClient } from "@/lib/api-client";
import { queryKeys } from "@/lib/query-keys";
import { PageHeader } from "@/components/page-header";
import { EmptyState } from "@/components/empty-state";

function formatTimestamp(ts: string): string {
  const date = new Date(ts);
  return new Intl.DateTimeFormat("en-US", {
    year: "numeric",
    month: "short",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false,
  }).format(date);
}

export default function AuditPage() {
  const { data: events, isLoading, error } = useQuery({
    queryKey: queryKeys.events({}),
    queryFn: () => apiClient.getEvents(),
  });

  return (
    <div className="space-y-6">
      <PageHeader title="Audit Trail" description="Domain events and activity log" />

      {isLoading && <p className="text-sm text-muted-foreground">Loading events...</p>}

      {error && (
        <p className="text-sm text-destructive">Failed to load audit events.</p>
      )}

      {!isLoading && !error && events && events.length === 0 && (
        <EmptyState
          title="No events yet"
          description="Domain events will appear here as activity occurs in the system."
        />
      )}

      {events && events.length > 0 && (
        <div className="rounded-lg border">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b bg-muted/50">
                <th className="px-4 py-2 text-left font-medium">Timestamp</th>
                <th className="px-4 py-2 text-left font-medium">Event Type</th>
                <th className="px-4 py-2 text-left font-medium">Entity Type</th>
                <th className="px-4 py-2 text-left font-medium">Entity ID</th>
                <th className="px-4 py-2 text-left font-medium">Actor</th>
              </tr>
            </thead>
            <tbody>
              {events.map((event) => (
                <tr key={event.id} className="border-b last:border-0 hover:bg-muted/30">
                  <td className="px-4 py-2 font-mono text-xs text-muted-foreground whitespace-nowrap">
                    {formatTimestamp(event.timestamp)}
                  </td>
                  <td className="px-4 py-2">
                    <span className="rounded bg-muted px-1.5 py-0.5 font-mono text-xs">
                      {event.event_type}
                    </span>
                  </td>
                  <td className="px-4 py-2 text-muted-foreground">{event.entity_type}</td>
                  <td className="px-4 py-2 font-mono text-xs text-muted-foreground">
                    {event.entity_id}
                  </td>
                  <td className="px-4 py-2 font-mono text-xs text-muted-foreground">
                    {event.actor_id}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
