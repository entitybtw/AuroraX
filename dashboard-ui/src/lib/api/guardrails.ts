import { apiFetch } from "./client";
import {
  GuardrailListSchema,
  GuardrailSchema,
  GuardrailTypeDefListSchema,
  type Guardrail,
  type GuardrailTypeDef,
  type UpsertGuardrailInput,
} from "./guardrails-types";

export function fetchGuardrails(): Promise<Guardrail[]> {
  return apiFetch("/admin/api/v1/guardrails", {
    schema: GuardrailListSchema,
  }) as Promise<Guardrail[]>;
}

export function fetchGuardrailTypes(): Promise<GuardrailTypeDef[]> {
  return apiFetch("/admin/api/v1/guardrails/types", {
    schema: GuardrailTypeDefListSchema,
  }) as Promise<GuardrailTypeDef[]>;
}

export function upsertGuardrail(input: UpsertGuardrailInput): Promise<Guardrail> {
  return apiFetch(`/admin/api/v1/guardrails/${encodeURIComponent(input.name)}`, {
    method: "PUT",
    json: input,
    schema: GuardrailSchema,
  }) as Promise<Guardrail>;
}

export function deleteGuardrail(name: string): Promise<void> {
  return apiFetch(`/admin/api/v1/guardrails/${encodeURIComponent(name)}`, {
    method: "DELETE",
  }) as Promise<void>;
}
