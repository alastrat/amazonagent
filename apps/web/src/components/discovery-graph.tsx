"use client";

import { useEffect, useRef, useCallback, useState } from "react";
import type {
  AssessmentGraph,
  AssessmentGraphNode,
  AssessmentGraphEdge,
} from "@/lib/types";

// ---------- colour helpers ----------

function nodeColor(node: AssessmentGraphNode): string {
  if (node.type === "root") return "#6366f1"; // indigo
  if (node.status === "scanning") return "#3b82f6"; // blue
  if (node.status === "not_scanned" || node.status === "skipped") return "#9ca3af"; // gray

  // scanned nodes colour by eligibility
  if (node.type === "category") {
    const rate = node.open_rate ?? 0;
    if (rate >= 60) return "#22c55e"; // green
    if (rate >= 30) return "#eab308"; // yellow
    return "#ef4444"; // red
  }

  if (node.eligible === true) return "#22c55e";
  if (node.eligible === false) return "#ef4444";
  return "#9ca3af";
}

function nodeRadius(node: AssessmentGraphNode): number {
  switch (node.type) {
    case "root":
      return 14;
    case "category":
      return 10;
    case "brand":
      return 6;
    case "product":
      return 4;
    default:
      return 6;
  }
}

// ---------- canvas-based force graph ----------

interface SimNode extends AssessmentGraphNode {
  x: number;
  y: number;
  vx: number;
  vy: number;
  fx?: number | null;
  fy?: number | null;
  _radius: number;
  _color: string;
}

interface SimEdge {
  source: SimNode;
  target: SimNode;
}

/**
 * DiscoveryGraph renders a force-directed graph on a canvas element.
 * It uses a simple velocity-Verlet simulation (no external d3-force dependency)
 * so we avoid heavy library imports and keep the bundle lean.
 */
export function DiscoveryGraph({
  graph,
  width = 600,
  height = 440,
  interactive = false,
}: {
  graph: AssessmentGraph;
  width?: number;
  height?: number;
  interactive?: boolean;
}) {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const nodesRef = useRef<SimNode[]>([]);
  const edgesRef = useRef<SimEdge[]>([]);
  const animRef = useRef<number>(0);
  const tickRef = useRef(0);

  const [tooltip, setTooltip] = useState<{
    text: string;
    x: number;
    y: number;
  } | null>(null);

  // ---------- build / update simulation data ----------

  const syncGraph = useCallback(
    (g: AssessmentGraph) => {
      const prevMap = new Map<string, SimNode>();
      for (const n of nodesRef.current) {
        prevMap.set(n.id, n);
      }

      const cx = width / 2;
      const cy = height / 2;

      const newNodes: SimNode[] = g.nodes.map((n) => {
        const prev = prevMap.get(n.id);
        if (prev) {
          // preserve position, update visual data
          prev.label = n.label;
          prev.status = n.status;
          prev.eligible = n.eligible;
          prev.open_rate = n.open_rate;
          prev.price = n.price;
          prev.margin = n.margin;
          prev._color = nodeColor(n);
          prev._radius = nodeRadius(n);
          return prev;
        }

        // initial placement based on type
        let ix = cx + (Math.random() - 0.5) * 120;
        let iy = cy + (Math.random() - 0.5) * 120;
        if (n.type === "root") {
          ix = cx;
          iy = cy;
        } else if (n.type === "category") {
          const angle = Math.random() * Math.PI * 2;
          ix = cx + Math.cos(angle) * 120;
          iy = cy + Math.sin(angle) * 120;
        } else if (n.type === "brand") {
          const angle = Math.random() * Math.PI * 2;
          ix = cx + Math.cos(angle) * 180;
          iy = cy + Math.sin(angle) * 180;
        } else {
          const angle = Math.random() * Math.PI * 2;
          ix = cx + Math.cos(angle) * 220;
          iy = cy + Math.sin(angle) * 220;
        }

        return {
          ...n,
          x: ix,
          y: iy,
          vx: 0,
          vy: 0,
          _radius: nodeRadius(n),
          _color: nodeColor(n),
        } as SimNode;
      });

      const nodeMap = new Map<string, SimNode>();
      for (const n of newNodes) nodeMap.set(n.id, n);

      // pin root
      const root = nodeMap.get("marketplace");
      if (root) {
        root.fx = cx;
        root.fy = cy;
      }

      const newEdges: SimEdge[] = [];
      for (const e of g.edges) {
        const s = nodeMap.get(e.source);
        const t = nodeMap.get(e.target);
        if (s && t) newEdges.push({ source: s, target: t });
      }

      nodesRef.current = newNodes;
      edgesRef.current = newEdges;
      tickRef.current = 0; // restart simulation energy
    },
    [width, height],
  );

  // ---------- force simulation tick ----------

  const simTick = useCallback(() => {
    const nodes = nodesRef.current;
    const edges = edgesRef.current;
    const alpha = Math.max(0.002, 0.3 * Math.pow(0.99, tickRef.current));
    tickRef.current++;

    const cx = width / 2;
    const cy = height / 2;

    // centre gravity
    for (const n of nodes) {
      if (n.fx != null) {
        n.x = n.fx;
        n.y = n.fy!;
        continue;
      }
      n.vx += (cx - n.x) * 0.002 * alpha;
      n.vy += (cy - n.y) * 0.002 * alpha;
    }

    // repulsion (simplified Barnes-Hut not needed for < 600 nodes)
    for (let i = 0; i < nodes.length; i++) {
      for (let j = i + 1; j < nodes.length; j++) {
        const a = nodes[i];
        const b = nodes[j];
        let dx = b.x - a.x;
        let dy = b.y - a.y;
        let dist = Math.sqrt(dx * dx + dy * dy) || 1;
        const minDist = a._radius + b._radius + 12;
        if (dist < minDist) dist = minDist;
        const force = (-300 * alpha) / (dist * dist);
        const fx = (dx / dist) * force;
        const fy = (dy / dist) * force;
        if (a.fx == null) {
          a.vx -= fx;
          a.vy -= fy;
        }
        if (b.fx == null) {
          b.vx += fx;
          b.vy += fy;
        }
      }
    }

    // spring (link force)
    const idealLen: Record<string, number> = {
      root: 100,
      category: 70,
      brand: 50,
      product: 35,
    };
    for (const e of edges) {
      let dx = e.target.x - e.source.x;
      let dy = e.target.y - e.source.y;
      const dist = Math.sqrt(dx * dx + dy * dy) || 1;
      const desired = idealLen[e.target.type] ?? 60;
      const diff = (dist - desired) * 0.06 * alpha;
      const fx = (dx / dist) * diff;
      const fy = (dy / dist) * diff;
      if (e.source.fx == null) {
        e.source.vx += fx;
        e.source.vy += fy;
      }
      if (e.target.fx == null) {
        e.target.vx -= fx;
        e.target.vy -= fy;
      }
    }

    // velocity integration + damping + bounds
    for (const n of nodes) {
      if (n.fx != null) continue;
      n.vx *= 0.85;
      n.vy *= 0.85;
      n.x += n.vx;
      n.y += n.vy;
      // keep in bounds
      const pad = n._radius + 4;
      n.x = Math.max(pad, Math.min(width - pad, n.x));
      n.y = Math.max(pad, Math.min(height - pad, n.y));
    }
  }, [width, height]);

  // ---------- draw ----------

  const draw = useCallback(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const ctx = canvas.getContext("2d");
    if (!ctx) return;

    ctx.clearRect(0, 0, width, height);

    // edges
    ctx.lineWidth = 0.8;
    ctx.strokeStyle = "rgba(156,163,175,0.35)";
    for (const e of edgesRef.current) {
      ctx.beginPath();
      ctx.moveTo(e.source.x, e.source.y);
      ctx.lineTo(e.target.x, e.target.y);
      ctx.stroke();
    }

    // nodes
    const now = Date.now();
    for (const n of nodesRef.current) {
      ctx.beginPath();
      let r = n._radius;

      // pulse animation for scanning nodes
      if (n.status === "scanning") {
        const pulse = Math.sin(now / 300) * 2;
        r += pulse;

        // glow
        ctx.shadowColor = "#3b82f6";
        ctx.shadowBlur = 8;
      } else {
        ctx.shadowColor = "transparent";
        ctx.shadowBlur = 0;
      }

      ctx.arc(n.x, n.y, Math.max(2, r), 0, Math.PI * 2);
      ctx.fillStyle = n._color;
      ctx.fill();

      ctx.shadowColor = "transparent";
      ctx.shadowBlur = 0;

      // label for root / category nodes
      if (n.type === "root" || n.type === "category") {
        ctx.font = n.type === "root" ? "bold 11px sans-serif" : "10px sans-serif";
        ctx.fillStyle = "#1f2937";
        ctx.textAlign = "center";
        ctx.textBaseline = "top";
        ctx.fillText(n.label, n.x, n.y + r + 3, 100);
      }
    }
  }, [width, height]);

  // ---------- animation loop ----------

  useEffect(() => {
    let running = true;
    const loop = () => {
      if (!running) return;
      simTick();
      draw();
      animRef.current = requestAnimationFrame(loop);
    };
    loop();
    return () => {
      running = false;
      cancelAnimationFrame(animRef.current);
    };
  }, [simTick, draw]);

  // ---------- sync when graph data changes ----------

  useEffect(() => {
    syncGraph(graph);
  }, [graph, syncGraph]);

  // ---------- hover tooltip ----------

  const handleMouseMove = useCallback(
    (e: React.MouseEvent<HTMLCanvasElement>) => {
      if (!interactive) return;
      const rect = canvasRef.current?.getBoundingClientRect();
      if (!rect) return;
      const mx = e.clientX - rect.left;
      const my = e.clientY - rect.top;

      for (const n of nodesRef.current) {
        const dx = mx - n.x;
        const dy = my - n.y;
        if (dx * dx + dy * dy < (n._radius + 4) ** 2) {
          let text = n.label;
          if (n.type === "category" && n.open_rate != null) {
            text += ` (${n.open_rate.toFixed(0)}% open)`;
          }
          if (n.type === "product") {
            if (n.price != null) text += ` — $${n.price.toFixed(2)}`;
            if (n.margin != null) text += `, ${n.margin.toFixed(1)}% margin`;
          }
          if (n.type === "brand") {
            text += n.eligible ? " (eligible)" : " (restricted)";
          }
          setTooltip({ text, x: e.clientX - rect.left, y: e.clientY - rect.top - 24 });
          return;
        }
      }
      setTooltip(null);
    },
    [interactive],
  );

  const handleMouseLeave = useCallback(() => setTooltip(null), []);

  return (
    <div className="relative">
      <canvas
        ref={canvasRef}
        width={width}
        height={height}
        className="w-full rounded-lg border bg-white"
        style={{ maxWidth: width, maxHeight: height }}
        onMouseMove={handleMouseMove}
        onMouseLeave={handleMouseLeave}
      />
      {tooltip && (
        <div
          className="pointer-events-none absolute z-10 rounded bg-gray-800 px-2 py-1 text-xs text-white shadow"
          style={{ left: tooltip.x, top: tooltip.y, transform: "translateX(-50%)" }}
        >
          {tooltip.text}
        </div>
      )}
      {/* Colour legend */}
      <div className="mt-2 flex flex-wrap gap-3 text-xs text-muted-foreground">
        <span className="flex items-center gap-1">
          <span className="inline-block h-2.5 w-2.5 rounded-full bg-green-500" /> Eligible
        </span>
        <span className="flex items-center gap-1">
          <span className="inline-block h-2.5 w-2.5 rounded-full bg-red-500" /> Restricted
        </span>
        <span className="flex items-center gap-1">
          <span className="inline-block h-2.5 w-2.5 rounded-full bg-blue-500" /> Scanning
        </span>
        <span className="flex items-center gap-1">
          <span className="inline-block h-2.5 w-2.5 rounded-full bg-gray-400" /> Not Scanned
        </span>
        <span className="flex items-center gap-1">
          <span className="inline-block h-2.5 w-2.5 rounded-full bg-yellow-500" /> Partially Open
        </span>
      </div>
    </div>
  );
}
