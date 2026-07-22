import { apiFetch } from "./client";
import {
  CategoryCountListSchema,
  ModelInventoryListSchema,
  type CategoryCount,
  type ModelInventoryItem,
} from "./models-types";

export function fetchModels(category?: string): Promise<ModelInventoryItem[]> {
  return apiFetch("/admin/api/v1/models", {
    query: { category },
    schema: ModelInventoryListSchema,
  }) as Promise<ModelInventoryItem[]>;
}

export function fetchModelCategories(): Promise<CategoryCount[]> {
  return apiFetch("/admin/api/v1/models/categories", {
    schema: CategoryCountListSchema,
  }) as Promise<CategoryCount[]>;
}
