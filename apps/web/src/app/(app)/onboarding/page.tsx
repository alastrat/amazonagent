"use client";

import { useState, useEffect } from "react";
import Link from "next/link";
import { useAssessmentStatus, useProfile, useStartAssessment } from "@/hooks/use-assessment";
import { useActivateStrategyVersion as useActivateVersion, useStrategyVersions } from "@/hooks/use-strategy";
import { PageHeader } from "@/components/page-header";
import { StatusPill } from "@/components/status-pill";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

type Step = "connect" | "discover" | "reveal" | "commit";

export default function OnboardingPage() {
  const [step, setStep] = useState<Step>("connect");
  const [accountAgeDays, setAccountAgeDays] = useState("");
  const [activeListings, setActiveListings] = useState("");
  const [statedCapital, setStatedCapital] = useState("");

  const startAssessment = useStartAssessment();
  const { data: assessment, isLoading: assessmentLoading } = useAssessmentStatus(step === "discover");
  const { data: profileData, isLoading: profileLoading } = useProfile(step === "reveal" || step === "commit");
  const activateVersion = useActivateVersion();
  const { data: versions } = useStrategyVersions();
  const draftVersion = versions?.find((v: any) => v.status === "draft");

  // Auto-advance from discover -> reveal when assessment completes
  useEffect(() => {
    if (step === "discover" && assessment?.status === "completed") {
      setStep("reveal");
    }
  }, [step, assessment?.status]);

  function handleStartAssessment() {
    startAssessment.mutate(
      {
        account_age_days: Number(accountAgeDays),
        active_listings: Number(activeListings),
        stated_capital: Number(statedCapital),
      },
      { onSuccess: () => setStep("discover") },
    );
  }

  function handleApproveStrategy() {
    if (!draftVersion) return;
    activateVersion.mutate(draftVersion.id, {
      onSuccess: () => setStep("commit"),
    });
  }

  return (
    <div className="space-y-6">
      <PageHeader
        title="Get Started"
        description="We'll assess your account and build a custom sourcing strategy"
      />

      {/* Step indicators */}
      <div className="flex gap-2">
        {(["connect", "discover", "reveal", "commit"] as Step[]).map((s, i) => (
          <div
            key={s}
            className={`flex items-center gap-2 rounded-full px-3 py-1 text-xs font-medium ${
              s === step
                ? "bg-primary text-primary-foreground"
                : (["connect", "discover", "reveal", "commit"] as Step[]).indexOf(step) > i
                  ? "bg-green-100 text-green-700"
                  : "bg-muted text-muted-foreground"
            }`}
          >
            {i + 1}. {s.charAt(0).toUpperCase() + s.slice(1)}
          </div>
        ))}
      </div>

      {/* Step 1: Connect */}
      {step === "connect" && (
        <Card>
          <CardHeader>
            <CardTitle>Tell us about your account</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <label className="text-sm font-medium">Account Age (days)</label>
              <Input
                type="number"
                placeholder="e.g. 365"
                value={accountAgeDays}
                onChange={(e) => setAccountAgeDays(e.target.value)}
                className="max-w-xs"
              />
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">Active Listings</label>
              <Input
                type="number"
                placeholder="e.g. 50"
                value={activeListings}
                onChange={(e) => setActiveListings(e.target.value)}
                className="max-w-xs"
              />
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">Starting Capital ($)</label>
              <Input
                type="number"
                placeholder="e.g. 5000"
                value={statedCapital}
                onChange={(e) => setStatedCapital(e.target.value)}
                className="max-w-xs"
              />
            </div>
            <Button
              onClick={handleStartAssessment}
              disabled={!accountAgeDays || !activeListings || !statedCapital || startAssessment.isPending}
            >
              {startAssessment.isPending ? "Starting..." : "Start Assessment"}
            </Button>
          </CardContent>
        </Card>
      )}

      {/* Step 2: Discover */}
      {step === "discover" && (
        <Card>
          <CardHeader>
            <CardTitle>Assessment in Progress</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="flex items-center gap-3">
              <StatusPill status={assessment?.status ?? "running"} />
              <span className="text-sm text-muted-foreground">
                Analyzing your account profile and category eligibility...
              </span>
            </div>
            <div className="h-2 w-full overflow-hidden rounded-full bg-muted">
              <div
                className="h-full rounded-full bg-primary transition-all"
                style={{
                  width: assessment?.status === "completed" ? "100%" : "60%",
                }}
              />
            </div>
          </CardContent>
        </Card>
      )}

      {/* Step 3: Reveal */}
      {step === "reveal" && (
        <div className="space-y-6">
          {profileLoading ? (
            <div>Loading...</div>
          ) : (
            <>
              {/* Profile archetype */}
              <Card>
                <CardHeader>
                  <CardTitle>Your Seller Profile</CardTitle>
                </CardHeader>
                <CardContent>
                  <div className="flex items-center gap-3">
                    <span className="inline-flex rounded-full bg-primary/10 px-3 py-1 text-sm font-semibold text-primary">
                      {profileData?.profile?.archetype ?? "Unknown"}
                    </span>
                    <span className="text-sm text-muted-foreground">
                      {assessment?.archetype ?? "Archetype determined by assessment"}
                    </span>
                  </div>
                </CardContent>
              </Card>

              {/* Fingerprint: Category eligibility */}
              <Card>
                <CardHeader>
                  <CardTitle>Category Eligibility</CardTitle>
                </CardHeader>
                <CardContent>
                  {profileData?.fingerprint?.categories && profileData.fingerprint.categories.length > 0 ? (
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

              {/* Strategy brief */}
              <Card>
                <CardHeader>
                  <CardTitle>Recommended Strategy</CardTitle>
                </CardHeader>
                <CardContent className="space-y-3">
                  {draftVersion?.goals && draftVersion.goals.length > 0 ? (
                    draftVersion.goals.map((goal: any, i: number) => (
                      <div key={i} className="flex items-center justify-between rounded-lg border p-3">
                        <div>
                          <p className="text-sm font-medium">{goal.type}</p>
                          <p className="text-xs text-muted-foreground">
                            Target: ${goal.target_amount?.toLocaleString()} in {goal.timeframe}
                          </p>
                        </div>
                      </div>
                    ))
                  ) : (
                    <p className="text-sm text-muted-foreground">No goals defined yet.</p>
                  )}
                </CardContent>
              </Card>

              <Button onClick={() => setStep("commit")}>Continue to Approval</Button>
            </>
          )}
        </div>
      )}

      {/* Step 4: Commit */}
      {step === "commit" && (
        <Card>
          <CardHeader>
            <CardTitle>Approve Your Strategy</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <p className="text-sm text-muted-foreground">
              Review complete. Approve to activate your personalized sourcing strategy
              and start receiving product suggestions.
            </p>
            <div className="flex gap-3">
              <Button onClick={() => setStep("reveal")} variant="outline">
                Back to Review
              </Button>
              <Button
                onClick={handleApproveStrategy}
                disabled={activateVersion.isPending}
              >
                {activateVersion.isPending ? "Activating..." : "Approve Strategy"}
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
