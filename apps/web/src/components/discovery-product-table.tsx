"use client";

import { useState, useCallback, useMemo } from "react";
import { Button } from "@/components/ui/button";
import type { ProductDetail, EligibilityStatus } from "@/lib/types";

type StatusFilter = EligibilityStatus | "all";

interface Props {
  products: ProductDetail[];
  selectedNode: { id: string; name: string; type: string } | null;
  showAllByDefault?: boolean; // true on reveal step — show all products without requiring node click
}

function getStatus(p: ProductDetail): EligibilityStatus {
  return p.eligibility_status || (p.eligible ? "eligible" : "restricted");
}

export function DiscoveryProductTable({ products, selectedNode, showAllByDefault = false }: Props) {
  const [statusFilter, setStatusFilter] = useState<StatusFilter>("all");

  const showPrompt = !selectedNode && !showAllByDefault;

  // Filter by selected node (or show all if showAllByDefault)
  const nodeFiltered = useMemo(() => {
    if (showPrompt) return [];
    return products
      .filter((p) => {
        if (!selectedNode) return true; // show all
        if (selectedNode.type === "category") return p.category === selectedNode.name;
        if (selectedNode.type === "subcategory") return p.subcategory === selectedNode.name;
        if (selectedNode.type === "brand") return p.brand === selectedNode.name;
        return false;
      })
      .sort((a, b) => b.est_margin_pct - a.est_margin_pct);
  }, [products, selectedNode, showPrompt]);

  // Deduplicate by ASIN
  const unique = useMemo(() => {
    const seen = new Set<string>();
    return nodeFiltered.filter((p) => {
      if (seen.has(p.asin)) return false;
      seen.add(p.asin);
      return true;
    });
  }, [nodeFiltered]);

  // Status filter counts
  const counts = useMemo(() => {
    const c = { all: 0, eligible: 0, ungatable: 0, restricted: 0 };
    for (const p of unique) {
      c.all++;
      const s = getStatus(p);
      if (s === "eligible") c.eligible++;
      else if (s === "ungatable") c.ungatable++;
      else c.restricted++;
    }
    return c;
  }, [unique]);

  // Apply status filter
  const displayed = statusFilter === "all"
    ? unique
    : unique.filter((p) => getStatus(p) === statusFilter);

  // CSV cell sanitizer — guards against formula injection (=, +, -, @)
  const csvCell = (value: unknown) => {
    const raw = String(value ?? "");
    const guarded = /^[=+\-@]/.test(raw) ? `'${raw}` : raw;
    return `"${guarded.replace(/"/g, '""')}"`;
  };

  const downloadCSV = useCallback(() => {
    const headers = ["ASIN", "Title", "Brand", "Subcategory", "Category", "Price", "Margin %", "Sellers", "Status", "Approval URL"];
    const rows = displayed.map((p) => [
      csvCell(p.asin),
      csvCell(p.title),
      csvCell(p.brand),
      csvCell(p.subcategory),
      csvCell(p.category),
      csvCell(p.price.toFixed(2)),
      csvCell(p.est_margin_pct.toFixed(1)),
      csvCell(p.seller_count),
      csvCell(getStatus(p)),
      csvCell(p.approval_url || ""),
    ]);
    const csv = [headers.join(","), ...rows.map((r) => r.join(","))].join("\n");
    const blob = new Blob([csv], { type: "text/csv" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `products-${selectedNode ? selectedNode.name.replace(/\s+/g, "-").toLowerCase() : "all"}.csv`;
    a.click();
    URL.revokeObjectURL(url);
  }, [displayed, selectedNode]);

  if (showPrompt) {
    return (
      <div className="rounded-lg border border-dashed p-6 text-center text-sm text-muted-foreground">
        Click a category, subcategory, or brand in the tree to see products
      </div>
    );
  }

  if (unique.length === 0) {
    return (
      <div className="rounded-lg border border-dashed p-6 text-center text-sm text-muted-foreground">
        {selectedNode
          ? <>No products found for {selectedNode.type} &ldquo;{selectedNode.name}&rdquo;</>
          : "No products found"}
      </div>
    );
  }

  const filterTabs: { key: StatusFilter; label: string; count: number; color?: string }[] = [
    { key: "all", label: "All", count: counts.all },
    { key: "eligible", label: "Eligible", count: counts.eligible, color: "text-green-500" },
    { key: "ungatable", label: "Can Apply", count: counts.ungatable, color: "text-amber-500" },
    { key: "restricted", label: "Restricted", count: counts.restricted, color: "text-red-500" },
  ];

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <p className="text-sm text-muted-foreground">
          Showing {displayed.length} product{displayed.length !== 1 ? "s" : ""}
          {selectedNode && (
            <> for <span className="font-medium text-foreground">{selectedNode.name}</span></>
          )}
        </p>
        <Button variant="outline" size="sm" onClick={downloadCSV}>
          Download CSV
        </Button>
      </div>

      {/* Status filter tabs */}
      <div className="inline-flex rounded-lg bg-muted/50 p-1 gap-0.5">
        {filterTabs.map((tab) => (
          <button
            key={tab.key}
            onClick={() => setStatusFilter(tab.key)}
            className={`px-3 py-1.5 rounded-md text-xs font-medium transition-colors ${
              statusFilter === tab.key
                ? "bg-background shadow-sm text-foreground"
                : "text-muted-foreground hover:text-foreground"
            }`}
          >
            <span className={statusFilter !== tab.key ? tab.color : undefined}>
              {tab.label}
            </span>
            <span className="ml-1 opacity-60">{tab.count}</span>
          </button>
        ))}
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
              <th className="px-4 py-2 text-left font-medium">Status</th>
            </tr>
          </thead>
          <tbody>
            {displayed.map((p) => (
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
                  {(() => {
                    const status = getStatus(p);
                    if (status === "eligible") {
                      return (
                        <span className="inline-flex rounded-full bg-green-100 px-2 py-0.5 text-xs font-medium text-green-700 dark:bg-green-950 dark:text-green-400">
                          Eligible
                        </span>
                      );
                    }
                    if (status === "ungatable") {
                      return (
                        <span className="inline-flex items-center gap-1">
                          <span className="inline-flex rounded-full bg-amber-100 px-2 py-0.5 text-xs font-medium text-amber-700 dark:bg-amber-950 dark:text-amber-400">
                            Apply
                          </span>
                          {p.approval_url && (
                            <a
                              href={p.approval_url}
                              target="_blank"
                              rel="noopener noreferrer"
                              className="text-xs text-blue-600 hover:underline dark:text-blue-400"
                            >
                              Request
                            </a>
                          )}
                        </span>
                      );
                    }
                    return (
                      <span className="inline-flex rounded-full bg-red-100 px-2 py-0.5 text-xs font-medium text-red-700 dark:bg-red-950 dark:text-red-400">
                        Restricted
                      </span>
                    );
                  })()}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
