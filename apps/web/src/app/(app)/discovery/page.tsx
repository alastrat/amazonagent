"use client";

import { useState, useEffect } from "react";
import { useDiscovery, useUpdateDiscovery } from "@/hooks/use-discovery";
import { PageHeader } from "@/components/page-header";
import { MetricCard } from "@/components/metric-card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import type { DiscoveryCadence } from "@/lib/types";

function formatDateTime(iso?: string): string {
  if (!iso) return "—";
  return new Date(iso).toLocaleString();
}

export default function DiscoveryPage() {
  const { data: config, isPending: isLoading } = useDiscovery();
  const updateDiscovery = useUpdateDiscovery();

  const [enabled, setEnabled] = useState(false);
  const [categories, setCategories] = useState("");
  const [cadence, setCadence] = useState<DiscoveryCadence>("nightly");
  const [marketplace, setMarketplace] = useState("US");
  const [minMargin, setMinMargin] = useState("30");

  // Populate form once config is loaded
  useEffect(() => {
    if (!config) return;
    setEnabled(config.enabled);
    setCategories(config.categories.join(", "));
    setCadence(config.cadence);
    setMarketplace(config.baseline_criteria.marketplace ?? "US");
    setMinMargin(
      config.baseline_criteria.min_margin_pct != null
        ? String(config.baseline_criteria.min_margin_pct)
        : "30"
    );
  }, [config]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    await updateDiscovery.mutateAsync({
      enabled,
      categories: categories.split(",").map((c) => c.trim()).filter(Boolean),
      cadence,
      baseline_criteria: {
        keywords: config?.baseline_criteria.keywords ?? [],
        marketplace,
        min_margin_pct: parseFloat(minMargin) || undefined,
      },
    });
  };

  if (isLoading) {
    return (
      <div className="space-y-6">
        <PageHeader
          title="Discovery"
          description="Configure continuous background product sourcing"
        />
        <p className="text-sm text-muted-foreground">Loading configuration...</p>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <PageHeader
        title="Discovery"
        description="Configure continuous background product sourcing"
      />

      {/* Run status metrics */}
      <div className="grid gap-4 md:grid-cols-2">
        <MetricCard
          title="Last Run"
          value={formatDateTime(config?.last_run_at)}
          description="Most recent discovery cycle completion"
        />
        <MetricCard
          title="Next Run"
          value={formatDateTime(config?.next_run_at)}
          description="Scheduled start of next discovery cycle"
        />
      </div>

      {/* Configuration form */}
      <Card className="max-w-lg">
        <CardHeader>
          <CardTitle>Discovery Settings</CardTitle>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit} className="space-y-4">
            {/* Enable toggle */}
            <div className="flex items-center gap-3">
              <label className="relative inline-flex cursor-pointer items-center">
                <input
                  type="checkbox"
                  className="peer sr-only"
                  checked={enabled}
                  onChange={(e) => setEnabled(e.target.checked)}
                />
                <div className="peer h-5 w-9 rounded-full border bg-muted after:absolute after:left-[2px] after:top-[2px] after:h-4 after:w-4 after:rounded-full after:bg-white after:transition-all peer-checked:bg-primary peer-checked:after:translate-x-full" />
              </label>
              <span className="text-sm font-medium">
                {enabled ? "Discovery enabled" : "Discovery disabled"}
              </span>
            </div>

            {/* Categories */}
            <div>
              <label className="text-sm font-medium">
                Categories (comma-separated)
              </label>
              <Input
                value={categories}
                onChange={(e) => setCategories(e.target.value)}
                placeholder="kitchen, home fitness, pet supplies"
              />
              <p className="mt-1 text-xs text-muted-foreground">
                Amazon product categories to scan during each discovery run.
              </p>
            </div>

            {/* Cadence */}
            <div>
              <label className="text-sm font-medium">Run Cadence</label>
              <select
                value={cadence}
                onChange={(e) => setCadence(e.target.value as DiscoveryCadence)}
                className="flex h-9 w-full rounded-md border bg-transparent px-3 py-1 text-sm"
              >
                <option value="nightly">Nightly</option>
                <option value="twice_daily">Twice Daily</option>
                <option value="weekly">Weekly</option>
              </select>
            </div>

            {/* Baseline criteria */}
            <div>
              <label className="text-sm font-medium">Marketplace</label>
              <select
                value={marketplace}
                onChange={(e) => setMarketplace(e.target.value)}
                className="flex h-9 w-full rounded-md border bg-transparent px-3 py-1 text-sm"
              >
                <option value="US">US</option>
                <option value="UK">UK</option>
                <option value="EU">EU</option>
              </select>
            </div>

            <div>
              <label className="text-sm font-medium">Minimum Margin %</label>
              <Input
                type="number"
                value={minMargin}
                onChange={(e) => setMinMargin(e.target.value)}
                placeholder="30"
                min={0}
                max={100}
              />
              <p className="mt-1 text-xs text-muted-foreground">
                Products below this margin threshold are excluded from discovery results.
              </p>
            </div>

            <Button type="submit" disabled={updateDiscovery.isPending}>
              {updateDiscovery.isPending ? "Saving..." : "Save Settings"}
            </Button>

            {updateDiscovery.isSuccess && (
              <p className="text-sm text-green-600">Settings saved successfully.</p>
            )}
            {updateDiscovery.isError && (
              <p className="text-sm text-destructive">
                {updateDiscovery.error instanceof Error
                  ? updateDiscovery.error.message
                  : "Failed to save settings."}
              </p>
            )}
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
