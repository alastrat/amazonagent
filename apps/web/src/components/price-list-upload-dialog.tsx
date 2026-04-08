"use client";

import { useState, useCallback, useRef } from "react";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

const API_BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8081";

interface UploadResponse {
  scan_id: string;
  message?: string;
}

interface PriceListUploadDialogProps {
  children: React.ReactNode;
  onSuccess?: (scanId: string) => void;
  token?: string | null;
}

export function PriceListUploadDialog({
  children,
  onSuccess,
  token,
}: PriceListUploadDialogProps) {
  const [open, setOpen] = useState(false);
  const [file, setFile] = useState<File | null>(null);
  const [distributor, setDistributor] = useState("");
  const [isDragging, setIsDragging] = useState(false);
  const [isUploading, setIsUploading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [scanId, setScanId] = useState<string | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  const reset = useCallback(() => {
    setFile(null);
    setDistributor("");
    setError(null);
    setScanId(null);
    setIsUploading(false);
    setIsDragging(false);
  }, []);

  const handleOpenChange = useCallback(
    (nextOpen: boolean) => {
      setOpen(nextOpen);
      if (!nextOpen) reset();
    },
    [reset]
  );

  const handleDrop = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    setIsDragging(false);
    const dropped = e.dataTransfer.files[0];
    if (dropped && dropped.name.endsWith(".csv")) {
      setFile(dropped);
      setError(null);
    } else {
      setError("Only .csv files are accepted");
    }
  }, []);

  const handleDragOver = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    setIsDragging(true);
  }, []);

  const handleDragLeave = useCallback(() => {
    setIsDragging(false);
  }, []);

  const handleFileChange = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      const selected = e.target.files?.[0];
      if (selected) {
        setFile(selected);
        setError(null);
      }
    },
    []
  );

  const handleSubmit = useCallback(async () => {
    if (!file || !distributor.trim()) return;

    setIsUploading(true);
    setError(null);

    try {
      const formData = new FormData();
      formData.append("file", file);
      formData.append("distributor", distributor.trim());

      const headers: Record<string, string> = {};
      if (token) {
        headers["Authorization"] = `Bearer ${token}`;
      }

      const res = await fetch(`${API_BASE}/pricelist/upload-funnel`, {
        method: "POST",
        headers,
        body: formData,
      });

      if (!res.ok) {
        const body = await res.json().catch(() => ({}));
        throw new Error(body.error || `Upload failed: ${res.status}`);
      }

      const data: UploadResponse = await res.json();
      setScanId(data.scan_id);
      onSuccess?.(data.scan_id);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Upload failed");
    } finally {
      setIsUploading(false);
    }
  }, [file, distributor, token, onSuccess]);

  const canSubmit = file && distributor.trim() && !isUploading;

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogTrigger render={<span />}>{children}</DialogTrigger>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Upload Price List</DialogTitle>
          <DialogDescription>
            Upload a CSV price list to run through the funnel pipeline.
          </DialogDescription>
        </DialogHeader>

        {scanId ? (
          <div className="space-y-3 py-2">
            <div className="rounded-lg bg-green-50 p-3 text-sm text-green-800">
              Upload successful! Scan job started.
            </div>
            <div className="text-sm text-muted-foreground">
              Scan ID:{" "}
              <code className="rounded bg-muted px-1.5 py-0.5 text-xs font-mono">
                {scanId}
              </code>
            </div>
          </div>
        ) : (
          <div className="space-y-4">
            {/* Drop zone */}
            <div
              onDrop={handleDrop}
              onDragOver={handleDragOver}
              onDragLeave={handleDragLeave}
              onClick={() => fileInputRef.current?.click()}
              className={`flex cursor-pointer flex-col items-center justify-center rounded-lg border-2 border-dashed p-6 transition-colors ${
                isDragging
                  ? "border-primary bg-primary/5"
                  : "border-muted-foreground/25 hover:border-muted-foreground/50"
              }`}
            >
              <input
                ref={fileInputRef}
                type="file"
                accept=".csv"
                onChange={handleFileChange}
                className="hidden"
              />
              {file ? (
                <div className="text-center">
                  <p className="text-sm font-medium">{file.name}</p>
                  <p className="text-xs text-muted-foreground">
                    {(file.size / 1024).toFixed(1)} KB
                  </p>
                </div>
              ) : (
                <div className="text-center">
                  <p className="text-sm text-muted-foreground">
                    Drag and drop a CSV file here, or click to browse
                  </p>
                  <p className="mt-1 text-xs text-muted-foreground/70">
                    .csv files only
                  </p>
                </div>
              )}
            </div>

            {/* Distributor name */}
            <div className="space-y-1.5">
              <label
                htmlFor="distributor-name"
                className="text-sm font-medium"
              >
                Distributor Name
              </label>
              <Input
                id="distributor-name"
                placeholder="e.g. Global Wholesale Co."
                value={distributor}
                onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
                  setDistributor(e.target.value)
                }
              />
            </div>

            {error && (
              <p className="text-sm text-red-600">{error}</p>
            )}
          </div>
        )}

        <DialogFooter>
          {scanId ? (
            <Button onClick={() => handleOpenChange(false)}>Done</Button>
          ) : (
            <Button
              onClick={handleSubmit}
              disabled={!canSubmit}
            >
              {isUploading ? "Uploading..." : "Upload & Process"}
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
