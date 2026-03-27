import type { PropsWithChildren } from 'react';

type PanelProps = PropsWithChildren<{
  title: string;
  subtitle?: string;
}>;

export function Panel({ title, subtitle, children }: PanelProps) {
  return (
    <section className="panel">
      <header className="panel__header">
        <div>
          <h2>{title}</h2>
          {subtitle ? <p>{subtitle}</p> : null}
        </div>
      </header>
      {children}
    </section>
  );
}
