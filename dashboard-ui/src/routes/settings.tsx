import { useEffect, useState } from "react";
import { SettingsIcon, ServerIcon, DatabaseIcon, GlobeIcon, BoxesIcon } from "lucide-react";
import { PageHeader } from "@/components/ui/page-header";
import { SettingsProvider } from "@/components/settings/SettingsContext";
import { GeneralTab } from "@/components/settings/GeneralTab";
import { CachingTab } from "@/components/settings/CachingTab";
import { NetworkingTab } from "@/components/settings/NetworkingTab";
import { ProvidersTab } from "@/components/settings/ProvidersTab";
import { InfrastructureTab } from "@/components/settings/InfrastructureTab";
import { EditionStatusChip } from "@/components/settings/EditionBadges";
import { cn } from "@/lib/utils";
import type { SettingsTab } from "@/components/settings/types";

const TABS: { id: SettingsTab; label: string; icon: React.ComponentType<{ className?: string }> }[] = [
  { id: "general", label: "General", icon: SettingsIcon },
  { id: "providers", label: "Providers", icon: ServerIcon },
  { id: "infrastructure", label: "Infrastructure", icon: BoxesIcon },
  { id: "caching", label: "Caching", icon: DatabaseIcon },
  { id: "networking", label: "Networking", icon: GlobeIcon },
];

function SettingsPageInner(): JSX.Element {
  const [activeTab, setActiveTab] = useState<SettingsTab>("general");
  const visibleTabs = TABS;

  useEffect(() => {
    if (!visibleTabs.some((tab) => tab.id === activeTab)) {
      setActiveTab("general");
    }
  }, [activeTab, visibleTabs]);

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between">
        <PageHeader title="Settings" subtitle="Configure gateway runtime behavior including providers, caching, security, and networking." />
        <EditionStatusChip />
      </div>

      <div className="flex flex-wrap gap-1 border-b border-border/60 pb-0">
        {visibleTabs.map((tab) => {
          const Icon = tab.icon;
          return (
            <button
              key={tab.id}
              onClick={() => setActiveTab(tab.id)}
              className={cn(
                "flex items-center gap-2 px-4 py-2.5 text-[13px] font-medium rounded-t-lg border border-b-0 transition-colors",
                activeTab === tab.id
                  ? "border-border/60 bg-surface text-foreground"
                  : "border-transparent text-muted-foreground hover:text-foreground hover:bg-surface-hover/30"
              )}
            >
              <Icon className="h-4 w-4" />
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
