"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { apiClient } from "@/lib/api-client";
import { queryKeys } from "@/lib/query-keys";
import type { Criteria } from "@/lib/types";

export function useCampaigns() {
  return useQuery({
    queryKey: queryKeys.campaigns.all,
    queryFn: () => apiClient.getCampaigns(),
  });
}

export function useCampaign(id: string) {
  return useQuery({
    queryKey: queryKeys.campaigns.detail(id),
    queryFn: () => apiClient.getCampaign(id),
  });
}

export function useCreateCampaign() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: { type: string; trigger_type: string; criteria: Criteria }) =>
      apiClient.createCampaign(data),
    onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.campaigns.all }),
  });
}
