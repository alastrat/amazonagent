"use client";

/**
 * CopilotKit tool renderers for the FBA Concierge.
 *
 * Backend tools (get_eligible_products, check_eligibility, etc.) are executed
 * by the Go backend via the AG-UI protocol. Their results are returned as text
 * in the SSE stream and rendered as markdown by CopilotKit.
 *
 * TODO: Add frontend actions with rich UI (product cards, charts) once
 * the basic AG-UI flow is verified working.
 */
export function CopilotToolRenderers() {
  return null;
}
