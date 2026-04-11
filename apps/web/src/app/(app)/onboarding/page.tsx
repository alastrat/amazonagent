"use client";

import { useState, useEffect } from "react";
import Link from "next/link";
import {
  useAssessmentStatus,
  useAssessmentGraph,
  useConnectSellerAccount,
  useSellerAccount,
  useStartAssessment,
  useProfile,
} from "@/hooks/use-assessment";
import { useAssessmentSSE } from "@/hooks/use-assessment-sse";
import {
  useActivateStrategyVersion as useActivateVersion,
  useStrategyVersions,
} from "@/hooks/use-strategy";
import { PageHeader } from "@/components/page-header";
import { StatusPill } from "@/components/status-pill";
import { DiscoveryGraph } from "@/components/discovery-graph";
import { DiscoveryProductTable } from "@/components/discovery-product-table";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import type {
  AssessmentGraph,
  AssessmentOutcome,
  TreeNode,
  UngatingStep,
} from "@/lib/types";

type Step = "connect" | "discover" | "reveal" | "commit";

const STEP_LABELS: Record<Step, string> = {
  connect: "Connect",
  discover: "Discover",
  reveal: "Reveal",
  commit: "Commit",
};
const STEPS: Step[] = ["connect", "discover", "reveal", "commit"];

export default function OnboardingPage() {
  const [step, setStep] = useState<Step>("connect");
  const [selectedNode, setSelectedNode] = useState<{ id: string; name: string; type: string } | null>(null);

  function changeStep(next: Step) {
    setSelectedNode(null);
    setStep(next);
  }

  // Connect form state
  const [clientId, setClientId] = useState("");
  const [clientSecret, setClientSecret] = useState("");
  const [refreshToken, setRefreshToken] = useState("");
  const [sellerId, setSellerId] = useState("");
  const [connectError, setConnectError] = useState<string | null>(null);

  // Hooks
  const connectAccount = useConnectSellerAccount();
  const { data: sellerAccount } = useSellerAccount();
  const startAssessment = useStartAssessment();
  const [graphPaused, setGraphPaused] = useState(false);
  const [eligibleOnly, setEligibleOnly] = useState(false);
  const { data: assessment } = useAssessmentStatus(step === "discover");
  const { data: graphData } = useAssessmentGraph(step === "discover" && !graphPaused);
  const { data: profileData, isLoading: profileLoading } = useProfile(
    step === "reveal" || step === "commit",
  );
  const activateVersion = useActivateVersion();
  const { data: versions } = useStrategyVersions();
  const draftVersion = versions?.find((v: any) => v.status === "draft");

  // If seller account is already connected, auto-advance past Step 1
  useEffect(() => {
    if (sellerAccount?.status === "valid" && step === "connect") {
      if (assessment?.status === "completed") {
        changeStep("reveal");
      } else if (assessment?.status === "running") {
        changeStep("discover");
      } else {
        // Account connected but no assessment — start it automatically
        startAssessment.mutate(undefined, {
          onSuccess: () => changeStep("discover"),
        });
      }
    }
  }, [sellerAccount, assessment, step]);

  // NOTE: auto-advance is handled by the SSE useEffect below (after sse hook)

  function handleRescan() {
    // Reset assessment and start fresh
    fetch(
      `${process.env.NEXT_PUBLIC_API_URL || "http://localhost:8081"}/assessment/reset`,
      {
        method: "DELETE",
        headers: { Authorization: `Bearer ${document.cookie.includes("token") ? "" : "dev-user-dev-tenant"}` },
      },
    ).then(() => {
      startAssessment.mutate(undefined, {
        onSuccess: () => changeStep("discover"),
      });
    });
  }

  function handleConnect() {
    setConnectError(null);
    connectAccount.mutate(
      {
        sp_api_client_id: clientId,
        sp_api_client_secret: clientSecret,
        sp_api_refresh_token: refreshToken,
        seller_id: sellerId,
      },
      {
        onSuccess: (account) => {
          if (account.status === "invalid") {
            setConnectError(
              account.error_message || "Credentials are invalid. Please check and try again.",
            );
            return;
          }
          // Start the assessment automatically after connecting
          startAssessment.mutate(undefined, {
            onSuccess: () => changeStep("discover"),
          });
        },
        onError: (err) => {
          setConnectError(err instanceof Error ? err.message : "Failed to connect account");
        },
      },
    );
  }

  function handleApproveStrategy() {
    if (!draftVersion) return;
    activateVersion.mutate(draftVersion.id, {
      onSuccess: () => changeStep("commit"),
    });
  }

  // SSE streaming for real-time updates during scanning
  const isScanning = step === "discover" && assessment?.status === "running";
  const sse = useAssessmentSSE(isScanning && !graphPaused);

  const outcome: AssessmentOutcome | undefined = graphData?.outcome;
  const graph: AssessmentGraph | undefined = graphData?.graph;

  // Use SSE tree/products/stats when streaming, fall back to API data when not
  const hasSSEData = sse.connected && (sse.products.length > 0 || isScanning);
  const rawTree: TreeNode | undefined = hasSSEData ? sse.tree : graphData?.tree;
  const liveProducts = hasSSEData ? sse.products : graphData?.products;
  const liveStats = hasSSEData ? sse.stats : graphData?.stats;

  // Auto-advance: SSE-driven (primary) or API fallback (page loaded after scan finished)
  useEffect(() => {
    if (step !== "discover") return;
    if (sse.isComplete) {
      changeStep("reveal");
      return;
    }
    // Fallback: if SSE never connected and API says completed
    if (!sse.connected && graphData?.status === "completed") {
      changeStep("reveal");
    }
  }, [step, sse.isComplete, sse.connected, graphData?.status]);

  // Filter tree to only show eligible + ungatable branches when toggle is on
  function filterEligible(node: TreeNode): TreeNode | null {
    if (node.type === "brand") {
      const status = node.eligibility_status ?? (node.eligible ? "eligible" : "restricted");
      return status === "eligible" || status === "ungatable" ? node : null;
    }
    if (node.children) {
      const filteredChildren = node.children
        .map(filterEligible)
        .filter((c): c is TreeNode => c !== null);
      if (filteredChildren.length === 0 && node.type !== "root") return null;
      return { ...node, children: filteredChildren };
    }
    return node;
  }

  const tree = eligibleOnly && rawTree ? filterEligible(rawTree) ?? rawTree : rawTree;

  return (
    <div className="space-y-6">
      <PageHeader
        title="Get Started"
        description="Connect your Amazon account and discover your selling opportunities"
      />

      {/* Step indicators */}
      <div className="flex gap-2">
        {STEPS.map((s, i) => (
          <div
            key={s}
            className={`flex items-center gap-2 rounded-full px-3 py-1 text-xs font-medium ${
              s === step
                ? "bg-primary text-primary-foreground"
                : STEPS.indexOf(step) > i
                  ? "bg-green-100 text-green-700"
                  : "bg-muted text-muted-foreground"
            }`}
          >
            {i + 1}. {STEP_LABELS[s]}
          </div>
        ))}
      </div>

      {/* ──────────────── Step 1: Connect ──────────────── */}
      {step === "connect" && (
        <Card>
          <CardHeader>
            <CardTitle>Connect Your Amazon Seller Account</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <p className="text-sm text-muted-foreground">
              To analyze your account, we need your SP-API credentials. You can find these in your
              Amazon Seller Central developer settings.
            </p>

            <div className="space-y-3 max-w-md">
              <div className="space-y-1.5">
                <label className="text-sm font-medium">SP-API Client ID</label>
                <Input
                  placeholder="amzn1.application-oa2-client.abc..."
                  value={clientId}
                  onChange={(e) => setClientId(e.target.value)}
                />
              </div>
              <div className="space-y-1.5">
                <label className="text-sm font-medium">SP-API Client Secret</label>
                <Input
                  type="password"
                  placeholder="Your client secret"
                  value={clientSecret}
                  onChange={(e) => setClientSecret(e.target.value)}
                />
              </div>
              <div className="space-y-1.5">
                <label className="text-sm font-medium">Refresh Token</label>
                <Input
                  type="password"
                  placeholder="Atzr|..."
                  value={refreshToken}
                  onChange={(e) => setRefreshToken(e.target.value)}
                />
              </div>
              <div className="space-y-1.5">
                <label className="text-sm font-medium">Seller ID</label>
                <Input
                  placeholder="e.g. A2EXAMPLE1234"
                  value={sellerId}
                  onChange={(e) => setSellerId(e.target.value)}
                />
              </div>
            </div>

            {connectError && (
              <div className="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-700">
                {connectError}
              </div>
            )}

            <Button
              onClick={handleConnect}
              disabled={
                !clientId ||
                !clientSecret ||
                !refreshToken ||
                !sellerId ||
                connectAccount.isPending ||
                startAssessment.isPending
              }
            >
              {connectAccount.isPending || startAssessment.isPending
                ? "Connecting..."
                : "Connect Amazon Account"}
            </Button>

            <div className="border-t pt-3 mt-2">
              <p className="text-xs text-muted-foreground">
                Coming soon: Connect with Amazon (one-click OAuth)
              </p>
            </div>
          </CardContent>
        </Card>
      )}

      {/* ──────────────── Step 2: Discover ──────────────── */}
      {step === "discover" && (
        <div className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle>Discovering Your Opportunities</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="flex items-center gap-3">
                <StatusPill status={assessment?.status ?? "running"} />
                <span className="text-sm text-muted-foreground">
                  {sse.currentCategory
                    ? `Scanning ${sse.currentCategory}...`
                    : "Searching categories and checking eligibility..."}
                </span>
              </div>

              {/* Progress bar */}
              {liveStats && (
                <div className="space-y-1">
                  <div className="h-2 w-full overflow-hidden rounded-full bg-muted">
                    <div
                      className="h-full rounded-full bg-primary transition-all duration-500"
                      style={{
                        width: `${
                          liveStats.categories_total > 0
                            ? Math.round(
                                (liveStats.categories_scanned / liveStats.categories_total) *
                                  100,
                              )
                            : 0
                        }%`,
                      }}
                    />
                  </div>
                  <p className="text-xs text-muted-foreground">
                    {liveStats.categories_scanned}/{liveStats.categories_total} categories
                    scanned
                  </p>
                </div>
              )}

              {/* Running stats */}
              {liveStats && (
                <div className="flex gap-6 text-sm">
                  <div>
                    <span className="font-medium text-green-600">{liveStats.eligible_products}</span>{" "}
                    <span className="text-muted-foreground">eligible</span>
                  </div>
                  {liveStats.ungatable_products > 0 && (
                    <div>
                      <span className="font-medium text-amber-600">{liveStats.ungatable_products}</span>{" "}
                      <span className="text-muted-foreground">can apply</span>
                    </div>
                  )}
                  <div>
                    <span className="font-medium">{liveStats.open_brands}</span>{" "}
                    <span className="text-muted-foreground">open brands</span>
                  </div>
                  <div>
                    <span className="font-medium text-red-600">{liveStats.restricted_products}</span>{" "}
                    <span className="text-muted-foreground">restricted</span>
                  </div>
                  {liveStats.qualified_products != null && liveStats.qualified_products > 0 && (
                    <div>
                      <span className="font-medium">{liveStats.qualified_products}</span>{" "}
                      <span className="text-muted-foreground">profitable</span>
                    </div>
                  )}
                </div>
              )}
            </CardContent>
          </Card>

          {/* Graph visualization */}
          {tree && (
            <Card>
              <CardHeader className="flex flex-row items-center justify-between">
                <CardTitle className="text-base">Discovery Graph</CardTitle>
                <div className="flex items-center gap-2">
                  <label className="flex items-center gap-1.5 text-xs text-muted-foreground cursor-pointer">
                    <input
                      type="checkbox"
                      checked={eligibleOnly}
                      onChange={(e) => setEligibleOnly(e.target.checked)}
                      className="rounded border-gray-300"
                    />
                    Eligible only
                  </label>
                  {step === "discover" && (
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => setGraphPaused((p) => !p)}
                    >
                      {graphPaused ? "Resume" : "Pause"}
                    </Button>
                  )}
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={handleRescan}
                    disabled={assessment?.status === "running"}
                  >
                    Rescan
                  </Button>
                </div>
              </CardHeader>
              <CardContent>
                <DiscoveryGraph
                  tree={tree}
                  products={liveProducts}
                  onNodeClick={(node) =>
                    setSelectedNode((prev) =>
                      prev?.id === node.id ? null : node,
                    )
                  }
                  height={440}
                />
                {liveProducts && liveProducts.length > 0 && (
                  <div className="mt-4">
                    <DiscoveryProductTable
                      products={liveProducts}
                      selectedNode={selectedNode}
                    />
                  </div>
                )}
              </CardContent>
            </Card>
          )}
        </div>
      )}

      {/* ──────────────── Step 3: Reveal ──────────────── */}
      {step === "reveal" && (
        <div className="space-y-6">
          {/* Summary stats */}
          <Card>
            <CardHeader>
              <CardTitle>Assessment Results</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="grid grid-cols-4 gap-3">
                <div className="rounded-lg border bg-muted/30 p-4 text-center">
                  <div className="text-2xl font-bold text-green-500">
                    {liveStats?.eligible_products ?? 0}
                  </div>
                  <div className="text-xs text-muted-foreground mt-1">Eligible Products</div>
                </div>
                <div className="rounded-lg border bg-muted/30 p-4 text-center">
                  <div className="text-2xl font-bold text-amber-500">
                    {liveStats?.ungatable_products ?? 0}
                  </div>
                  <div className="text-xs text-muted-foreground mt-1">Can Apply for Approval</div>
                </div>
                <div className="rounded-lg border bg-muted/30 p-4 text-center">
                  <div className="text-2xl font-bold text-red-500">
                    {liveStats?.restricted_products ?? 0}
                  </div>
                  <div className="text-xs text-muted-foreground mt-1">Restricted</div>
                </div>
                <div className="rounded-lg border bg-muted/30 p-4 text-center">
                  <div className="text-2xl font-bold text-indigo-400">
                    {liveStats?.categories_scanned ?? 0}
                  </div>
                  <div className="text-xs text-muted-foreground mt-1">Categories Scanned</div>
                </div>
              </div>
              <p className="text-sm text-muted-foreground">
                Across{" "}
                <span className="font-medium text-foreground">{liveStats?.open_brands ?? 0} brands</span>,
                you have immediate access to{" "}
                <span className="font-medium text-green-500">{liveStats?.eligible_products ?? 0} products</span>
                {(liveStats?.ungatable_products ?? 0) > 0 && (
                  <> and can request approval for{" "}
                    <span className="font-medium text-amber-500">{liveStats?.ungatable_products} more</span>
                  </>
                )}.
              </p>
            </CardContent>
          </Card>

          {/* Ungating roadmap (only if no opportunities) */}
          {outcome && !outcome.has_opportunities && outcome.ungating && (
            <Card>
              <CardHeader>
                <CardTitle>Ungating Roadmap</CardTitle>
              </CardHeader>
              <CardContent className="space-y-4">
                <p className="text-sm text-muted-foreground">
                  Your account is currently restricted in most categories. Here is your path forward.
                </p>
                {outcome.ungating.recommended_path.map((s: UngatingStep) => (
                  <div key={s.order} className="rounded-lg border p-4 space-y-1">
                    <div className="flex items-center gap-2">
                      <span className="flex h-6 w-6 items-center justify-center rounded-full bg-primary text-xs font-bold text-primary-foreground">
                        {s.order}
                      </span>
                      <span className="font-medium text-sm">Get Ungated in {s.category}</span>
                      <span
                        className={`ml-auto rounded-full px-2 py-0.5 text-xs font-medium ${
                          s.difficulty === "easy"
                            ? "bg-green-100 text-green-700"
                            : s.difficulty === "medium"
                              ? "bg-yellow-100 text-yellow-700"
                              : "bg-red-100 text-red-700"
                        }`}
                      >
                        {s.difficulty}
                      </span>
                    </div>
                    <p className="text-sm text-muted-foreground pl-8">{s.action}</p>
                    <div className="flex gap-4 pl-8 text-xs text-muted-foreground">
                      <span>~{s.est_days} days</span>
                      <span>{s.impact}</span>
                    </div>
                  </div>
                ))}
              </CardContent>
            </Card>
          )}

          {/* Graph + filterable product table */}
          <Card>
            <CardHeader className="flex flex-row items-center justify-between space-y-0">
              <CardTitle className="text-base">Your Opportunity Map</CardTitle>
              <div className="flex items-center gap-3">
                <label className="flex items-center gap-1.5 text-sm">
                  <input
                    type="checkbox"
                    checked={eligibleOnly}
                    onChange={(e) => setEligibleOnly(e.target.checked)}
                    className="rounded"
                  />
                  Eligible only
                </label>
                <Button variant="outline" size="sm" onClick={handleRescan}>
                  Rescan
                </Button>
              </div>
            </CardHeader>
            <CardContent>
              {tree && (
                <DiscoveryGraph
                  tree={tree}
                  products={liveProducts}
                  onNodeClick={(node) =>
                    setSelectedNode((prev) =>
                      prev?.id === node.id ? null : node,
                    )
                  }
                  height={440}
                />
              )}
            </CardContent>
            {liveProducts && liveProducts.length > 0 && (
              <>
                <div className="border-t" />
                <CardContent className="pt-4">
                  <DiscoveryProductTable
                    products={liveProducts}
                    selectedNode={selectedNode}
                    showAllByDefault
                  />
                </CardContent>
              </>
            )}
          </Card>

          <div className="flex justify-end">
            <Button onClick={() => changeStep("commit")}>Continue to Approval &rarr;</Button>
          </div>
        </div>
      )}

      {/* ──────────────── Step 4: Commit ──────────────── */}
      {step === "commit" && (
        <Card>
          <CardHeader>
            <CardTitle>Approve Your Strategy</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <p className="text-sm text-muted-foreground">
              Review complete. Approve to activate your personalized sourcing strategy and start
              receiving product suggestions.
            </p>
            <div className="flex gap-3">
              <Button onClick={() => changeStep("reveal")} variant="outline">
                Back to Review
              </Button>
              <Button onClick={handleApproveStrategy} disabled={activateVersion.isPending}>
                {activateVersion.isPending
                  ? "Activating..."
                  : outcome?.has_opportunities
                    ? "Approve Strategy"
                    : "Start Ungating Plan"}
              </Button>
            </div>
            {activateVersion.isSuccess && (
              <div className="rounded-lg border border-green-200 bg-green-50 p-3 text-sm text-green-700">
                Strategy activated!{" "}
                <Link href="/strategy" className="font-medium underline">
                  View your strategy dashboard
                </Link>
              </div>
            )}
          </CardContent>
        </Card>
      )}
    </div>
  );
}
