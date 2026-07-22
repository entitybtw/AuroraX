export function ForbiddenPage(): JSX.Element {
  return (
    <div className="flex min-h-[60vh] items-center justify-center">
      <div className="max-w-md border border-border/60 bg-surface px-6 py-5 text-center">
        <h1 className="font-serif text-2xl text-foreground">Access denied</h1>
        <p className="mt-2 text-sm text-muted-foreground">
          Your current session does not have permission to view this admin page.
        </p>
      </div>
    </div>
  );
}
