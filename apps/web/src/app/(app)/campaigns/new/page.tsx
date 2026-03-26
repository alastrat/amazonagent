"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useCreateCampaign } from "@/hooks/use-campaigns";
import { PageHeader } from "@/components/page-header";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";

export default function NewCampaignPage() {
  const router = useRouter();
  const createCampaign = useCreateCampaign();
  const [keywords, setKeywords] = useState("");
  const [marketplace, setMarketplace] = useState("US");
  const [minMargin, setMinMargin] = useState("30");

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    await createCampaign.mutateAsync({
      type: "manual",
      trigger_type: "dashboard",
      criteria: {
        keywords: keywords.split(",").map((k) => k.trim()).filter(Boolean),
        min_margin_pct: parseFloat(minMargin) || undefined,
        marketplace,
      },
    });
    router.push("/campaigns");
  };

  return (
    <div className="space-y-6">
      <PageHeader title="New Campaign" description="Start a new product research campaign" />

      <Card className="max-w-lg">
        <CardHeader>
          <CardTitle>Campaign Criteria</CardTitle>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit} className="space-y-4">
            <div>
              <label className="text-sm font-medium">Keywords (comma-separated)</label>
              <Input
                value={keywords}
                onChange={(e) => setKeywords(e.target.value)}
                placeholder="kitchen gadgets, home fitness"
                required
              />
            </div>
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
              />
            </div>
            <Button type="submit" disabled={createCampaign.isPending}>
              {createCampaign.isPending ? "Creating..." : "Create Campaign"}
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
