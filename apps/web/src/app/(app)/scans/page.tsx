"use client";

import { useScans } from "@/hooks/use-scans";
import { PageHeader } from "@/components/page-header";
import { EmptyState } from "@/components/empty-state";
import { ScanProgressCard } from "@/components/scan-progress-card";
import { PriceListUploadDialog } from "@/components/price-list-upload-dialog";
import { Button } from "@/components/ui/button";

export default function ScansPage() {
  const { data: scans, isLoading } = useScans();

  const activeScans = scans?.filter((s) => s.status === "running" || s.status === "pending") ?? [];
  const completedScans = scans?.filter((s) => s.status !== "running" && s.status !== "pending") ?? [];

  return (
    <div className="space-y-6">
      <PageHeader
        title="Scan History"
        description={`${scans?.length ?? 0} scans`}
        action={
          <PriceListUploadDialog>
            <Button>Upload Price List</Button>
          </PriceListUploadDialog>
        }
      />

      {isLoading ? (
        <div>Loading...</div>
      ) : !scans || scans.length === 0 ? (
        <EmptyState
          title="No scans yet"
          description="Upload a price list to start scanning for profitable products."
          action={
            <PriceListUploadDialog>
              <Button>Upload Price List</Button>
            </PriceListUploadDialog>
          }
        />
      ) : (
        <>
          {activeScans.length > 0 && (
            <div className="space-y-4">
              <h2 className="text-lg font-medium">Active Scans</h2>
              <div className="grid gap-4 md:grid-cols-2">
                {activeScans.map((scan) => (
                  <ScanProgressCard key={scan.id} scan={scan} />
                ))}
              </div>
            </div>
          )}

          {completedScans.length > 0 && (
            <div className="space-y-4">
              <h2 className="text-lg font-medium">Completed Scans</h2>
              <div className="grid gap-4 md:grid-cols-2">
                {completedScans.map((scan) => (
                  <ScanProgressCard key={scan.id} scan={scan} />
                ))}
              </div>
            </div>
          )}
        </>
      )}
    </div>
  );
}
