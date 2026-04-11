"use client";

import { useCallback } from "react";
import { Button } from "@/components/ui/button";
import type { ProductDetail } from "@/lib/types";

interface Props {
  products: ProductDetail[];
  selectedNode: { id: string; name: string; type: string } | null;
}

export function DiscoveryProductTable({ products, selectedNode }: Props) {
  if (!selectedNode) {
    return (
      <div className="rounded-lg border border-dashed p-6 text-center text-sm text-muted-foreground">
        Click a category, subcategory, or brand in the tree to see products
      </div>
    );
  }

  // Filter by selected node
  const filtered = products
    .filter((p) => {
      if (selectedNode.type === "category") {
        return p.category === selectedNode.name;
      }
      if (selectedNode.type === "subcategory") {
        return p.subcategory === selectedNode.name;
      }
      if (selectedNode.type === "brand") {
        return p.brand === selectedNode.name;
      }
      return false;
    })
    .sort((a, b) => b.est_margin_pct - a.est_margin_pct);

  // Deduplicate by ASIN
  const seen = new Set<string>();
  const unique = filtered.filter((p) => {
    if (seen.has(p.asin)) return false;
    seen.add(p.asin);
    return true;
  });

  const downloadCSV = useCallback(() => {
    const headers = ["ASIN", "Title", "Brand", "Subcategory", "Category", "Price", "Margin %", "Sellers", "Eligible"];
    const rows = unique.map((p) => [
      p.asin,
      `"${(p.title || "").replace(/"/g, '""')}"`,
      `"${(p.brand || "").replace(/"/g, '""')}"`,
      `"${(p.subcategory || "").replace(/"/g, '""')}"`,
      `"${(p.category || "").replace(/"/g, '""')}"`,
      p.price.toFixed(2),
      p.est_margin_pct.toFixed(1),
      String(p.seller_count),
      p.eligible ? "Yes" : "No",
    ]);
    const csv = [headers.join(","), ...rows.map((r) => r.join(","))].join("\n");
    const blob = new Blob([csv], { type: "text/csv" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `products-${selectedNode.name.replace(/\s+/g, "-").toLowerCase()}.csv`;
    a.click();
    URL.revokeObjectURL(url);
  }, [unique, selectedNode]);

  if (unique.length === 0) {
    return (
      <div className="rounded-lg border border-dashed p-6 text-center text-sm text-muted-foreground">
        No products found for {selectedNode.type} &ldquo;{selectedNode.name}&rdquo;
      </div>
    );
  }

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between">
        <p className="text-sm text-muted-foreground">
          Showing {unique.length} product{unique.length !== 1 ? "s" : ""} for{" "}
          <span className="font-medium text-foreground">{selectedNode.name}</span>
        </p>
        <Button variant="outline" size="sm" onClick={downloadCSV}>
          Download CSV
        </Button>
      </div>
      <div className="rounded-lg border overflow-auto max-h-[400px]">
        <table className="w-full text-sm">
          <thead className="sticky top-0 bg-muted/90 backdrop-blur-sm">
            <tr className="border-b">
              <th className="px-4 py-2 text-left font-medium">ASIN</th>
              <th className="px-4 py-2 text-left font-medium">Title</th>
              <th className="px-4 py-2 text-left font-medium">Brand</th>
              <th className="px-4 py-2 text-left font-medium">Subcategory</th>
              <th className="px-4 py-2 text-left font-medium">Category</th>
              <th className="px-4 py-2 text-left font-medium">Price</th>
              <th className="px-4 py-2 text-left font-medium">Margin %</th>
              <th className="px-4 py-2 text-left font-medium">Sellers</th>
              <th className="px-4 py-2 text-left font-medium">Eligible</th>
            </tr>
          </thead>
          <tbody>
            {unique.map((p) => (
              <tr key={p.asin} className="border-b last:border-0 hover:bg-muted/30">
                <td className="px-4 py-2 font-mono text-xs">{p.asin}</td>
                <td className="px-4 py-2 max-w-[200px] truncate">{p.title}</td>
                <td className="px-4 py-2 text-muted-foreground">{p.brand || "—"}</td>
                <td className="px-4 py-2 text-muted-foreground">{p.subcategory || "—"}</td>
                <td className="px-4 py-2 text-muted-foreground">{p.category}</td>
                <td className="px-4 py-2">${p.price.toFixed(2)}</td>
                <td className="px-4 py-2">{p.est_margin_pct.toFixed(1)}%</td>
                <td className="px-4 py-2">{p.seller_count}</td>
                <td className="px-4 py-2">
                  {p.eligible ? (
                    <span className="inline-flex rounded-full bg-green-100 px-2 py-0.5 text-xs font-medium text-green-700">
                      Yes
                    </span>
                  ) : (
                    <span className="inline-flex rounded-full bg-red-100 px-2 py-0.5 text-xs font-medium text-red-700">
                      No
                    </span>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
