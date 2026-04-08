"use client";

import { useState } from "react";
import Link from "next/link";
import { useBrands } from "@/hooks/use-brands";
import { PageHeader } from "@/components/page-header";
import { EmptyState } from "@/components/empty-state";
import { Input } from "@/components/ui/input";

export default function BrandsPage() {
  const [category, setCategory] = useState("");
  const [minMargin, setMinMargin] = useState("");
  const [minProducts, setMinProducts] = useState("");

  const params: Record<string, string> = {};
  if (category) params.category = category;
  if (minMargin) params.min_margin = minMargin;
  if (minProducts) params.min_products = minProducts;

  const { data, isLoading } = useBrands(params);

  return (
    <div className="space-y-6">
      <PageHeader
        title="Brand Intelligence"
        description="Aggregated brand metrics across your catalog"
      />

      <div className="flex flex-wrap gap-3">
        <select
          value={category}
          onChange={(e) => setCategory(e.target.value)}
          className="flex h-9 rounded-md border bg-transparent px-3 py-1 text-sm"
        >
          <option value="">All categories</option>
        </select>
        <Input
          placeholder="Min avg margin %"
          type="number"
          value={minMargin}
          onChange={(e) => setMinMargin(e.target.value)}
          className="w-36"
        />
        <Input
          placeholder="Min products"
          type="number"
          value={minProducts}
          onChange={(e) => setMinProducts(e.target.value)}
          className="w-32"
        />
      </div>

      {isLoading ? (
        <div>Loading...</div>
      ) : !data?.brands || data.brands.length === 0 ? (
        <EmptyState
          title="No brands found"
          description="Upload a price list or run a campaign to start building brand intelligence."
        />
      ) : (
        <div className="rounded-lg border">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b bg-muted/50">
                <th className="px-4 py-2 text-left font-medium">Brand</th>
                <th className="px-4 py-2 text-left font-medium">Category</th>
                <th className="px-4 py-2 text-right font-medium">Products</th>
                <th className="px-4 py-2 text-right font-medium">High Margin</th>
                <th className="px-4 py-2 text-right font-medium">Avg Margin</th>
                <th className="px-4 py-2 text-right font-medium">Avg Sellers</th>
              </tr>
            </thead>
            <tbody>
              {data.brands.map((brand) => (
                <tr key={brand.brand_id} className="border-b last:border-0 hover:bg-muted/30">
                  <td className="px-4 py-2">
                    <Link href={`/brands/${brand.brand_id}`} className="text-primary hover:underline font-medium">
                      {brand.brand_name}
                    </Link>
                  </td>
                  <td className="px-4 py-2 text-muted-foreground">{brand.category || "\u2014"}</td>
                  <td className="px-4 py-2 text-right">{brand.product_count}</td>
                  <td className="px-4 py-2 text-right">{brand.high_margin_count}</td>
                  <td className="px-4 py-2 text-right">{brand.avg_margin.toFixed(1)}%</td>
                  <td className="px-4 py-2 text-right">{brand.avg_sellers.toFixed(1)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
