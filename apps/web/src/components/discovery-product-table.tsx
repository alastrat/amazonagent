"use client";

import type { ProductDetail } from "@/lib/types";

interface Props {
  products: ProductDetail[];
  selectedNode: { id: string; name: string; type: string } | null;
}

export function DiscoveryProductTable({ products, selectedNode }: Props) {
  if (!selectedNode) {
    return (
      <div className="rounded-lg border border-dashed p-6 text-center text-sm text-muted-foreground">
        Click a category or brand in the tree to see products
      </div>
    );
  }

  const filtered = products
    .filter((p) => {
      if (selectedNode.type === "category") {
        return p.category === selectedNode.name;
      }
      if (selectedNode.type === "brand") {
        return p.brand === selectedNode.name;
      }
      return false;
    })
    .sort((a, b) => b.est_margin_pct - a.est_margin_pct);

  if (filtered.length === 0) {
    return (
      <div className="rounded-lg border border-dashed p-6 text-center text-sm text-muted-foreground">
        No products found for {selectedNode.type} &ldquo;{selectedNode.name}&rdquo;
      </div>
    );
  }

  return (
    <div className="space-y-2">
      <p className="text-sm text-muted-foreground">
        Showing {filtered.length} product{filtered.length !== 1 ? "s" : ""} for{" "}
        <span className="font-medium text-foreground">{selectedNode.name}</span>
      </p>
      <div className="rounded-lg border">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b bg-muted/50">
              <th className="px-4 py-2 text-left font-medium">ASIN</th>
              <th className="px-4 py-2 text-left font-medium">Title</th>
              <th className="px-4 py-2 text-left font-medium">Brand</th>
              <th className="px-4 py-2 text-left font-medium">Category</th>
              <th className="px-4 py-2 text-left font-medium">Price</th>
              <th className="px-4 py-2 text-left font-medium">Margin %</th>
              <th className="px-4 py-2 text-left font-medium">Sellers</th>
              <th className="px-4 py-2 text-left font-medium">Eligible</th>
            </tr>
          </thead>
          <tbody>
            {filtered.map((p) => (
              <tr key={p.asin} className="border-b last:border-0 hover:bg-muted/30">
                <td className="px-4 py-2 font-mono text-xs">{p.asin}</td>
                <td className="px-4 py-2 max-w-[200px] truncate">{p.title}</td>
                <td className="px-4 py-2 text-muted-foreground">{p.brand}</td>
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
