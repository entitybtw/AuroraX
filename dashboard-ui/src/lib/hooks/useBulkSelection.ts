import { useState, useCallback, useMemo } from "react";

export function useBulkSelection<T extends string | number>() {
  const [selected, setSelected] = useState<Set<T>>(new Set());

  const toggle = useCallback((id: T) => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }, []);

  const toggleAll = useCallback((ids: T[]) => {
    setSelected((prev) => {
      if (ids.every((id) => prev.has(id))) return new Set<T>();
      return new Set(ids);
    });
  }, []);

  const clear = useCallback(() => setSelected(new Set()), []);

  const isSelected = useCallback((id: T) => selected.has(id), [selected]);

  const allSelected = useCallback((ids: T[]) => ids.length > 0 && ids.every((id) => selected.has(id)), [selected]);
  const someSelected = useCallback((ids: T[]) => ids.some((id) => selected.has(id)) && !ids.every((id) => selected.has(id)), [selected]);

  const selectedCount = selected.size;

  return useMemo(() => ({
    selected,
    selectedCount,
    toggle,
    toggleAll,
    clear,
    isSelected,
    allSelected,
    someSelected,
    setSelected,
  }), [selected, selectedCount, toggle, toggleAll, clear, isSelected, allSelected, someSelected]);
}
