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

export function AppShell({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const [chatOpen, setChatOpen] = useState(() => {
    if (typeof window !== "undefined") {
      return localStorage.getItem("chat-panel-open") === "true";
    }
    return false;
  });

  useEffect(() => {
    localStorage.setItem("chat-panel-open", String(chatOpen));
  }, [chatOpen]);

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
        labels={{
          title: "FBA Concierge",
          placeholder: "Ask your concierge...",
        }}
        className="z-50"
      />
    </div>
  );
}
