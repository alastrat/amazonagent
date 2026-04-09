"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { apiClient } from "@/lib/api-client";
import { queryKeys } from "@/lib/query-keys";

export function useActiveStrategy() {
  return useQuery({
    queryKey: queryKeys.strategy.active,
    queryFn: () => apiClient.getActiveStrategy(),
  });
}

export function useStrategyVersions() {
  return useQuery({
    queryKey: queryKeys.strategy.versions,
    queryFn: () => apiClient.getStrategyVersions(),
  });
}

export function useStrategyVersion(id: string) {
  return useQuery({
    queryKey: queryKeys.strategy.detail(id),
    queryFn: () => apiClient.getStrategyVersion(id),
    enabled: !!id,
  });
}

export function useActivateStrategyVersion() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => apiClient.activateStrategyVersion(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.strategy.active });
      qc.invalidateQueries({ queryKey: queryKeys.strategy.versions });
    },
  });
}

export function useRollbackStrategyVersion() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => apiClient.rollbackStrategyVersion(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.strategy.active });
      qc.invalidateQueries({ queryKey: queryKeys.strategy.versions });
    },
  });
}
