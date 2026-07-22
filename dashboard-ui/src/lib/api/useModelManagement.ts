import {
  useMutation,
  useQuery,
  useQueryClient,
  type UseMutationResult,
  type UseQueryResult,
} from "@tanstack/react-query";
import { fetchAliases, upsertAlias, deleteAlias } from "./aliases";
import type { AliasView, UpsertAliasInput } from "./aliases-types";
import {
  deleteModelOverride,
  fetchModelOverrides,
  upsertModelOverride,
} from "./model-overrides";
import {
  deleteModelPricingOverride,
  exportModelPricingOverrides,
  fetchModelPricing,
  fetchModelPricingBackups,
  importModelPricingOverrides,
  restoreModelPricingBackup,
  upsertModelPricingOverride,
} from "./model-pricing";
import type {
  ModelOverrideView,
  UpsertModelOverrideInput,
} from "./model-overrides-types";
import type {
  ImportModelPricingInput,
  ModelPricingBackup,
  ModelPricingImportResponse,
  ModelPricingView,
  UpsertModelPricingInput,
} from "./model-pricing-types";

export function useAliases(): UseQueryResult<AliasView[], Error> {
  return useQuery<AliasView[], Error>({
    queryKey: ["aliases"],
    queryFn: fetchAliases,
    staleTime: 30_000,
    retry: (failureCount, error) =>
      !("status" in error && (error as { status: number }).status === 503) && failureCount < 1,
  });
}

export function useModelOverrides(): UseQueryResult<ModelOverrideView[], Error> {
  return useQuery<ModelOverrideView[], Error>({
    queryKey: ["model-overrides"],
    queryFn: fetchModelOverrides,
    staleTime: 30_000,
    retry: (failureCount, error) =>
      !("status" in error && (error as { status: number }).status === 503) && failureCount < 1,
  });
}

export function useUpsertAlias(): UseMutationResult<AliasView, Error, UpsertAliasInput> {
  const qc = useQueryClient();
  return useMutation<AliasView, Error, UpsertAliasInput>({
    mutationFn: upsertAlias,
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["aliases"] });
      qc.invalidateQueries({ queryKey: ["models"] });
    },
  });
}

export function useDeleteAlias(): UseMutationResult<void, Error, string> {
  const qc = useQueryClient();
  return useMutation<void, Error, string>({
    mutationFn: deleteAlias,
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["aliases"] });
      qc.invalidateQueries({ queryKey: ["models"] });
    },
  });
}

export function useUpsertModelOverride(): UseMutationResult<
  ModelOverrideView,
  Error,
  UpsertModelOverrideInput
> {
  const qc = useQueryClient();
  return useMutation<ModelOverrideView, Error, UpsertModelOverrideInput>({
    mutationFn: upsertModelOverride,
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["model-overrides"] });
      qc.invalidateQueries({ queryKey: ["models"] });
    },
  });
}

export function useDeleteModelOverride(): UseMutationResult<void, Error, string> {
  const qc = useQueryClient();
  return useMutation<void, Error, string>({
    mutationFn: deleteModelOverride,
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["model-overrides"] });
      qc.invalidateQueries({ queryKey: ["models"] });
    },
  });
}

export function useModelPricing(): UseQueryResult<ModelPricingView[], Error> {
  return useQuery<ModelPricingView[], Error>({
    queryKey: ["model-pricing"],
    queryFn: fetchModelPricing,
    staleTime: 30_000,
    retry: (failureCount, error) =>
      !("status" in error && (error as { status: number }).status === 503) && failureCount < 1,
  });
}

export function useUpsertModelPricing(): UseMutationResult<
  ModelPricingView,
  Error,
  UpsertModelPricingInput
> {
  const qc = useQueryClient();
  return useMutation<ModelPricingView, Error, UpsertModelPricingInput>({
    mutationFn: upsertModelPricingOverride,
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["model-pricing"] });
      qc.invalidateQueries({ queryKey: ["models"] });
    },
  });
}

export function useDeleteModelPricing(): UseMutationResult<void, Error, string> {
  const qc = useQueryClient();
  return useMutation<void, Error, string>({
    mutationFn: deleteModelPricingOverride,
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["model-pricing"] });
      qc.invalidateQueries({ queryKey: ["models"] });
    },
  });
}

export function useModelPricingBackups(): UseQueryResult<ModelPricingBackup[], Error> {
  return useQuery<ModelPricingBackup[], Error>({
    queryKey: ["model-pricing-backups"],
    queryFn: fetchModelPricingBackups,
    staleTime: 30_000,
  });
}

export function useExportModelPricing(): UseMutationResult<string, Error, void> {
  return useMutation<string, Error, void>({ mutationFn: exportModelPricingOverrides });
}

export function useImportModelPricing(): UseMutationResult<
  ModelPricingImportResponse,
  Error,
  ImportModelPricingInput
> {
  const qc = useQueryClient();
  return useMutation<ModelPricingImportResponse, Error, ImportModelPricingInput>({
    mutationFn: importModelPricingOverrides,
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["model-pricing"] });
      qc.invalidateQueries({ queryKey: ["model-pricing-backups"] });
      qc.invalidateQueries({ queryKey: ["models"] });
    },
  });
}

export function useRestoreModelPricingBackup(): UseMutationResult<
  { message: string; backup_name: string },
  Error,
  string
> {
  const qc = useQueryClient();
  return useMutation<{ message: string; backup_name: string }, Error, string>({
    mutationFn: restoreModelPricingBackup,
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["model-pricing"] });
      qc.invalidateQueries({ queryKey: ["model-pricing-backups"] });
      qc.invalidateQueries({ queryKey: ["models"] });
    },
  });
}
