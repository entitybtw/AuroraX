import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import { fetchModelCategories, fetchModels } from "./models";
import type { CategoryCount, ModelInventoryItem } from "./models-types";

export function useModels(category?: string): UseQueryResult<ModelInventoryItem[], Error> {
  return useQuery<ModelInventoryItem[], Error>({
    queryKey: ["models", category ?? "all"],
    queryFn: () => fetchModels(category),
    staleTime: 60_000,
  });
}

export function useModelCategories(): UseQueryResult<CategoryCount[], Error> {
  return useQuery<CategoryCount[], Error>({
    queryKey: ["models", "categories"],
    queryFn: fetchModelCategories,
    staleTime: 60_000,
  });
}
