import { NextRequest, NextResponse } from "next/server";

const API_BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8081";

export async function POST(req: NextRequest) {
  // Forward to Go backend AG-UI endpoint
  const body = await req.text();
  const token =
    req.headers.get("authorization") || "Bearer dev-user-dev-tenant";

  const resp = await fetch(`${API_BASE}/api/copilotkit`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Authorization: token,
    },
    body,
  });

  // Stream SSE response back to CopilotKit
  return new NextResponse(resp.body, {
    headers: {
      "Content-Type": "text/event-stream",
      "Cache-Control": "no-cache",
      Connection: "keep-alive",
    },
  });
}
