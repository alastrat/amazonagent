"use client";

import { useMemo, useCallback } from "react";
import Tree from "react-d3-tree";
import type { RawNodeDatum, CustomNodeElementProps } from "react-d3-tree";
import type { TreeNode } from "@/lib/types";

// ---------- helpers ----------

function categoryBorderColor(openRate?: number): string {
  if (openRate == null) return "#eab308"; // yellow fallback
  if (openRate > 50) return "#22c55e"; // green
  if (openRate < 20) return "#ef4444"; // red
  return "#eab308"; // yellow
}

/** Convert backend TreeNode to react-d3-tree RawNodeDatum */
function toRawNode(node: TreeNode): RawNodeDatum {
  return {
    name: node.name,
    attributes: {
      id: node.id,
      type: node.type ?? "root",
      ...(node.open_rate != null ? { open_rate: node.open_rate } : {}),
      ...(node.eligible_count != null ? { eligible_count: node.eligible_count } : {}),
      ...(node.total_count != null ? { total_count: node.total_count } : {}),
      ...(node.eligible != null ? { eligible: String(node.eligible) } : {}),
    },
    children: node.children?.map(toRawNode) ?? [],
  };
}

// ---------- custom node rendering ----------

function renderCustomNode({ nodeDatum, toggleNode }: CustomNodeElementProps) {
  const attrs = nodeDatum.attributes ?? {};
  const nodeType = (attrs.type as string) ?? "root";
  const openRate = attrs.open_rate != null ? Number(attrs.open_rate) : undefined;
  const eligibleCount = attrs.eligible_count != null ? Number(attrs.eligible_count) : undefined;
  const totalCount = attrs.total_count != null ? Number(attrs.total_count) : undefined;
  const eligible = attrs.eligible === "true";

  if (nodeType === "root") {
    return (
      <g onClick={toggleNode} style={{ cursor: "pointer" }}>
        <circle r={18} fill="#6366f1" stroke="#4f46e5" strokeWidth={2} />
        <text
          dy="0.35em"
          textAnchor="middle"
          fill="white"
          fontSize={9}
          fontWeight="bold"
          style={{ pointerEvents: "none" }}
        >
          {nodeDatum.name}
        </text>
      </g>
    );
  }

  if (nodeType === "category") {
    const border = categoryBorderColor(openRate);
    const w = 160;
    const h = 48;
    return (
      <g onClick={toggleNode} style={{ cursor: "pointer" }}>
        <rect
          x={-w / 2}
          y={-h / 2}
          width={w}
          height={h}
          rx={6}
          fill="white"
          stroke={border}
          strokeWidth={2.5}
        />
        {/* Name */}
        <text
          dy={eligibleCount != null ? "-0.3em" : "0.35em"}
          textAnchor="middle"
          fill="#1f2937"
          fontSize={11}
          fontWeight={600}
          style={{ pointerEvents: "none" }}
        >
          {nodeDatum.name.length > 20 ? nodeDatum.name.slice(0, 18) + "..." : nodeDatum.name}
        </text>
        {/* Stats line */}
        {eligibleCount != null && totalCount != null && (
          <text
            dy="1.1em"
            textAnchor="middle"
            fill="#6b7280"
            fontSize={9}
            style={{ pointerEvents: "none" }}
          >
            {eligibleCount}/{totalCount} eligible
          </text>
        )}
        {/* Open rate badge */}
        {openRate != null && (
          <>
            <rect
              x={w / 2 - 34}
              y={-h / 2 - 8}
              width={32}
              height={16}
              rx={8}
              fill={border}
            />
            <text
              x={w / 2 - 18}
              y={-h / 2 + 4}
              textAnchor="middle"
              fill="white"
              fontSize={9}
              fontWeight={600}
              style={{ pointerEvents: "none" }}
            >
              {openRate}%
            </text>
          </>
        )}
      </g>
    );
  }

  // Brand node
  const dotColor = eligible ? "#22c55e" : "#ef4444";
  return (
    <g style={{ cursor: "default" }}>
      <circle r={8} fill={dotColor} stroke={eligible ? "#16a34a" : "#dc2626"} strokeWidth={1.5} />
      <text
        x={14}
        dy="0.35em"
        textAnchor="start"
        fill="#374151"
        fontSize={10}
        style={{ pointerEvents: "none" }}
      >
        {nodeDatum.name}
      </text>
    </g>
  );
}

// ---------- component ----------

export function DiscoveryGraph({
  tree,
  width = 680,
  height = 440,
}: {
  tree: TreeNode;
  width?: number;
  height?: number;
}) {
  const data = useMemo(() => toRawNode(tree), [tree]);

  const translate = useMemo(() => ({ x: 80, y: height / 2 }), [height]);

  const nodeSize = useMemo(() => ({ x: 220, y: 70 }), []);

  const renderNode = useCallback(
    (props: CustomNodeElementProps) => renderCustomNode(props),
    [],
  );

  return (
    <div className="relative">
      <div
        style={{ width, height }}
        className="rounded-lg border bg-white overflow-hidden"
      >
        <Tree
          data={data}
          orientation="horizontal"
          translate={translate}
          nodeSize={nodeSize}
          renderCustomNodeElement={renderNode}
          pathFunc="step"
          collapsible
          initialDepth={1}
          zoomable
          draggable
          separation={{ siblings: 1, nonSiblings: 1.4 }}
          pathClassFunc={() => "stroke-gray-300 stroke-1 fill-none"}
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
          <span className="inline-block h-3 w-3 rounded border-2 border-indigo-500 bg-indigo-500" />{" "}
          Root
        </span>
      </div>
    </div>
  );
}
