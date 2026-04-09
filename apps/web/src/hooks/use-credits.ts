"use client";

import { useQuery } from "@tanstack/react-query";
import { apiClient } from "@/lib/api-client";
import { queryKeys } from "@/lib/query-keys";

export function useCredits() {
  return useQuery({
    queryKey: queryKeys.credits.account,
    queryFn: () => apiClient.getCredits(),
  });
}

export function useCreditTransactions() {
  return useQuery({
    queryKey: queryKeys.credits.transactions,
    queryFn: () => apiClient.getCreditTransactions(),
  });
}
