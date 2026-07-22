import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  deleteGuardrail,
  fetchGuardrails,
  fetchGuardrailTypes,
  upsertGuardrail,
} from "./guardrails";
import type { Guardrail, GuardrailTypeDef, UpsertGuardrailInput } from "./guardrails-types";

export function useGuardrails() {
  const queryClient = useQueryClient();

  const query = useQuery<Guardrail[], Error>({
    queryKey: ["guardrails"],
    queryFn: fetchGuardrails,
  });

  const typesQuery = useQuery<GuardrailTypeDef[], Error>({
    queryKey: ["guardrail-types"],
    queryFn: fetchGuardrailTypes,
  });

  const upsertMutation = useMutation({
    mutationFn: (input: UpsertGuardrailInput) => upsertGuardrail(input),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["guardrails"] });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (name: string) => deleteGuardrail(name),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["guardrails"] });
    },
  });

  return {
    ...query,
    types: typesQuery.data ?? [],
    typesLoading: typesQuery.isLoading,
    upsertMutation,
    deleteMutation,
  };
}
