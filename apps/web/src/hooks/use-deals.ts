"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { apiClient } from "@/lib/api-client";
import { queryKeys } from "@/lib/query-keys";

export function useDeals(params?: Record<string, string>) {
  return useQuery({
    queryKey: queryKeys.deals.list(params || {}),
    queryFn: () => apiClient.getDeals(params),
  });
}

export function useDeal(id: string) {
  return useQuery({
    queryKey: queryKeys.deals.detail(id),
    queryFn: () => apiClient.getDeal(id),
  });
}

export function useApproveDeal() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => apiClient.approveDeal(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.deals.all }),
  });
}

export function useRejectDeal() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, reason }: { id: string; reason?: string }) =>
      apiClient.rejectDeal(id, reason),
    onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.deals.all }),
  });
}
