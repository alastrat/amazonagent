"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { apiClient } from "@/lib/api-client";
import { queryKeys } from "@/lib/query-keys";

export function useCatalogProducts(params?: Record<string, string>) {
  return useQuery({
    queryKey: queryKeys.catalog.products(params || {}),
    queryFn: () => apiClient.getCatalogProducts(params),
  });
}

export function useCatalogStats() {
  return useQuery({
    queryKey: queryKeys.catalog.stats,
    queryFn: () => apiClient.getCatalogStats(),
  });
}

export function useEvaluateProducts() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (asins: string[]) => apiClient.evaluateProducts(asins),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.scans.all });
    },
  });
}
