"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { apiClient } from "@/lib/api-client";
import { queryKeys } from "@/lib/query-keys";

export function useScans() {
  return useQuery({
    queryKey: queryKeys.scans.all,
    queryFn: () => apiClient.getScans(),
  });
}

export function useScan(id: string, isActive: boolean) {
  return useQuery({
    queryKey: queryKeys.scans.detail(id),
    queryFn: () => apiClient.getScan(id),
    enabled: !!id,
    refetchInterval: (query) => {
      const status = query.state.data?.status;
      if (status === "completed" || status === "failed") return false;
      return isActive ? 2000 : false;
    },
  });
}

export function useUploadPriceList() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ file, distributor }: { file: File; distributor: string }) =>
      apiClient.uploadPriceList(file, distributor),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.scans.all });
    },
  });
}

export function usePollScanJob(id: string) {
  return useScan(id, true);
}
