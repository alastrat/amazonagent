"use client";

import Link from "next/link";
import { usePendingSuggestions, useAcceptSuggestion, useDismissSuggestion } from "@/hooks/use-suggestions";
import { PageHeader } from "@/components/page-header";
import { EmptyState } from "@/components/empty-state";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";

export default function SuggestionsPage() {
  const { data: suggestions, isLoading } = usePendingSuggestions();
  const acceptSuggestion = useAcceptSuggestion();
  const dismissSuggestion = useDismissSuggestion();

  const pendingCount = suggestions?.filter((s: any) => s.status === "pending").length ?? 0;

  return (
    <div className="space-y-6">
      <PageHeader
        title="Suggestions"
        description={`${pendingCount} pending suggestion${pendingCount !== 1 ? "s" : ""}`}
      />

      {isLoading ? (
        <div>Loading...</div>
      ) : !suggestions || suggestions.length === 0 ? (
        <EmptyState
          title="No suggestions yet"
          description="Suggestions will appear here as the discovery engine finds products matching your strategy."
        />
      ) : (
        <div className="grid gap-4 md:grid-cols-2">
          {suggestions.map((suggestion: any) => (
            <Card key={suggestion.id}>
              <CardHeader>
                <CardTitle className="flex items-center justify-between">
                  <span className="truncate">{suggestion.title}</span>
                  <span className="font-mono text-xs text-muted-foreground">{suggestion.asin}</span>
                </CardTitle>
              </CardHeader>
              <CardContent className="space-y-3">
                <div className="grid grid-cols-2 gap-2 text-sm">
                  <div>
                    <span className="text-muted-foreground">Brand:</span>{" "}
                    <span className="font-medium">{suggestion.brand}</span>
                  </div>
                  <div>
                    <span className="text-muted-foreground">Category:</span>{" "}
                    <span className="font-medium">{suggestion.category}</span>
                  </div>
                  <div>
                    <span className="text-muted-foreground">Price:</span>{" "}
                    <span className="font-medium">${suggestion.price?.toFixed(2)}</span>
                  </div>
                  <div>
                    <span className="text-muted-foreground">Margin:</span>{" "}
                    <span className="font-medium">{suggestion.margin_pct?.toFixed(1)}%</span>
                  </div>
                </div>

                {suggestion.reason && (
                  <p className="text-xs text-muted-foreground">{suggestion.reason}</p>
                )}

                {suggestion.status === "accepted" && suggestion.deal_id ? (
                  <div className="rounded-md bg-green-50 px-3 py-2 text-xs text-green-700">
                    Accepted -{" "}
                    <Link href={`/deals/${suggestion.deal_id}`} className="font-medium underline">
                      View Deal
                    </Link>
                  </div>
                ) : suggestion.status === "dismissed" ? (
                  <div className="rounded-md bg-muted px-3 py-2 text-xs text-muted-foreground">
                    Dismissed
                  </div>
                ) : (
                  <div className="flex gap-2">
                    <Button
                      size="sm"
                      onClick={() => acceptSuggestion.mutate(suggestion.id)}
                      disabled={acceptSuggestion.isPending}
                    >
                      Accept
                    </Button>
                    <Button
                      size="sm"
                      variant="outline"
                      onClick={() => dismissSuggestion.mutate(suggestion.id)}
                      disabled={dismissSuggestion.isPending}
                    >
                      Dismiss
                    </Button>
                  </div>
                )}
              </CardContent>
            </Card>
          ))}
        </div>
      )}
    </div>
  );
}
