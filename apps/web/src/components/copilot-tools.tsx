"use client";

/**
 * CopilotKit tool renderers for the FBA Concierge.
 *
 * Backend tools (get_eligible_products, check_eligibility, etc.) are executed
 * by the Go backend via the AG-UI protocol. Their results are returned as text
 * in the SSE stream and rendered as markdown by CopilotKit.
 *
 * This component is a placeholder for future frontend-only actions
 * (e.g., UI interactions, chart rendering, approval buttons).
 */
export function CopilotToolRenderers() {
  return <></>;
}
  // ── get_assessment_summary ────────────────────────────────────────────
  useCopilotAction({
    name: "get_assessment_summary",
    description: "Get assessment summary stats",
    parameters: [],
    render: ({ status, result }) => {
      if (status === "inProgress") {
        return (
          <div className="animate-pulse text-sm text-muted-foreground py-2">
            Loading assessment summary...
          </div>
        );
      }
      const data = result as Record<string, number> | undefined;
      if (!data) return <></>;
      return (
        <div className="grid grid-cols-3 gap-2 my-2">
          <StatCard
            label="Eligible"
            value={data.eligible_products ?? data.eligible ?? 0}
            color="text-green-600"
          />
          <StatCard
            label="Ungatable"
            value={data.ungatable_products ?? data.ungatable ?? 0}
            color="text-amber-600"
          />
          <StatCard
            label="Restricted"
            value={data.restricted_products ?? data.restricted ?? 0}
            color="text-red-600"
          />
        </div>
      );
    },
  });

  // ── get_eligible_products ─────────────────────────────────────────────
  useCopilotAction({
    name: "get_eligible_products",
    description: "Get eligible products",
    parameters: [],
    render: ({ status, result }) => {
      if (status === "inProgress") {
        return (
          <div className="animate-pulse text-sm text-muted-foreground py-2">
            Fetching eligible products...
          </div>
        );
      }
      const products = (result as ProductResult[] | undefined) ?? [];
      if (products.length === 0) return <></>;
      return <ProductGrid products={products} variant="eligible" />;
    },
  });

  // ── get_ungatable_products ────────────────────────────────────────────
  useCopilotAction({
    name: "get_ungatable_products",
    description: "Get ungatable products",
    parameters: [],
    render: ({ status, result }) => {
      if (status === "inProgress") {
        return (
          <div className="animate-pulse text-sm text-muted-foreground py-2">
            Fetching ungatable products...
          </div>
        );
      }
      const products = (result as ProductResult[] | undefined) ?? [];
      if (products.length === 0) return <></>;
      return <ProductGrid products={products} variant="ungatable" />;
    },
  });

  // ── check_eligibility ─────────────────────────────────────────────────
  useCopilotAction({
    name: "check_eligibility",
    description: "Check eligibility for a product",
    parameters: [{ name: "asin", type: "string", description: "ASIN to check" }],
    render: ({ status, result }) => {
      if (status === "inProgress") {
        return (
          <div className="animate-pulse text-sm text-muted-foreground py-2">
            Checking eligibility...
          </div>
        );
      }
      const data = result as EligibilityResult | undefined;
      if (!data) return <></>;
      const colorMap: Record<string, string> = {
        eligible: "border-green-300 bg-green-50 dark:bg-green-950/30",
        ungatable: "border-amber-300 bg-amber-50 dark:bg-amber-950/30",
        restricted: "border-red-300 bg-red-50 dark:bg-red-950/30",
      };
      const badgeColor: Record<string, string> = {
        eligible: "bg-green-100 text-green-700",
        ungatable: "bg-amber-100 text-amber-700",
        restricted: "bg-red-100 text-red-700",
      };
      const status_ = data.eligibility_status ?? data.status ?? "restricted";
      return (
        <Card className={`my-2 ${colorMap[status_] ?? ""}`}>
          <CardContent className="py-3">
            <div className="flex items-center justify-between">
              <div>
                <p className="font-medium text-sm">{data.asin}</p>
                {data.title && (
                  <p className="text-xs text-muted-foreground truncate max-w-[280px]">
                    {data.title}
                  </p>
                )}
              </div>
              <span
                className={`rounded-full px-2 py-0.5 text-xs font-medium ${badgeColor[status_] ?? "bg-gray-100 text-gray-700"}`}
              >
                {status_}
              </span>
            </div>
            {data.approval_url && (
              <a
                href={data.approval_url}
                target="_blank"
                rel="noopener noreferrer"
                className="mt-2 inline-block text-xs text-primary underline"
              >
                Apply for approval
              </a>
            )}
          </CardContent>
        </Card>
      );
    },
  });

  // ── search_products ───────────────────────────────────────────────────
  useCopilotAction({
    name: "search_products",
    description: "Search products",
    parameters: [
      { name: "query", type: "string", description: "Search query" },
    ],
    render: ({ status, result }) => {
      if (status === "inProgress") {
        return (
          <div className="animate-pulse text-sm text-muted-foreground py-2">
            Searching products...
          </div>
        );
      }
      const products = (result as ProductResult[] | undefined) ?? [];
      if (products.length === 0) return <></>;
      return <ProductGrid products={products} variant="search" />;
    },
  });

  return <></>;
}

// ── Shared sub-components ─────────────────────────────────────────────────

interface ProductResult {
  asin?: string;
  title?: string;
  brand?: string;
  category?: string;
  price?: number;
  buy_box_price?: number;
  est_margin_pct?: number;
  estimated_margin_pct?: number;
  seller_count?: number;
  bsr_rank?: number;
  eligibility_status?: string;
  approval_url?: string;
}

interface EligibilityResult {
  asin: string;
  title?: string;
  eligibility_status?: string;
  status?: string;
  approval_url?: string;
}

function StatCard({
  label,
  value,
  color,
}: {
  label: string;
  value: number;
  color: string;
}) {
  return (
    <div className="rounded-lg border bg-muted/30 p-3 text-center">
      <div className={`text-xl font-bold ${color}`}>{value}</div>
      <div className="text-[10px] text-muted-foreground mt-0.5">{label}</div>
    </div>
  );
}

function ProductGrid({
  products,
  variant,
}: {
  products: ProductResult[];
  variant: "eligible" | "ungatable" | "search";
}) {
  return (
    <div className="grid grid-cols-1 gap-2 my-2">
      {products.slice(0, 10).map((p, i) => {
        const margin = p.est_margin_pct ?? p.estimated_margin_pct;
        const price = p.buy_box_price ?? p.price;
        return (
          <div
            key={p.asin ?? i}
            className="rounded-lg border bg-card p-3 text-sm space-y-1"
          >
            <div className="flex items-start justify-between gap-2">
              <div className="min-w-0 flex-1">
                <p className="font-medium text-xs truncate">
                  {p.title ?? p.asin ?? "Unknown"}
                </p>
                <div className="flex items-center gap-2 mt-0.5 text-[10px] text-muted-foreground">
                  {p.asin && <span>{p.asin}</span>}
                  {p.brand && (
                    <>
                      <span className="text-muted-foreground/40">|</span>
                      <span>{p.brand}</span>
                    </>
                  )}
                </div>
              </div>
              <div className="flex items-center gap-1.5 shrink-0">
                {margin != null && (
                  <span
                    className={`rounded-full px-1.5 py-0.5 text-[10px] font-medium ${
                      margin >= 30
                        ? "bg-green-100 text-green-700"
                        : margin >= 15
                          ? "bg-amber-100 text-amber-700"
                          : "bg-red-100 text-red-700"
                    }`}
                  >
                    {margin.toFixed(0)}%
                  </span>
                )}
                {variant === "ungatable" && (
                  <span className="rounded-full bg-amber-100 text-amber-700 px-1.5 py-0.5 text-[10px] font-medium">
                    Apply
                  </span>
                )}
              </div>
            </div>
            <div className="flex gap-3 text-[10px] text-muted-foreground">
              {price != null && <span>${price.toFixed(2)}</span>}
              {p.seller_count != null && (
                <span>{p.seller_count} sellers</span>
              )}
              {p.bsr_rank != null && <span>BSR #{p.bsr_rank}</span>}
              {p.category && <span>{p.category}</span>}
            </div>
            {variant === "ungatable" && p.approval_url && (
              <a
                href={p.approval_url}
                target="_blank"
                rel="noopener noreferrer"
                className="text-[10px] text-primary underline"
              >
                Request approval
              </a>
            )}
          </div>
        );
      })}
      {products.length > 10 && (
        <p className="text-[10px] text-muted-foreground text-center">
          ...and {products.length - 10} more
        </p>
      )}
    </div>
  );
}
