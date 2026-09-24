import { useEffect, useState } from "react";
import {
  SettingsIcon,
  ServerIcon,
  DatabaseIcon,
  GlobeIcon,
  BoxesIcon,
  KeyIcon,
  CpuIcon,
  PuzzleIcon,
} from "lucide-react";
import { PageHeader } from "@/components/ui/page-header";
import { SettingsProvider } from "@/components/settings/SettingsContext";
import { GeneralTab } from "@/components/settings/GeneralTab";
import { CachingTab } from "@/components/settings/CachingTab";
import { NetworkingTab } from "@/components/settings/NetworkingTab";
import { ProvidersTab } from "@/components/settings/ProvidersTab";
import { InfrastructureTab } from "@/components/settings/InfrastructureTab";
import { SessionHubTab } from "@/components/settings/SessionHubTab";
import { SidecarTab } from "@/components/settings/SidecarTab";
import { ExtensionsTab } from "@/components/settings/ExtensionsTab";
import { EditionStatusChip } from "@/components/settings/EditionBadges";
import { ExtensionBlocks, ExtensionWidgets } from "@/components/extensions/ExtensionWidgets";
import {
  useExtensionSettingsTabs,
  useExtensionUIContext,
} from "@/lib/extensions/ui-context";
import { cn } from "@/lib/utils";
import { useDashboardConfig } from "@/lib/api/useDashboardConfig";
import { extractHiddenFeatures, hiddenFeatureSet } from "@/lib/features/features";
import type { SettingsTab } from "@/components/settings/types";

const BUILTIN_TABS: {
  id: SettingsTab;
  label: string;
  icon: React.ComponentType<{ className?: string }>;
  featureId?: string;
}[] = [
  { id: "general", label: "General", icon: SettingsIcon },
  { id: "providers", label: "Providers", icon: ServerIcon, featureId: "providers" },
  { id: "infrastructure", label: "Infrastructure", icon: BoxesIcon, featureId: "infrastructure" },
  { id: "caching", label: "Caching", icon: DatabaseIcon, featureId: "caching" },
  { id: "networking", label: "Networking", icon: GlobeIcon, featureId: "networking" },
  { id: "sessionhub", label: "Session Hub", icon: KeyIcon, featureId: "sessionhub" },
  { id: "extensions", label: "Extensions", icon: PuzzleIcon, featureId: "extensions" },
  { id: "sidecar", label: "Sidecar", icon: CpuIcon, featureId: "sidecar" },
];

type TabItem = {
  id: string;
  label: string;
  icon: React.ComponentType<{ className?: string }>;
  extensionId?: string;
};

function SettingsPageInner(): JSX.Element {
  const [activeTab, setActiveTab] = useState<string>("general");
  const { hideSettingsTabs } = useExtensionUIContext();
  const extTabs = useExtensionSettingsTabs();
  const { data: config } = useDashboardConfig();
  const hiddenFeatures = hiddenFeatureSet(extractHiddenFeatures(config));

  const visibleTabs: TabItem[] = [
    ...BUILTIN_TABS.filter(
      (t) =>
        !hideSettingsTabs.has(t.id) &&
        !(t.featureId && hiddenFeatures.has(t.featureId)),
    ).map((t) => ({
      id: t.id as string,
      label: t.label,
      icon: t.icon,
    })),
    ...extTabs.map((t) => ({
      id: t.id,
      label: t.label,
      icon: PuzzleIcon,
      extensionId: t.extensionId,
    })),
  ];

  useEffect(() => {
    const tabParam = window.location.hash.replace(/^#/, "").trim();
    if (tabParam && visibleTabs.some((tab) => tab.id === tabParam)) {
      setActiveTab(tabParam);
      window.history.replaceState(null, "", window.location.pathname + window.location.search);
    }
  }, [visibleTabs]);

  useEffect(() => {
    const onTab = (event: Event) => {
      const detail = (event as CustomEvent<unknown>).detail;
      if (typeof detail === "string" && detail) setActiveTab(detail);
    };
    window.addEventListener("aurora:settings-tab", onTab);
    return () => window.removeEventListener("aurora:settings-tab", onTab);
  }, []);

  const selectTab = (id: string) => {
    setActiveTab(id);
  };

  useEffect(() => {
    if (!visibleTabs.some((tab) => tab.id === activeTab)) {
      setActiveTab("general");
    }
  }, [activeTab, visibleTabs]);

  const activeExtTab = extTabs.find((t) => t.id === activeTab);

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between">
        <PageHeader title="Settings" subtitle="Configure gateway runtime behavior including providers, caching, security, and networking." />
        <EditionStatusChip />
      </div>

      <div className="flex overflow-x-auto gap-1 border-b border-border/60 pb-0 -mx-4 px-4 sm:mx-0 sm:px-0">
        {visibleTabs.map((tab) => {
          const Icon = tab.icon;
          return (
            <button
              key={tab.id}
              onClick={() => selectTab(tab.id)}
              className={cn(
                "flex items-center gap-2 whitespace-nowrap px-4 py-2.5 text-[13px] font-medium rounded-t-lg border border-b-0 transition-colors",
                activeTab === tab.id
                  ? "border-border/60 bg-surface text-foreground"
                  : "border-transparent text-muted-foreground hover:text-foreground hover:bg-surface-hover/30"
              )}
            >
              <Icon className="h-4 w-4 shrink-0" />
              {tab.label}
            </button>
          );
        })}
      </div>

      {activeTab === "general" && <GeneralTab />}
      {activeTab === "caching" && <CachingTab />}
      {activeTab === "networking" && <NetworkingTab />}
      {activeTab === "providers" && <ProvidersTab />}
      {activeTab === "infrastructure" && <InfrastructureTab />}
      {activeTab === "sessionhub" && <SessionHubTab />}
      {activeTab === "extensions" && <ExtensionsTab />}
      {activeTab === "sidecar" && <SidecarTab />}

      {activeExtTab && (
        <div className="border border-border/40 bg-surface/35 p-6">
          <ExtensionBlocks blocks={activeExtTab.blocks ?? []} />
        </div>
      )}

      <ExtensionWidgets slot="settings" />
    </div>
  );
}

export function SettingsPage(): JSX.Element {
  return (
    <SettingsProvider>
      <SettingsPageInner />
    </SettingsProvider>
  );
}
