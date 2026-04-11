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
import {
  useActivateStrategyVersion as useActivateVersion,
  useStrategyVersions,
} from "@/hooks/use-strategy";
import { PageHeader } from "@/components/page-header";
import { StatusPill } from "@/components/status-pill";
import { DiscoveryGraph } from "@/components/discovery-graph";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import type {
  AssessmentGraph,
  AssessmentOutcome,
  CategorySummary,
  ProductRecommendation,
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
  const { data: assessment } = useAssessmentStatus(step === "discover");
  const { data: graphData } = useAssessmentGraph(step === "discover");
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
        setStep("reveal");
      } else if (assessment?.status === "running") {
        setStep("discover");
      } else {
        // Account connected but no assessment — start it automatically
        startAssessment.mutate(undefined, {
          onSuccess: () => setStep("discover"),
        });
      }
    }
  }, [sellerAccount, assessment, step]);

  // Auto-advance from discover -> reveal when assessment completes
  useEffect(() => {
    if (step === "discover" && graphData?.status === "completed") {
      setStep("reveal");
    }
  }, [step, graphData?.status]);

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
            onSuccess: () => setStep("discover"),
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
      onSuccess: () => setStep("commit"),
    });
  }

  const outcome: AssessmentOutcome | undefined = graphData?.outcome;
  const graph: AssessmentGraph | undefined = graphData?.graph;
  const tree: TreeNode | undefined = graphData?.tree;

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
                  Searching categories and checking eligibility...
                </span>
              </div>

              {/* Progress bar */}
              {graph?.stats && (
                <div className="space-y-1">
                  <div className="h-2 w-full overflow-hidden rounded-full bg-muted">
                    <div
                      className="h-full rounded-full bg-primary transition-all duration-500"
                      style={{
                        width: `${
                          graph.stats.categories_total > 0
                            ? Math.round(
                                (graph.stats.categories_scanned / graph.stats.categories_total) *
                                  100,
                              )
                            : 0
                        }%`,
                      }}
                    />
                  </div>
                  <p className="text-xs text-muted-foreground">
                    {graph.stats.categories_scanned}/{graph.stats.categories_total} categories
                    scanned
                  </p>
                </div>
              )}

              {/* Running stats */}
              {graph?.stats && (
                <div className="flex gap-6 text-sm">
                  <div>
                    <span className="font-medium">{graph.stats.eligible_products}</span>{" "}
                    <span className="text-muted-foreground">eligible products</span>
                  </div>
                  <div>
                    <span className="font-medium">{graph.stats.open_brands}</span>{" "}
                    <span className="text-muted-foreground">open brands</span>
                  </div>
                  <div>
                    <span className="font-medium">{graph.stats.restricted_products}</span>{" "}
                    <span className="text-muted-foreground">restricted</span>
                  </div>
                  {graph.stats.qualified_products != null && graph.stats.qualified_products > 0 && (
                    <div>
                      <span className="font-medium">{graph.stats.qualified_products}</span>{" "}
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
              <CardHeader>
                <CardTitle className="text-base">Discovery Graph</CardTitle>
              </CardHeader>
              <CardContent>
                <DiscoveryGraph tree={tree} width={680} height={440} />
              </CardContent>
            </Card>
          )}
        </div>
      )}

      {/* ──────────────── Step 3: Reveal ──────────────── */}
      {step === "reveal" && (
        <div className="space-y-6">
          {profileLoading ? (
            <div>Loading results...</div>
          ) : (
            <>
              {/* Outcome A: Opportunities found */}
              {outcome?.has_opportunities && outcome.opportunity && (
                <>
                  <Card>
                    <CardHeader>
                      <CardTitle>
                        Great News! You Can Sell in {outcome.eligible_categories} Categories
                      </CardTitle>
                    </CardHeader>
                    <CardContent>
                      <p className="text-sm text-muted-foreground mb-4">
                        We found {outcome.qualified_count} profitable products across your eligible
                        categories.
                      </p>

                      {/* Category table */}
                      {outcome.opportunity.categories.length > 0 && (
                        <div className="rounded-lg border">
                          <table className="w-full text-sm">
                            <thead>
                              <tr className="border-b bg-muted/50">
                                <th className="px-4 py-2 text-left font-medium">Category</th>
                                <th className="px-4 py-2 text-left font-medium">
                                  Qualified Products
                                </th>
                                <th className="px-4 py-2 text-left font-medium">Avg Margin</th>
                                <th className="px-4 py-2 text-left font-medium">Open Rate</th>
                              </tr>
                            </thead>
                            <tbody>
                              {outcome.opportunity.categories.map((cat: CategorySummary) => (
                                <tr key={cat.category} className="border-b last:border-0">
                                  <td className="px-4 py-2">{cat.category}</td>
                                  <td className="px-4 py-2">{cat.qualified_count}</td>
                                  <td className="px-4 py-2">{cat.avg_margin_pct.toFixed(1)}%</td>
                                  <td className="px-4 py-2">
                                    {(cat.open_rate * 100).toFixed(0)}%
                                  </td>
                                </tr>
                              ))}
                            </tbody>
                          </table>
                        </div>
                      )}
                    </CardContent>
                  </Card>

                  {/* Top Opportunities */}
                  {outcome.opportunity.products.length > 0 && (
                    <Card>
                      <CardHeader>
                        <CardTitle>Top Opportunities</CardTitle>
                      </CardHeader>
                      <CardContent>
                        <div className="rounded-lg border">
                          <table className="w-full text-sm">
                            <thead>
                              <tr className="border-b bg-muted/50">
                                <th className="px-4 py-2 text-left font-medium">ASIN</th>
                                <th className="px-4 py-2 text-left font-medium">Title</th>
                                <th className="px-4 py-2 text-left font-medium">Brand</th>
                                <th className="px-4 py-2 text-left font-medium">Category</th>
                                <th className="px-4 py-2 text-left font-medium">Price</th>
                                <th className="px-4 py-2 text-left font-medium">Est. Margin</th>
                                <th className="px-4 py-2 text-left font-medium">Sellers</th>
                              </tr>
                            </thead>
                            <tbody>
                              {[...outcome.opportunity.products]
                                .sort((a, b) => b.est_margin_pct - a.est_margin_pct)
                                .slice(0, 20)
                                .map((p: ProductRecommendation) => (
                                  <tr key={p.asin} className="border-b last:border-0 hover:bg-muted/30">
                                    <td className="px-4 py-2 font-mono text-xs">{p.asin}</td>
                                    <td className="px-4 py-2 max-w-[200px] truncate">{p.title}</td>
                                    <td className="px-4 py-2 text-muted-foreground">{p.brand}</td>
                                    <td className="px-4 py-2 text-muted-foreground">{p.category}</td>
                                    <td className="px-4 py-2">${p.buy_box_price.toFixed(2)}</td>
                                    <td className="px-4 py-2">{p.est_margin_pct.toFixed(1)}%</td>
                                    <td className="px-4 py-2">{p.seller_count}</td>
                                  </tr>
                                ))}
                            </tbody>
                          </table>
                        </div>
                      </CardContent>
                    </Card>
                  )}

                  {/* Strategy goals */}
                  {outcome.opportunity.strategy.length > 0 && (
                    <Card>
                      <CardHeader>
                        <CardTitle>Your Strategy</CardTitle>
                      </CardHeader>
                      <CardContent className="space-y-3">
                        {outcome.opportunity.strategy.map((goal, i) => (
                          <div
                            key={i}
                            className="flex items-center justify-between rounded-lg border p-3"
                          >
                            <div>
                              <p className="text-sm font-medium">{goal.type}</p>
                              <p className="text-xs text-muted-foreground">
                                Target: ${goal.target_amount?.toLocaleString()} by{" "}
                                {new Date(goal.timeframe_end).toLocaleDateString()}
                              </p>
                            </div>
                          </div>
                        ))}
                      </CardContent>
                    </Card>
                  )}
                </>
              )}

              {/* Outcome B: Restricted — ungating roadmap */}
              {outcome && !outcome.has_opportunities && outcome.ungating && (
                <>
                  <Card>
                    <CardHeader>
                      <CardTitle>Your Account Needs Ungating</CardTitle>
                    </CardHeader>
                    <CardContent className="space-y-4">
                      <p className="text-sm text-muted-foreground">
                        We searched products across 20 categories. Your account is currently
                        restricted from selling profitably in those categories. This is normal for
                        new accounts — here is your path forward.
                      </p>

                      {outcome.ungating.estimated_timeline && (
                        <p className="text-sm font-medium">
                          Estimated timeline: {outcome.ungating.estimated_timeline}
                        </p>
                      )}
                    </CardContent>
                  </Card>

                  <Card>
                    <CardHeader>
                      <CardTitle>Ungating Roadmap</CardTitle>
                    </CardHeader>
                    <CardContent className="space-y-4">
                      {outcome.ungating.recommended_path.map((step: UngatingStep) => (
                        <div key={step.order} className="rounded-lg border p-4 space-y-1">
                          <div className="flex items-center gap-2">
                            <span className="flex h-6 w-6 items-center justify-center rounded-full bg-primary text-xs font-bold text-primary-foreground">
                              {step.order}
                            </span>
                            <span className="font-medium text-sm">
                              Get Ungated in {step.category}
                            </span>
                            <span
                              className={`ml-auto rounded-full px-2 py-0.5 text-xs font-medium ${
                                step.difficulty === "easy"
                                  ? "bg-green-100 text-green-700"
                                  : step.difficulty === "medium"
                                    ? "bg-yellow-100 text-yellow-700"
                                    : "bg-red-100 text-red-700"
                              }`}
                            >
                              {step.difficulty}
                            </span>
                          </div>
                          <p className="text-sm text-muted-foreground pl-8">{step.action}</p>
                          <div className="flex gap-4 pl-8 text-xs text-muted-foreground">
                            <span>~{step.est_days} days</span>
                            <span>{step.impact}</span>
                          </div>
                        </div>
                      ))}
                    </CardContent>
                  </Card>
                </>
              )}

              {/* Fallback: use existing profile data if outcome is not available */}
              {!outcome && profileData && (
                <>
                  <Card>
                    <CardHeader>
                      <CardTitle>Your Seller Profile</CardTitle>
                    </CardHeader>
                    <CardContent>
                      <div className="flex items-center gap-3">
                        <span className="inline-flex rounded-full bg-primary/10 px-3 py-1 text-sm font-semibold text-primary">
                          {profileData.profile?.archetype ?? "Unknown"}
                        </span>
                        <span className="text-sm text-muted-foreground">
                          {assessment?.archetype ?? "Archetype determined by assessment"}
                        </span>
                      </div>
                    </CardContent>
                  </Card>

                  <Card>
                    <CardHeader>
                      <CardTitle>Category Eligibility</CardTitle>
                    </CardHeader>
                    <CardContent>
                      {profileData.fingerprint?.categories &&
                      profileData.fingerprint.categories.length > 0 ? (
                        <div className="rounded-lg border">
                          <table className="w-full text-sm">
                            <thead>
                              <tr className="border-b bg-muted/50">
                                <th className="px-4 py-2 text-left font-medium">Category</th>
                                <th className="px-4 py-2 text-left font-medium">Status</th>
                                <th className="px-4 py-2 text-left font-medium">Open Rate</th>
                              </tr>
                            </thead>
                            <tbody>
                              {profileData.fingerprint.categories.map((cat: any) => (
                                <tr key={cat.name} className="border-b last:border-0">
                                  <td className="px-4 py-2">{cat.name}</td>
                                  <td className="px-4 py-2">
                                    <StatusPill status={cat.eligible ? "approved" : "rejected"} />
                                  </td>
                                  <td className="px-4 py-2 text-muted-foreground">
                                    {(cat.open_rate * 100).toFixed(1)}%
                                  </td>
                                </tr>
                              ))}
                            </tbody>
                          </table>
                        </div>
                      ) : (
                        <p className="text-sm text-muted-foreground">No category data available.</p>
                      )}
                    </CardContent>
                  </Card>

                  {draftVersion?.goals && draftVersion.goals.length > 0 && (
                    <Card>
                      <CardHeader>
                        <CardTitle>Recommended Strategy</CardTitle>
                      </CardHeader>
                      <CardContent className="space-y-3">
                        {draftVersion.goals.map((goal: any, i: number) => (
                          <div
                            key={i}
                            className="flex items-center justify-between rounded-lg border p-3"
                          >
                            <div>
                              <p className="text-sm font-medium">{goal.type}</p>
                              <p className="text-xs text-muted-foreground">
                                Target: ${goal.target_amount?.toLocaleString()} in {goal.timeframe}
                              </p>
                            </div>
                          </div>
                        ))}
                      </CardContent>
                    </Card>
                  )}
                </>
              )}

              {/* Hierarchical tree visualization */}
              {tree && (
                <Card>
                  <CardHeader>
                    <CardTitle className="text-base">
                      Eligibility Map{" "}
                      <span className="text-xs font-normal text-muted-foreground">
                        (click categories to expand/collapse)
                      </span>
                    </CardTitle>
                  </CardHeader>
                  <CardContent>
                    <DiscoveryGraph tree={tree} width={680} height={440} />
                  </CardContent>
                </Card>
              )}

              <Button onClick={() => setStep("commit")}>Continue to Approval</Button>
            </>
          )}
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
              <Button onClick={() => setStep("reveal")} variant="outline">
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
