import { DatabaseIcon, NetworkIcon } from "lucide-react";
import { SiMongodb, SiPostgresql, SiSqlite } from "react-icons/si";
import type { IconType } from "react-icons";
import { Surface, SectionHeader } from "@/components/ui/surface";
import { useSettings, StatusChip } from "./SettingsContext";

type ServiceCard = {
  id: string;
  name: string;
  description: string;
  enabled: boolean;
  icon?: IconType;
  color: string;
  glyph?: string;
  details: Array<{ label: string; value: string }>;
};

function valueOrDefault(value: unknown, fallback = "Not configured"): string {
  if (typeof value === "string" && value.trim()) return value.trim();
  if (typeof value === "number" && value > 0) return String(value);
  return fallback;
}

function BrandMark({ service }: { service: ServiceCard }): JSX.Element {
  const Icon = service.icon;
  return (
    <div className="flex h-11 w-11 shrink-0 items-center justify-center border" style={{ borderColor: `${service.color}55`, backgroundColor: `${service.color}18` }}>
      {Icon ? <Icon className="h-5 w-5" style={{ color: service.color }} /> : <span className="text-[18px] font-black" style={{ color: service.color }}>{service.glyph}</span>}
    </div>
  );
}

function ConfigCard({ service }: { service: ServiceCard }): JSX.Element {
  return (
    <div className="border border-border/40 bg-surface p-4 transition-colors hover:bg-surface-hover/30">
      <div className="flex items-start gap-3">
        <BrandMark service={service} />
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2">
            <h3 className="text-[14px] font-semibold text-foreground">{service.name}</h3>
            <StatusChip enabled={service.enabled} />
          </div>
          <p className="mt-1 text-[12px] leading-relaxed text-muted-foreground">{service.description}</p>
        </div>
      </div>
      <div className="mt-4 grid gap-2">
        {service.details.map(detail => (
          <div key={detail.label} className="flex items-center justify-between gap-3 text-[12px]">
            <span className="text-muted-foreground">{detail.label}</span>
            <code className="max-w-[62%] truncate border border-border/40 bg-background/60 px-2 py-1 text-[11px] text-foreground" title={detail.value}>{detail.value}</code>
          </div>
        ))}
      </div>
    </div>
  );
}

export function InfrastructureTab(): JSX.Element {
  const { runtimeSettings } = useSettings();
  const storage = runtimeSettings?.storage ?? {};
  const storageType = valueOrDefault(storage.type, "sqlite").toLowerCase();

  const services: ServiceCard[] = [
    {
      id: "sqlite",
      name: "SQLite",
      description: "Default local storage backend for audit logs, usage records, batches, aliases, and admin-managed state.",
      enabled: storageType === "sqlite",
      icon: SiSqlite,
      color: "#003B57",
      details: [
        { label: "Backend", value: storageType === "sqlite" ? "Active" : "Available" },
        { label: "Path", value: valueOrDefault(storage.sqlite_path, "data/aurora.db") },
      ],
    },
    {
      id: "postgresql",
      name: "PostgreSQL",
      description: "Relational storage option for durable usage, audit, budget, key, and workflow data.",
      enabled: storageType === "postgresql",
      icon: SiPostgresql,
      color: "#4169E1",
      details: [
        { label: "Backend", value: storageType === "postgresql" ? "Active" : "Available" },
        { label: "URL", value: valueOrDefault(storage.postgresql_url) },
        { label: "Max conns", value: valueOrDefault(storage.postgresql_max_conns, "10") },
      ],
    },
    {
      id: "mongodb",
      name: "MongoDB",
      description: "Document storage option for deployments that prefer MongoDB-backed operational records.",
      enabled: storageType === "mongodb",
      icon: SiMongodb,
      color: "#47A248",
      details: [
        { label: "Backend", value: storageType === "mongodb" ? "Active" : "Available" },
        { label: "URL", value: valueOrDefault(storage.mongodb_url) },
        { label: "Database", value: valueOrDefault(storage.mongodb_database, "aurora") },
      ],
    },
  ];

  return (
    <div className="flex flex-col gap-6">
      <Surface id="runtime-infrastructure" className="p-6 scroll-mt-20">
        <div className="flex flex-col gap-6">
          <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
            <div className="flex items-start gap-3">
              <div className="border border-border/40 bg-background/80 p-2">
                <DatabaseIcon className="h-4 w-4 text-accent" />
              </div>
              <SectionHeader title="Storage Infrastructure" subtitle="Durable storage backends configured for audit logs, usage records, batches, aliases, and admin-managed state." />
            </div>
            <div className="flex items-center gap-2 text-[12px] text-muted-foreground">
              <NetworkIcon className="h-4 w-4" />
              Secrets are redacted server-side
            </div>
          </div>
          <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
            {services.map(service => <ConfigCard key={service.id} service={service} />)}
          </div>
        </div>
      </Surface>
    </div>
  );
}
