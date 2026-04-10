"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { apiClient } from "@/lib/api-client";
import { queryKeys } from "@/lib/query-keys";
import type { ConnectSellerAccountRequest } from "@/lib/types";

export function useConnectSellerAccount() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (credentials: ConnectSellerAccountRequest) =>
      apiClient.connectSellerAccount(credentials),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.sellerAccount });
    },
  });
}

export function useSellerAccount() {
  return useQuery({
    queryKey: queryKeys.sellerAccount,
    queryFn: () => apiClient.getSellerAccount(),
  });
}

export function useDisconnectSellerAccount() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => apiClient.disconnectSellerAccount(),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.sellerAccount });
    },
  });
}

export function useStartAssessment() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => apiClient.startAssessment(),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.assessment.status });
      qc.invalidateQueries({ queryKey: queryKeys.assessment.profile });
      qc.invalidateQueries({ queryKey: queryKeys.assessment.graph });
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

export function useAssessmentGraph(isPolling: boolean) {
  return useQuery({
    queryKey: queryKeys.assessment.graph,
    queryFn: () => apiClient.getAssessmentGraph(),
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
