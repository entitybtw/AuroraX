import * as React from "react";
import type { AuditEntry } from "@/lib/api/audit-types";
import { fetchAuditLog } from "@/lib/api/audit";

const MAX_LIVE_ENTRIES = 500;
const POLL_INTERVAL_MS = 3000;

interface LiveLogsOptions {
  enabled?: boolean;
  tenant?: string;
  onEntry?: (entry: AuditEntry) => void;
}

export function useLiveLogs({ enabled = true, tenant, onEntry }: LiveLogsOptions = {}) {
  const [liveEntries, setLiveEntries] = React.useState<AuditEntry[]>([]);
  const [isConnected, setIsConnected] = React.useState(false);
  const seenRef = React.useRef(new Set<string>());
  const eventSourceRef = React.useRef<EventSource | null>(null);
  const pollTimerRef = React.useRef<ReturnType<typeof setInterval> | null>(null);

  const clear = React.useCallback(() => {
    setLiveEntries([]);
    seenRef.current.clear();
  }, []);

  React.useEffect(() => {
    if (!enabled) {
      clear();
      setIsConnected(false);
      return;
    }

    let mounted = true;
    let retryCount = 0;
    const maxRetries = 5;
    const basePath = (window as { __BASE_PATH__?: string }).__BASE_PATH__ ?? "";
    let retryTimeout: ReturnType<typeof setTimeout> | null = null;

    function addEntry(entry: AuditEntry) {
      if (!mounted) return;
      if (!entry.id || seenRef.current.has(entry.id)) return;
      seenRef.current.add(entry.id);
      setLiveEntries((prev) => {
        const next = [entry, ...prev];
        return next.length > MAX_LIVE_ENTRIES ? next.slice(0, MAX_LIVE_ENTRIES) : next;
      });
      onEntry?.(entry);
    }

    function trySSE() {
      try {
        const qs = tenant ? `?tenant=${encodeURIComponent(tenant)}` : "";
        const url = `${basePath}/admin/api/v1/audit/log/stream${qs}`;
        const es = new EventSource(url);
        eventSourceRef.current = es;

        es.onopen = () => {
          if (mounted) {
            setIsConnected(true);
            retryCount = 0;
          }
        };

        es.onmessage = (event) => {
          try {
            const data = JSON.parse(event.data);
            addEntry(data);
          } catch {
            // ignore parse errors
          }
        };

        es.onerror = () => {
          if (mounted) setIsConnected(false);
          es.close();
          eventSourceRef.current = null;
          if (retryCount < maxRetries) {
            retryCount++;
            const delay = Math.min(1000 * Math.pow(2, retryCount), 15000);
            retryTimeout = setTimeout(() => { if (mounted) trySSE(); }, delay);
          } else {
            startPolling();
          }
        };
      } catch {
        startPolling();
      }
    }

    async function pollOnce() {
      if (!mounted) return;
      try {
        const page = await fetchAuditLog({
          limit: 25,
          sort: "-timestamp",
          ...(tenant ? { tenant } : {}),
        });
        for (const entry of page.entries) {
          addEntry(entry);
        }
      } catch {
        // ignore poll errors
      }
    }

    function startPolling() {
      if (!mounted) return;
      setIsConnected(false);
      pollOnce();
      pollTimerRef.current = setInterval(pollOnce, POLL_INTERVAL_MS);
    }

    trySSE();

    return () => {
      mounted = false;
      eventSourceRef.current?.close();
      eventSourceRef.current = null;
      if (retryTimeout) {
        clearTimeout(retryTimeout);
        retryTimeout = null;
      }
      if (pollTimerRef.current) {
        clearInterval(pollTimerRef.current);
        pollTimerRef.current = null;
      }
    };
  }, [enabled, onEntry, clear, tenant]);

  return { liveEntries, isConnected, clear };
}
