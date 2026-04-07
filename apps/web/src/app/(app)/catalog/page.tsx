"use client";

import { useState } from "react";
import Link from "next/link";
import { useCatalogProducts, useCatalogStats } from "@/hooks/use-catalog";
import { PageHeader } from "@/components/page-header";
import { EligibilityBadge } from "@/components/eligibility-badge";
import { EmptyState } from "@/components/empty-state";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";

export default function CatalogPage() {
  const [search, setSearch] = useState("");
  const [category, setCategory] = useState("");
  const [eligibility, setEligibility] = useState("");
  const [source, setSource] = useState("");
  const [minMargin, setMinMargin] = useState("");
  const [maxSellers, setMaxSellers] = useState("");
  const [selected, setSelected] = useState<Set<string>>(new Set());

  const params: Record<string, string> = {};
  if (search) params.search = search;
  if (category) params.category = category;
  if (eligibility) params.eligibility = eligibility;
  if (source) params.source = source;
  if (minMargin) params.min_margin = minMargin;
  if (maxSellers) params.max_sellers = maxSellers;

  const { data, isLoading } = useCatalogProducts(params);
  const { data: stats } = useCatalogStats();

  function toggleSelect(id: string) {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  function toggleAll() {
    if (!data?.products) return;
    if (selected.size === data.products.length) {
      setSelected(new Set());
    } else {
      setSelected(new Set(data.products.map((p) => p.id)));
    }
  }

  return (
    <div className="space-y-6">
      <PageHeader
        title="Catalog Explorer"
        description={`${data?.total ?? 0} products${stats ? ` \u00b7 ${stats.eligible_count} eligible` : ""}`}
        action={
          selected.size > 0 ? (
            <Button disabled>Evaluate Selected ({selected.size})</Button>
          ) : undefined
        }
      />

      <div className="flex flex-wrap gap-3">
        <Input
          placeholder="Search by ASIN, title, or brand..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="max-w-sm"
        />
        <select
          value={category}
          onChange={(e) => setCategory(e.target.value)}
          className="flex h-9 rounded-md border bg-transparent px-3 py-1 text-sm"
        >
          <option value="">All categories</option>
        </select>
        <select
          value={eligibility}
          onChange={(e) => setEligibility(e.target.value)}
          className="flex h-9 rounded-md border bg-transparent px-3 py-1 text-sm"
        >
          <option value="">All eligibility</option>
          <option value="eligible">Eligible</option>
          <option value="restricted">Restricted</option>
          <option value="ineligible">Ineligible</option>
          <option value="unknown">Unknown</option>
        </select>
        <select
          value={source}
          onChange={(e) => setSource(e.target.value)}
          className="flex h-9 rounded-md border bg-transparent px-3 py-1 text-sm"
        >
          <option value="">All sources</option>
          <option value="pricelist">Price List</option>
          <option value="campaign">Campaign</option>
          <option value="manual">Manual</option>
        </select>
        <Input
          placeholder="Min margin %"
          type="number"
          value={minMargin}
          onChange={(e) => setMinMargin(e.target.value)}
          className="w-28"
        />
        <Input
          placeholder="Max sellers"
          type="number"
          value={maxSellers}
          onChange={(e) => setMaxSellers(e.target.value)}
          className="w-28"
        />
      </div>

      {isLoading ? (
        <div>Loading...</div>
      ) : !data?.products || data.products.length === 0 ? (
        <EmptyState
          title="No products found"
          description="Adjust your filters or upload a price list to discover products."
        />
      ) : (
        <div className="rounded-lg border overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b bg-muted/50">
                <th className="px-4 py-2 text-left">
                  <input
                    type="checkbox"
                    checked={selected.size === data.products.length}
                    onChange={toggleAll}
                  />
                </th>
                <th className="px-4 py-2 text-left font-medium">ASIN</th>
                <th className="px-4 py-2 text-left font-medium">Title</th>
                <th className="px-4 py-2 text-left font-medium">Brand</th>
                <th className="px-4 py-2 text-left font-medium">Category</th>
                <th className="px-4 py-2 text-right font-medium">Est. Price</th>
                <th className="px-4 py-2 text-right font-medium">Buy Box</th>
                <th className="px-4 py-2 text-right font-medium">Est Margin</th>
                <th className="px-4 py-2 text-right font-medium">Real Margin</th>
                <th className="px-4 py-2 text-right font-medium">BSR</th>
                <th className="px-4 py-2 text-right font-medium">Sellers</th>
                <th className="px-4 py-2 text-left font-medium">Eligibility</th>
                <th className="px-4 py-2 text-left font-medium">Last Updated</th>
              </tr>
            </thead>
            <tbody>
              {data.products.map((product) => (
                <tr key={product.id} className="border-b last:border-0 hover:bg-muted/30">
                  <td className="px-4 py-2">
                    <input
                      type="checkbox"
                      checked={selected.has(product.id)}
                      onChange={() => toggleSelect(product.id)}
                    />
                  </td>
                  <td className="px-4 py-2">
                    <Link
                      href={`https://amazon.com/dp/${product.asin}`}
                      target="_blank"
                      className="font-mono text-xs text-primary hover:underline"
                    >
                      {product.asin}
                    </Link>
                  </td>
                  <td className="px-4 py-2 max-w-[200px] truncate">{product.title}</td>
                  <td className="px-4 py-2 text-muted-foreground">
                    {product.brand_id ? (
                      <Link href={`/brands/${product.brand_id}`} className="hover:underline">
                        {product.brand_id}
                      </Link>
                    ) : (
                      "\u2014"
                    )}
                  </td>
                  <td className="px-4 py-2 text-muted-foreground">{product.category || "\u2014"}</td>
                  <td className="px-4 py-2 text-right">
                    {product.estimated_price != null ? `$${product.estimated_price.toFixed(2)}` : "\u2014"}
                  </td>
                  <td className="px-4 py-2 text-right">
                    {product.buy_box_price != null ? `$${product.buy_box_price.toFixed(2)}` : "\u2014"}
                  </td>
                  <td className="px-4 py-2 text-right">
                    {product.estimated_margin_pct != null ? `${product.estimated_margin_pct.toFixed(1)}%` : "\u2014"}
                  </td>
                  <td className="px-4 py-2 text-right">
                    {product.real_margin_pct != null ? `${product.real_margin_pct.toFixed(1)}%` : "\u2014"}
                  </td>
                  <td className="px-4 py-2 text-right">
                    {product.bsr_rank != null ? product.bsr_rank.toLocaleString() : "\u2014"}
                  </td>
                  <td className="px-4 py-2 text-right">{product.seller_count ?? "\u2014"}</td>
                  <td className="px-4 py-2">
                    <EligibilityBadge status={product.eligibility_status} />
                  </td>
                  <td className="px-4 py-2 text-muted-foreground text-xs">
                    {product.last_seen_at ? new Date(product.last_seen_at).toLocaleDateString() : "\u2014"}
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
