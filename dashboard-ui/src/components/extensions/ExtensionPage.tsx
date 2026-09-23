import { useParams } from "@tanstack/react-router";
import { PageHeader } from "@/components/ui/page-header";
import { ExtensionBlocks } from "@/components/extensions/ExtensionWidgets";
import { useExtensionPages } from "@/lib/extensions/ui-context";

/**
 * Renders a dashboard page contributed by an applied extension.
 * Route: /admin/dashboard/ext/:pagePath (pagePath is pages[].path).
 * No HTML/JS from the extension is executed — only structured blocks.
 */
export function ExtensionPage(): JSX.Element {
  const params = useParams({ strict: false }) as { pagePath?: string };
  const pages = useExtensionPages();
  const pagePath = (params.pagePath ?? "").replace(/^\/+|\/+$/g, "");
  const page = pages.find((p) => p.path.replace(/^\/+|\/+$/g, "") === pagePath);

  if (!page) {
    return (
      <div className="flex flex-col gap-6">
        <PageHeader title="Extension page" subtitle="Not found" />
        <p className="text-sm text-muted-foreground">
          This extension page is not available. The extension may be removed or not applied.
        </p>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        title={page.title}
        kicker={page.extensionName}
        subtitle={page.summary ?? ""}
      />
      <div className="border border-border/40 bg-surface/35 p-6">
        <ExtensionBlocks blocks={page.blocks ?? []} />
      </div>
    </div>
  );
}
