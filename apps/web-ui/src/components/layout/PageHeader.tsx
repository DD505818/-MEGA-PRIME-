export function PageHeader({ title, subtitle }: { title: string; subtitle: string }) {
  return (
    <header className="space-y-1">
      <h1 className="text-2xl font-semibold">{title}</h1>
      <p className="text-sm text-white/60">{subtitle}</p>
    </header>
  );
}
