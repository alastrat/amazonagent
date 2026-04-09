"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { apiClient } from "@/lib/api-client";
import { queryKeys } from "@/lib/query-keys";

export function usePendingSuggestions() {
  return useQuery({
    queryKey: queryKeys.suggestions.pending,
    queryFn: () => apiClient.getPendingSuggestions(),
  });
}

export function useAllSuggestions() {
  return useQuery({
    queryKey: queryKeys.suggestions.all,
    queryFn: () => apiClient.getAllSuggestions(),
  });
}

export function useAcceptSuggestion() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => apiClient.acceptSuggestion(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.suggestions.pending });
      qc.invalidateQueries({ queryKey: queryKeys.suggestions.all });
    },
  });
}

export function useDismissSuggestion() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => apiClient.dismissSuggestion(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.suggestions.pending });
      qc.invalidateQueries({ queryKey: queryKeys.suggestions.all });
    },
  });
}
