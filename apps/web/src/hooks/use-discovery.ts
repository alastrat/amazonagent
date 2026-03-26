"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { apiClient } from "@/lib/api-client";
import { queryKeys } from "@/lib/query-keys";
import type { DiscoveryConfig } from "@/lib/types";

export function useDiscovery() {
  return useQuery({
    queryKey: queryKeys.discovery,
    queryFn: () => apiClient.getDiscovery(),
  });
}

export function useUpdateDiscovery() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: Partial<DiscoveryConfig>) =>
      apiClient.updateDiscovery(data),
    onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.discovery }),
  });
}
