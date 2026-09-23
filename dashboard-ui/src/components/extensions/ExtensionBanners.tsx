import {
  AlertTriangle,
  CheckCircle2,
  Info,
  XCircle,
  type LucideIcon,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { useExtensionBanners } from "@/lib/extensions/ui-context";

const LEVEL_STYLES: Record<string, string> = {
  info: "border-info/40 bg-info/10 text-info",
  success: "border-success/40 bg-success/10 text-success",
  warning: "border-warning/40 bg-warning/10 text-warning",
  error: "border-destructive/40 bg-destructive/10 text-destructive",
};

const LEVEL_ICONS: Record<string, LucideIcon> = {
  info: Info,
  success: CheckCircle2,
  warning: AlertTriangle,
  error: XCircle,
};

function safeExternal(href: string): boolean {
  try {
    const u = new URL(href, window.location.origin);
    return u.protocol === "http:" || u.protocol === "https:";
  } catch {
    return false;
  }
}

export function ExtensionBanners(): JSX.Element | null {
  const banners = useExtensionBanners();
  if (banners.length === 0) return null;

  return (
    <div className="flex flex-col gap-2 mb-4" role="status">
      {banners.map((b) => {
        const level = b.level || "info";
        const Icon = LEVEL_ICONS[level] ?? Info;
        const style = LEVEL_STYLES[level] ?? LEVEL_STYLES.info!;
        const showLink = b.link_url && b.link_label && safeExternal(b.link_url);
        return (
          <div
            key={`${b.extensionId}:${b.id}`}
            className={cn("flex items-start gap-3 border px-4 py-3 text-sm", style)}
          >
            <Icon className="mt-0.5 h-4 w-4 shrink-0" aria-hidden />
            <div className="min-w-0 flex-1">
              <span className="text-foreground">{b.message}</span>
              {showLink && (
                <a
                  href={b.link_url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="ml-2 font-medium underline underline-offset-2 hover:opacity-80"
                >
                  {b.link_label}
                </a>
              )}
            </div>
          </div>
        );
      })}
    </div>
  );
}
