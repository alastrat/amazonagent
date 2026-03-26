"use client";

import { useState } from "react";
import type { Evidence } from "@/lib/types";

export function EvidencePanel({ evidence }: { evidence: Evidence }) {
  const [expanded, setExpanded] = useState<string | null>(null);

  const sections = [
    { key: "demand", label: "Demand Analysis", data: evidence.demand },
    { key: "competition", label: "Competition Analysis", data: evidence.competition },
    { key: "margin", label: "Profitability Analysis", data: evidence.margin },
    { key: "risk", label: "Risk Assessment", data: evidence.risk },
    { key: "sourcing", label: "Sourcing Feasibility", data: evidence.sourcing },
  ];

  return (
    <div className="space-y-2">
      {sections.map((section) => (
        <div key={section.key} className="rounded-lg border">
          <button
            className="flex w-full items-center justify-between p-3 text-left text-sm font-medium"
            onClick={() => setExpanded(expanded === section.key ? null : section.key)}
          >
            {section.label}
            <span className="text-muted-foreground">{expanded === section.key ? "\u2212" : "+"}</span>
          </button>
          {expanded === section.key && section.data && (
            <div className="border-t px-3 pb-3 pt-2 text-sm">
              <p className="whitespace-pre-wrap">{section.data.reasoning}</p>
              {section.data.data && Object.keys(section.data.data).length > 0 && (
                <pre className="mt-2 overflow-auto rounded bg-muted p-2 text-xs">
                  {JSON.stringify(section.data.data, null, 2)}
                </pre>
              )}
            </div>
          )}
        </div>
      ))}
    </div>
  );
}
