import { useEffect, useState } from "react";
import { getApiKey, isApiKeyVerified, subscribe } from "./storage";

export interface ApiKeyState {
  key: string;
  verified: boolean;
}

/** Subscribes the component to API-key changes (cross-tab + same-tab). */
export function useApiKey(): string {
  return useApiKeyState().key;
}

export function useApiKeyState(): ApiKeyState {
  const [state, setState] = useState<ApiKeyState>(() => ({
    key: getApiKey(),
    verified: isApiKeyVerified(),
  }));
  useEffect(
    () =>
      subscribe(() =>
        setState({
          key: getApiKey(),
          verified: isApiKeyVerified(),
        }),
      ),
    [],
  );
  return state;
}
