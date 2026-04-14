"use client";

import { useState, useEffect, useCallback } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { CopilotSidebar } from "@copilotkit/react-ui";

const navItems = [
  { href: "/dashboard", label: "Dashboard" },
  { href: "/campaigns", label: "Campaigns" },
  { href: "/deals", label: "Deals" },
  { href: "/catalog", label: "Catalog" },
  { href: "/brands", label: "Brands" },
  { href: "/scans", label: "Scans" },
  { href: "/onboarding", label: "Onboarding" },
  { href: "/strategy", label: "Strategy" },
  { href: "/suggestions", label: "Suggestions" },
  { href: "/discovery", label: "Discovery" },
  { href: "/audit", label: "Audit" },
  { href: "/settings", label: "Settings" },
];

const API_BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8081";
const AUTH = "Bearer dev-user-dev-tenant";
const BASE_INSTRUCTIONS =
  "You are an FBA wholesale concierge for an Amazon seller. " +
  "You MUST use your tools to answer questions — never respond from memory or general knowledge. " +
  "Available tools: get_assessment_summary, get_eligible_products, get_ungatable_products, get_seller_profile. " +
  "When the user asks about products, eligibility, or their account, ALWAYS call the relevant tool first, then respond with the real data. " +
  "Be concise and action-oriented. Show ASINs, prices, margins, and approval URLs when available.";

export function AppShell({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const [chatOpen, setChatOpen] = useState(() => {
    if (typeof window !== "undefined") {
      return localStorage.getItem("chat-panel-open") === "true";
    }
    return false;
  });
  const [systemInstructions, setSystemInstructions] = useState(BASE_INSTRUCTIONS);

  useEffect(() => {
    localStorage.setItem("chat-panel-open", String(chatOpen));
  }, [chatOpen]);

  // Load previous conversation from DB and inject as context into system prompt
  useEffect(() => {
    (async () => {
      try {
        const resp = await fetch(`${API_BASE}/chat/history`, {
          headers: { Authorization: AUTH },
        });
        if (!resp.ok) return;
        const data = await resp.json();
        const msgs = data.messages ?? [];
        if (msgs.length === 0) return;

        // Build conversation summary for the system prompt
        const historyLines = msgs.slice(-10).map((m: { role: string; content: string }) =>
          `${m.role === "user" ? "User" : "Assistant"}: ${m.content.slice(0, 300)}${m.content.length > 300 ? "..." : ""}`
        );
        const historySummary =
          "\n\nPREVIOUS CONVERSATION (for context — do NOT repeat these answers, use them as context for follow-ups):\n" +
          historyLines.join("\n");

        setSystemInstructions(BASE_INSTRUCTIONS + historySummary);
      } catch {
        // history load failed — use base instructions
      }
    })();
  }, []);

  const toggleChat = useCallback(() => {
    setChatOpen((prev) => !prev);
  }, []);

  return (
    <div className="flex h-screen">
      <aside className="flex w-56 flex-col border-r bg-muted/30">
        <div className="p-4">
          <h2 className="text-lg font-semibold">FBA Orchestrator</h2>
        </div>
        <nav className="flex-1 space-y-1 px-2">
          {navItems.map((item) => {
            const active = pathname.startsWith(item.href);
            return (
              <Link
                key={item.href}
                href={item.href}
                className={`block rounded-md px-3 py-2 text-sm font-medium ${
                  active
                    ? "bg-primary text-primary-foreground"
                    : "text-muted-foreground hover:bg-muted"
                }`}
              >
                {item.label}
              </Link>
            );
          })}
        </nav>
        <div className="border-t p-2">
          <button
            onClick={toggleChat}
            className={`w-full rounded-md px-3 py-2 text-sm font-medium text-left ${
              chatOpen
                ? "bg-primary text-primary-foreground"
                : "text-muted-foreground hover:bg-muted"
            }`}
          >
            {chatOpen ? "Close Concierge" : "Ask Concierge"}
          </button>
        </div>
      </aside>
      <main className="flex-1 overflow-auto p-6">
        {children}
      </main>
      <CopilotSidebar
        defaultOpen={chatOpen}
        onSetOpen={setChatOpen}
        instructions={systemInstructions}
        labels={{
          title: "FBA Concierge",
          placeholder: "Ask your concierge...",
        }}
        className="z-50"
      />
    </div>
  );
}
