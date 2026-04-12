"use client";

import { useEffect, useReducer, useRef } from "react";
import { apiClient } from "@/lib/api-client";
import type {
  TreeNode,
  ProductDetail,
  AssessmentGraphStats,
  EligibilityStatus,
  SSEEvent,
} from "@/lib/types";

const API_BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8081";

// ---------------------------------------------------------------------------
// State
// ---------------------------------------------------------------------------

export interface SSEState {
  tree: TreeNode;
  products: ProductDetail[];
  stats: AssessmentGraphStats;
  currentCategory: string | null;
  isComplete: boolean;
  connected: boolean;
}

const emptyTree: TreeNode = {
  id: "root",
  name: "Amazon US",
  type: "root",
  value: 1,
  children: [],
};

const emptyStats: AssessmentGraphStats = {
  categories_scanned: 0,
  categories_total: 20,
  eligible_products: 0,
  ungatable_products: 0,
  restricted_products: 0,
  open_brands: 0,
  restricted_brands: 0,
};

const initialState: SSEState = {
  tree: emptyTree,
  products: [],
  stats: { ...emptyStats },
  currentCategory: null,
  isComplete: false,
  connected: false,
};

// ---------------------------------------------------------------------------
// Actions
// ---------------------------------------------------------------------------

type Action =
  | { type: "CONNECTED" }
  | { type: "CATCHUP"; events: SSEEvent[] }
  | { type: "PRODUCT_BATCH"; products: SSEEvent[] }
  | { type: "CATEGORY_START"; data: Record<string, unknown> }
  | { type: "CATEGORY_COMPLETE"; data: Record<string, unknown> }
  | { type: "COMPLETE" }
  | { type: "RESET" };

// ---------------------------------------------------------------------------
// Tree builder helpers
// ---------------------------------------------------------------------------

function slugify(name: string): string {
  return name.toLowerCase().replace(/ & /g, "-").replace(/, /g, "-").replace(/ /g, "-");
}

function addProductToTree(tree: TreeNode, p: SSEEvent["data"]): TreeNode {
  const category = (p.category as string) || "Uncategorized";
  const subcategory = (p.subcategory as string) || category;
  const brand = (p.brand as string) || "Generic";
  const eligStatus = (p.eligibility_status as EligibilityStatus) || (p.eligible ? "eligible" : "restricted");

  // Clone tree shallowly
  const newTree = { ...tree, children: [...(tree.children || [])] };

  // Find or create category node
  let catIdx = newTree.children!.findIndex((c) => c.name === category);
  if (catIdx === -1) {
    newTree.children!.push({
      id: `cat-${slugify(category)}`,
      name: category,
      type: "category",
      eligible_count: 0,
      ungatable_count: 0,
      total_count: 0,
      value: 1,
      children: [],
    });
    catIdx = newTree.children!.length - 1;
  }
  const catNode = { ...newTree.children![catIdx], children: [...(newTree.children![catIdx].children || [])] };
  newTree.children![catIdx] = catNode;
  catNode.total_count = (catNode.total_count || 0) + 1;
  if (eligStatus === "eligible") catNode.eligible_count = (catNode.eligible_count || 0) + 1;
  if (eligStatus === "ungatable") catNode.ungatable_count = (catNode.ungatable_count || 0) + 1;
  const catTotal = catNode.total_count || 1;
  catNode.open_rate = ((catNode.eligible_count || 0) / catTotal) * 100;
  catNode.value = Math.max(catNode.eligible_count || 1, 1);

  // Find or create subcategory node
  let subIdx = catNode.children!.findIndex((c) => c.name === subcategory);
  if (subIdx === -1) {
    catNode.children!.push({
      id: `subcat-${slugify(subcategory)}`,
      name: subcategory,
      type: "subcategory",
      eligible_count: 0,
      ungatable_count: 0,
      total_count: 0,
      value: 1,
      children: [],
    });
    subIdx = catNode.children!.length - 1;
  }
  const subNode = { ...catNode.children![subIdx], children: [...(catNode.children![subIdx].children || [])] };
  catNode.children![subIdx] = subNode;
  subNode.total_count = (subNode.total_count || 0) + 1;
  if (eligStatus === "eligible") subNode.eligible_count = (subNode.eligible_count || 0) + 1;
  if (eligStatus === "ungatable") subNode.ungatable_count = (subNode.ungatable_count || 0) + 1;
  subNode.value = Math.max((subNode.eligible_count || 0) + (subNode.ungatable_count || 0), 1);

  // Find or create brand node
  let brandIdx = subNode.children!.findIndex((c) => c.name === brand);
  if (brandIdx === -1) {
    subNode.children!.push({
      id: `brand-${slugify(brand)}`,
      name: brand,
      type: "brand",
      eligible: eligStatus === "eligible",
      eligibility_status: eligStatus,
      value: 1,
    });
    brandIdx = subNode.children!.length - 1;
  } else {
    const existing = subNode.children![brandIdx];
    // Upgrade status: eligible > ungatable > restricted
    const statusRank = { eligible: 3, ungatable: 2, restricted: 1 };
    const currentRank = statusRank[existing.eligibility_status || "restricted"];
    const newRank = statusRank[eligStatus];
    if (newRank > currentRank) {
      subNode.children![brandIdx] = {
        ...existing,
        eligible: eligStatus === "eligible",
        eligibility_status: eligStatus,
        value: (existing.value || 1) + 1,
      };
    } else {
      subNode.children![brandIdx] = { ...existing, value: (existing.value || 1) + 1 };
    }
  }

  newTree.value = newTree.children!.length || 1;
  return newTree;
}

// ---------------------------------------------------------------------------
// Reducer
// ---------------------------------------------------------------------------

function sseReducer(state: SSEState, action: Action): SSEState {
  switch (action.type) {
    case "CONNECTED":
      return { ...state, connected: true };

    case "CATCHUP": {
      // Replay all product_found events from history
      let tree: TreeNode = { ...emptyTree, children: [] };
      const products: ProductDetail[] = [];
      const stats = { ...emptyStats };
      const seenASINs = new Set<string>();
      const openBrands = new Set<string>();

      for (const evt of action.events) {
        if (evt.type === "product_found") {
          const d = evt.data;
          const asin = d.asin as string;
          if (seenASINs.has(asin)) continue;
          seenASINs.add(asin);

          tree = addProductToTree(tree, d);
          const eligStatus = (d.eligibility_status as EligibilityStatus) || "restricted";
          products.push({
            asin,
            title: (d.title as string) || "",
            brand: (d.brand as string) || "Generic",
            category: (d.category as string) || "",
            subcategory: (d.subcategory as string) || "",
            price: (d.price as number) || 0,
            est_margin_pct: 0,
            seller_count: (d.seller_count as number) || 0,
            eligible: (d.eligible as boolean) || false,
            eligibility_status: eligStatus,
            approval_url: (d.approval_url as string) || undefined,
          });

          if (eligStatus === "eligible") stats.eligible_products++;
          else if (eligStatus === "ungatable") stats.ungatable_products++;
          else stats.restricted_products++;

          if (eligStatus === "eligible" || eligStatus === "ungatable") {
            openBrands.add((d.brand as string) || "Generic");
          }
        } else if (evt.type === "category_complete") {
          stats.categories_scanned++;
        }
      }
      stats.open_brands = openBrands.size;

      return { ...state, tree, products, stats, connected: true };
    }

    case "PRODUCT_BATCH": {
      let tree = state.tree;
      const products = [...state.products];
      const stats = { ...state.stats };
      const openBrands = new Set<string>();
      // Rebuild open brands set from existing products
      for (const p of state.products) {
        if (p.eligibility_status === "eligible" || p.eligibility_status === "ungatable") {
          openBrands.add(p.brand);
        }
      }
      const seenASINs = new Set(state.products.map((p) => p.asin));

      for (const evt of action.products) {
        const d = evt.data;
        const asin = d.asin as string;
        if (seenASINs.has(asin)) continue;
        seenASINs.add(asin);

        tree = addProductToTree(tree, d);
        const eligStatus = (d.eligibility_status as EligibilityStatus) || "restricted";
        products.push({
          asin,
          title: (d.title as string) || "",
          brand: (d.brand as string) || "Generic",
          category: (d.category as string) || "",
          subcategory: (d.subcategory as string) || "",
          price: (d.price as number) || 0,
          est_margin_pct: 0,
          seller_count: (d.seller_count as number) || 0,
          eligible: (d.eligible as boolean) || false,
          eligibility_status: eligStatus,
          approval_url: (d.approval_url as string) || undefined,
        });

        if (eligStatus === "eligible") stats.eligible_products++;
        else if (eligStatus === "ungatable") stats.ungatable_products++;
        else stats.restricted_products++;

        if (eligStatus === "eligible" || eligStatus === "ungatable") {
          openBrands.add((d.brand as string) || "Generic");
        }
      }
      stats.open_brands = openBrands.size;

      return { ...state, tree, products, stats };
    }

    case "CATEGORY_START":
      return { ...state, currentCategory: (action.data.category as string) || null };

    case "CATEGORY_COMPLETE":
      return {
        ...state,
        currentCategory: null,
        stats: {
          ...state.stats,
          categories_scanned: state.stats.categories_scanned + 1,
        },
      };

    case "COMPLETE":
      return { ...state, isComplete: true };

    case "RESET":
      return { ...initialState };

    default:
      return state;
  }
}

// ---------------------------------------------------------------------------
// Hook
// ---------------------------------------------------------------------------

export function useAssessmentSSE(enabled: boolean) {
  const [state, dispatch] = useReducer(sseReducer, initialState);
  const pendingRef = useRef<SSEEvent[]>([]);
  const rafRef = useRef<number | null>(null);

  useEffect(() => {
    if (!enabled) {
      dispatch({ type: "RESET" });
      return;
    }

    const token = apiClient.getToken();
    const url = `${API_BASE}/assessment/events${token ? `?token=${token}` : ""}`;
    const es = new EventSource(url);

    function flushBatch() {
      if (pendingRef.current.length > 0) {
        dispatch({ type: "PRODUCT_BATCH", products: [...pendingRef.current] });
        pendingRef.current = [];
      }
      rafRef.current = null;
    }

    es.addEventListener("catchup", (e) => {
      const events: SSEEvent[] = JSON.parse((e as MessageEvent).data);
      dispatch({ type: "CATCHUP", events });
    });

    es.addEventListener("product_found", (e) => {
      const evt: SSEEvent = JSON.parse((e as MessageEvent).data);
      pendingRef.current.push(evt);
      if (rafRef.current === null) {
        rafRef.current = requestAnimationFrame(flushBatch);
      }
    });

    es.addEventListener("category_start", (e) => {
      const evt: SSEEvent = JSON.parse((e as MessageEvent).data);
      dispatch({ type: "CATEGORY_START", data: evt.data });
    });

    es.addEventListener("category_complete", (e) => {
      // Flush any pending products first
      flushBatch();
      const evt: SSEEvent = JSON.parse((e as MessageEvent).data);
      dispatch({ type: "CATEGORY_COMPLETE", data: evt.data });
    });

    es.addEventListener("assessment_complete", () => {
      flushBatch();
      dispatch({ type: "COMPLETE" });
    });

    es.addEventListener("done", () => {
      flushBatch();
      dispatch({ type: "COMPLETE" });
      es.close();
    });

    es.onopen = () => {
      dispatch({ type: "CONNECTED" });
    };

    es.onerror = () => {
      // EventSource auto-reconnects; no action needed
    };

    return () => {
      if (rafRef.current !== null) {
        cancelAnimationFrame(rafRef.current);
      }
      es.close();
    };
  }, [enabled]);

  return state;
}
