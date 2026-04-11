"use client";

import { useMemo } from "react";
import ReactECharts from "echarts-for-react";
import type { TreeNode, ProductDetail } from "@/lib/types";

// ---------- types ----------

interface EChartsTreeNode {
  name: string;
  value?: number;
  children?: EChartsTreeNode[];
  itemStyle?: { color: string; borderColor?: string };
  label?: { color?: string };
  // custom data passed through for click handler
  nodeData?: {
    id: string;
    type: string;
    asin?: string;
    eligible?: boolean;
    eligibility_status?: string;
  };
}

interface Props {
  tree: TreeNode;
  products?: ProductDetail[];
  onNodeClick?: (node: { id: string; name: string; type: string }) => void;
  height?: number;
}

// ---------- helpers ----------

function categoryColor(openRate?: number): string {
  if (openRate == null) return "#eab308"; // yellow fallback
  if (openRate > 50) return "#22c55e"; // green
  if (openRate < 20) return "#ef4444"; // red
  return "#eab308"; // yellow
}

function subcategoryColor(node: TreeNode): string {
  const eligible = node.eligible_count ?? 0;
  const ungatable = node.ungatable_count ?? 0;
  const total = node.total_count;
  if (total != null && total > 0) {
    const openRatio = eligible / total;
    const accessibleRatio = (eligible + ungatable) / total;
    if (openRatio > 0.5) return "#22c55e"; // green — mostly eligible
    if (accessibleRatio > 0.5) return "#f59e0b"; // amber — mostly ungatable
    if (accessibleRatio >= 0.2) return "#eab308"; // yellow
    return "#ef4444"; // red
  }
  return "#eab308"; // yellow fallback
}

function brandColor(node: TreeNode): string {
  const status = node.eligibility_status;
  if (status === "eligible") return "#22c55e"; // green
  if (status === "ungatable") return "#f59e0b"; // amber
  return "#ef4444"; // red — restricted
}

function symbolSizeByType(type?: string): number {
  switch (type) {
    case "root":
      return 20;
    case "category":
      return 14;
    case "subcategory":
      return 10;
    case "brand":
      return 9;
    case "product":
      return 5;
    default:
      return 10;
  }
}

/** Convert backend TreeNode to ECharts tree data format */
function toEChartsNode(node: TreeNode): EChartsTreeNode {
  const type = node.type ?? "root";

  let color: string;
  let borderColor: string | undefined;

  switch (type) {
    case "root":
      color = "#6366f1";
      borderColor = "#4f46e5";
      break;
    case "category":
      color = categoryColor(node.open_rate);
      break;
    case "subcategory":
      color = subcategoryColor(node);
      break;
    case "brand":
      color = brandColor(node);
      break;
    case "product": {
      const ps = node.eligibility_status ?? (node.eligible ? "eligible" : "restricted");
      color = ps === "eligible" ? "#86efac" : ps === "ungatable" ? "#fcd34d" : "#fca5a5";
      break;
    }
    default:
      color = "#6b7280";
  }

  const result: EChartsTreeNode = {
    name: node.name,
    value: node.value,
    itemStyle: { color, ...(borderColor ? { borderColor } : {}) },
    nodeData: {
      id: node.id,
      type,
      asin: node.asin,
      eligible: node.eligible,
      eligibility_status: node.eligibility_status,
    },
  };

  if (node.children && node.children.length > 0) {
    result.children = node.children.map(toEChartsNode);
  }

  return result;
}

// ---------- component ----------

export function DiscoveryGraph({ tree, products: _products, onNodeClick, height = 440 }: Props) {
  const data = useMemo(() => [toEChartsNode(tree)], [tree]);

  const option = useMemo(
    () => ({
      tooltip: {
        trigger: "item" as const,
        formatter: (params: any) => {
          const d = params.data;
          const nd = d?.nodeData;
          const safe = (v: unknown) =>
            String(v ?? "")
              .replace(/&/g, "&amp;")
              .replace(/</g, "&lt;")
              .replace(/>/g, "&gt;")
              .replace(/"/g, "&quot;");
          const name = safe(d?.name);
          if (!nd) return name;
          const type = nd.type;
          if (type === "root") return `<strong>${name}</strong>`;
          if (type === "category") return `<strong>${name}</strong><br/>Category`;
          if (type === "subcategory") return `<strong>${name}</strong><br/>Subcategory`;
          if (type === "brand") {
            const status = nd.eligibility_status ?? "restricted";
            const badge = status === "eligible" ? "Eligible" : status === "ungatable" ? "Can Apply" : "Restricted";
            return `<strong>${name}</strong><br/>Brand &middot; ${badge}`;
          }
          if (type === "product") {
            return `<strong>${name}</strong><br/>ASIN: ${safe(nd.asin ?? "N/A")}`;
          }
          return name;
        },
      },
      series: [
        {
          type: "tree",
          layout: "radial",
          data,
          initialTreeDepth: 2,
          symbol: "circle",
          symbolSize: (value: number, params: any) => {
            const nd = params?.data?.nodeData;
            return symbolSizeByType(nd?.type);
          },
          emphasis: {
            focus: "descendant" as const,
          },
          animationDurationUpdate: 750,
          roam: true,
          label: {
            show: true,
            fontSize: 10,
            formatter: (params: any) => {
              const name = params?.data?.name ?? "";
              return name.length > 22 ? name.slice(0, 20) + "..." : name;
            },
          },
          lineStyle: {
            color: "#d1d5db",
            width: 1,
          },
          leaves: {
            label: {
              show: true,
              fontSize: 9,
            },
          },
        },
      ],
    }),
    [data],
  );

  const onEvents = useMemo((): Record<string, Function> => {
    if (!onNodeClick) return {} as Record<string, Function>;
    return {
      click: (params: any) => {
        const nd = params?.data?.nodeData;
        if (nd && (nd.type === "category" || nd.type === "subcategory" || nd.type === "brand")) {
          onNodeClick({
            id: nd.id,
            name: params.data.name,
            type: nd.type,
          });
        }
      },
    };
  }, [onNodeClick]);

  return (
    <div className="relative">
      <div
        style={{ height }}
        className="rounded-lg border bg-white overflow-hidden"
      >
        <ReactECharts
          option={option}
          style={{ height: "100%", width: "100%" }}
          onEvents={onEvents}
          notMerge
        />
      </div>
      {/* Colour legend */}
      <div className="mt-2 flex flex-wrap gap-3 text-xs text-muted-foreground">
        <span className="flex items-center gap-1">
          <span className="inline-block h-2.5 w-2.5 rounded-full bg-green-500" /> Eligible / Open
          &gt;50%
        </span>
        <span className="flex items-center gap-1">
          <span className="inline-block h-2.5 w-2.5 rounded-full bg-yellow-500" /> Partially Open
          20-50%
        </span>
        <span className="flex items-center gap-1">
          <span className="inline-block h-2.5 w-2.5 rounded-full bg-red-500" /> Restricted / Open
          &lt;20%
        </span>
        <span className="flex items-center gap-1">
          <span className="inline-block h-2.5 w-2.5 rounded-full bg-gray-400" /> Subcategory
          (colored by eligible ratio)
        </span>
        <span className="flex items-center gap-1">
          <span className="inline-block h-3 w-3 rounded border-2 border-indigo-500 bg-indigo-500" />{" "}
          Root
        </span>
      </div>
    </div>
  );
}
