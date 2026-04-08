"use client";

import { useQuery } from "@tanstack/react-query";
import { apiClient } from "@/lib/api-client";
import { queryKeys } from "@/lib/query-keys";

export function useBrands(params?: Record<string, string>) {
  return useQuery({
    queryKey: queryKeys.brands.list(params || {}),
    queryFn: () => apiClient.getBrands(params),
  });
}

export function useBrandProducts(brandId: string, params?: Record<string, string>) {
  return useQuery({
    queryKey: queryKeys.brands.products(brandId, params || {}),
    queryFn: () => apiClient.getBrandProducts(brandId, params),
    enabled: !!brandId,
  });
}
