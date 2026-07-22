import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  deactivateWorkflow,
  fetchWorkflows,
  upsertWorkflow,
} from "./workflows";
import type { UpsertWorkflowInput, Workflow } from "./workflows-types";

export function useWorkflows() {
  const queryClient = useQueryClient();

  const query = useQuery<Workflow[], Error>({
    queryKey: ["workflows"],
    queryFn: fetchWorkflows,
  });

  const upsertMutation = useMutation({
    mutationFn: (input: UpsertWorkflowInput) => upsertWorkflow(input),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["workflows"] });
    },
  });

  const deactivateMutation = useMutation({
    mutationFn: (id: string) => deactivateWorkflow(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["workflows"] });
    },
  });

  return {
    ...query,
    upsertMutation,
    deactivateMutation,
  };
}
