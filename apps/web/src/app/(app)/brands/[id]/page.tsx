"use client";

import { use, useState } from "react";
import Link from "next/link";
import { useBrands, useBrandProducts } from "@/hooks/use-brands";
import { PageHeader } from "@/components/page-header";
import { EligibilityBadge } from "@/components/eligibility-badge";
import { EmptyState } from "@/components/empty-state";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";

export default function BrandDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const [search, setSearch] = useState("");
  const [eligibility, setEligibility] = useState("");

  const productParams: Record<string, string> = {};
  if (search) productParams.search = search;
  if (eligibility) productParams.eligibility = eligibility;

  // Fetch brand info from brands list (filtered by this brand id)
  const { data: brandsData, isLoading: brandsLoading } = useBrands({ brand_id: id });
  const brand = brandsData?.brands?.[0];

  const { data, isLoading } = useBrandProducts(id, productParams);

  if (brandsLoading) return <div className="p-4">Loading...</div>;
  if (!brand) return <div className="p-4">Brand not found</div>;

  return (
    <div className="space-y-6">
      <PageHeader
        title={brand.brand_name}
        description={`${brand.category || "Uncategorized"} \u00b7 ${brand.product_count} products`}
      />

      <div className="grid gap-4 md:grid-cols-4">
        <Card>
          <CardHeader className="pb-2"><CardTitle className="text-sm">Products</CardTitle></CardHeader>
          <CardContent><div className="text-2xl font-bold">{brand.product_count}</div></CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2"><CardTitle className="text-sm">High Margin</CardTitle></CardHeader>
          <CardContent><div className="text-2xl font-bold">{brand.high_margin_count}</div></CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2"><CardTitle className="text-sm">Avg Margin</CardTitle></CardHeader>
          <CardContent><div className="text-2xl font-bold">{brand.avg_margin.toFixed(1)}%</div></CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2"><CardTitle className="text-sm">Avg Sellers</CardTitle></CardHeader>
          <CardContent><div className="text-2xl font-bold">{brand.avg_sellers.toFixed(1)}</div></CardContent>
        </Card>
      </div>

      <div className="space-y-4">
        <h2 className="text-lg font-medium">Products</h2>

        <div className="flex flex-wrap gap-3">
          <Input
            placeholder="Search by ASIN or title..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="max-w-sm"
          />
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
        </div>

        {isLoading ? (
          <div>Loading...</div>
        ) : !data?.products || data.products.length === 0 ? (
          <EmptyState
            title="No products found"
            description="No products match the current filters for this brand."
          />
        ) : (
          <div className="rounded-lg border overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b bg-muted/50">
                  <th className="px-4 py-2 text-left font-medium">ASIN</th>
                  <th className="px-4 py-2 text-left font-medium">Title</th>
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
                      <Link
                        href={`https://amazon.com/dp/${product.asin}`}
                        target="_blank"
                        className="font-mono text-xs text-primary hover:underline"
                      >
                        {product.asin}
                      </Link>
                    </td>
                    <td className="px-4 py-2 max-w-[200px] truncate">{product.title}</td>
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
    </div>
  );
}
