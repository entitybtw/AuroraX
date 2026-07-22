import * as React from "react";

interface TenantScopeContextValue {
  tenant: string;
  setTenant: (tenant: string) => void;
  tenantOptions: Array<{ id: string; slug: string; name: string }>;
}

const TenantScopeContext = React.createContext<TenantScopeContextValue | null>(null);

export function TenantScopeProvider({ children }: { children: React.ReactNode }): JSX.Element {
  const value = React.useMemo(
    () => ({ tenant: "", setTenant: () => {}, tenantOptions: [{ id: "", slug: "default", name: "Default" }] }),
    [],
  );

  return <TenantScopeContext.Provider value={value}>{children}</TenantScopeContext.Provider>;
}

export function useTenantScope(): TenantScopeContextValue {
  const value = React.useContext(TenantScopeContext);
  if (!value) {
    throw new Error("useTenantScope must be used inside TenantScopeProvider");
  }
  return value;
}
