import {
  CopilotRuntime,
  AnthropicAdapter,
  copilotRuntimeNextJSAppRouterEndpoint,
} from "@copilotkit/runtime";
import { NextRequest } from "next/server";

const API_BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8081";
const AUTH_TOKEN = "Bearer dev-user-dev-tenant";

// Helper to call Go backend tools
async function callGoTool(toolName: string, input: Record<string, unknown> = {}): Promise<string> {
  const resp = await fetch(`${API_BASE}/chat/send`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      "Authorization": AUTH_TOKEN,
    },
    body: JSON.stringify({ message: `[TOOL:${toolName}] ${JSON.stringify(input)}` }),
  });
  if (!resp.ok) return `Error: ${resp.statusText}`;
  return resp.text();
}

const runtime = new CopilotRuntime({
  actions: [
    {
      name: "get_assessment_summary",
      description: "Get the seller's assessment summary — eligible, ungatable, and restricted product counts, category breakdown, and open rates. Call this first to understand the seller's situation.",
      parameters: [],
      handler: async () => {
        const resp = await fetch(`${API_BASE}/assessment/graph`, {
          headers: { "Authorization": AUTH_TOKEN },
        });
        const data = await resp.json();
        return JSON.stringify(data.stats || {});
      },
    },
    {
      name: "get_eligible_products",
      description: "Get products the seller can list immediately on Amazon. Returns ASINs, titles, brands, prices, margins.",
      parameters: [],
      handler: async () => {
        const resp = await fetch(`${API_BASE}/assessment/graph`, {
          headers: { "Authorization": AUTH_TOKEN },
        });
        const data = await resp.json();
        const products = (data.products || []).filter((p: any) => p.eligibility_status === "eligible");
        return JSON.stringify({ count: products.length, products: products.slice(0, 20) });
      },
    },
    {
      name: "get_ungatable_products",
      description: "Get products that need approval but the seller CAN apply. Returns ASINs, brands, prices, margins, and Seller Central approval URLs.",
      parameters: [],
      handler: async () => {
        const resp = await fetch(`${API_BASE}/assessment/graph`, {
          headers: { "Authorization": AUTH_TOKEN },
        });
        const data = await resp.json();
        const products = (data.products || []).filter((p: any) => p.eligibility_status === "ungatable");
        return JSON.stringify({ count: products.length, products: products.slice(0, 20) });
      },
    },
    {
      name: "get_seller_profile",
      description: "Get the seller's profile including archetype and assessment status.",
      parameters: [],
      handler: async () => {
        const resp = await fetch(`${API_BASE}/assessment/profile`, {
          headers: { "Authorization": AUTH_TOKEN },
        });
        return resp.text();
      },
    },
  ],
});

const serviceAdapter = new AnthropicAdapter({
  model: "claude-sonnet-4-5-20250929",
});

export const POST = async (req: NextRequest) => {
  const { handleRequest } = copilotRuntimeNextJSAppRouterEndpoint({
    runtime,
    serviceAdapter,
    endpoint: "/api/copilotkit",
  });
  return handleRequest(req);
};
