import { useState, useCallback, useRef, useMemo } from "react";

export function useBulkSelection<T extends string | number>() {
  const [selected, setSelected] = useState<Set<T>>(new Set());
  const lastClickedRef = useRef<T | null>(null);

  const toggle = useCallback((id: T) => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
    lastClickedRef.current = id;
  }, []);

  const rangeSelect = useCallback((id: T, allIds: T[]) => {
    const last = lastClickedRef.current;
    if (last === null || last === id) {
      setSelected((prev) => {
        const next = new Set(prev);
        if (next.has(id)) next.delete(id);
        else next.add(id);
        return next;
      });
      lastClickedRef.current = id;
      return;
    }
    const fromIdx = allIds.indexOf(last);
    const toIdx = allIds.indexOf(id);
    if (fromIdx === -1 || toIdx === -1) {
      setSelected((prev) => {
        const next = new Set(prev);
        if (next.has(id)) next.delete(id);
        else next.add(id);
        return next;
      });
      lastClickedRef.current = id;
      return;
    }
    const start = Math.min(fromIdx, toIdx);
    const end = Math.max(fromIdx, toIdx);
    const range = allIds.slice(start, end + 1);
    setSelected((prev) => {
      const next = new Set(prev);
      for (const rid of range) next.add(rid);
      return next;
    });
    lastClickedRef.current = id;
  }, []);

  const toggleAll = useCallback((ids: T[]) => {
    setSelected((prev) => {
      if (ids.every((id) => prev.has(id))) return new Set<T>();
      return new Set(ids);
    });
  }, []);

  const clear = useCallback(() => {
    setSelected(new Set());
    lastClickedRef.current = null;
  }, []);

  const isSelected = useCallback((id: T) => selected.has(id), [selected]);

  const allSelected = useCallback((ids: T[]) => ids.length > 0 && ids.every((id) => selected.has(id)), [selected]);
  const someSelected = useCallback((ids: T[]) => ids.some((id) => selected.has(id)) && !ids.every((id) => selected.has(id)), [selected]);

  const selectedCount = selected.size;

  return useMemo(() => ({
    selected,
    selectedCount,
    toggle,
    rangeSelect,
    toggleAll,
    clear,
    isSelected,
    allSelected,
    someSelected,
    setSelected,
  }), [selected, selectedCount, toggle, rangeSelect, toggleAll, clear, isSelected, allSelected, someSelected]);
}
