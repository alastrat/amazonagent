"use client";

import { useCopilotAction } from "@copilotkit/react-core";

const API_BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8081";
const AUTH = "Bearer dev-user-dev-tenant";

/**
 * Registers frontend tools that call the Go backend API.
 * CopilotKit's frontend actions are the correct way to expose tools to Claude.
 */
export function CopilotToolRenderers() {
  useCopilotAction({
    name: "get_assessment_summary",
    description: "Get the seller's assessment summary — eligible, ungatable, restricted product counts, categories scanned, and overall open rate. ALWAYS call this first when the user asks about their account, products, or eligibility.",
    parameters: [],
    handler: async () => {
      const resp = await fetch(`${API_BASE}/assessment/graph`, {
        headers: { Authorization: AUTH },
      });
      const data = await resp.json();
      return JSON.stringify(data.stats || {});
    },
  });

  useCopilotAction({
    name: "get_eligible_products",
    description: "Get the list of products the seller can list immediately on Amazon. Returns ASINs, titles, brands, prices, margins, and seller counts.",
    parameters: [],
    handler: async () => {
      const resp = await fetch(`${API_BASE}/assessment/graph`, {
        headers: { Authorization: AUTH },
      });
      const data = await resp.json();
      const products = (data.products || []).filter(
        (p: { eligibility_status?: string }) => p.eligibility_status === "eligible",
      );
      return JSON.stringify({ count: products.length, products: products.slice(0, 20) });
    },
  });

  useCopilotAction({
    name: "get_ungatable_products",
    description: "Get products that require approval but the seller CAN apply for. Returns ASINs, brands, prices, margins, and Seller Central approval URLs.",
    parameters: [],
    handler: async () => {
      const resp = await fetch(`${API_BASE}/assessment/graph`, {
        headers: { Authorization: AUTH },
      });
      const data = await resp.json();
      const products = (data.products || []).filter(
        (p: { eligibility_status?: string }) => p.eligibility_status === "ungatable",
      );
      return JSON.stringify({ count: products.length, products: products.slice(0, 20) });
    },
  });

  useCopilotAction({
    name: "get_seller_profile",
    description: "Get the seller's profile — archetype classification and assessment status.",
    parameters: [],
    handler: async () => {
      const resp = await fetch(`${API_BASE}/assessment/profile`, {
        headers: { Authorization: AUTH },
      });
      return resp.text();
    },
  });

  return null;
}
