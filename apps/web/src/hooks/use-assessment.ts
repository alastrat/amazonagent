"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { apiClient } from "@/lib/api-client";
import { queryKeys } from "@/lib/query-keys";

export function useStartAssessment() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: { account_age_days: number; active_listings: number; stated_capital: number }) =>
      apiClient.startAssessment(data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.assessment.status });
      qc.invalidateQueries({ queryKey: queryKeys.assessment.profile });
    },
  });
}

export function useAssessmentStatus(isPolling: boolean) {
  return useQuery({
    queryKey: queryKeys.assessment.status,
    queryFn: () => apiClient.getAssessmentStatus(),
    refetchInterval: (query) => {
      const status = query.state.data?.status;
      if (status === "completed" || status === "failed") return false;
      return isPolling ? 2000 : false;
    },
  });
}

export function useProfile(enabled = true) {
  return useQuery({
    queryKey: queryKeys.assessment.profile,
    queryFn: () => apiClient.getAssessmentProfile(),
    enabled,
  });
}
