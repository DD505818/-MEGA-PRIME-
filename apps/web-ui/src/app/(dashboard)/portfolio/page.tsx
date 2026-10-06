'use client';

import { PageHeader } from '@/components/layout/PageHeader';
import { SurfacePanel } from '@/components/common/SurfacePanel';
import { usePortfolio } from '@/hooks/usePortfolio';

export default function PortfolioPage() {
  const { data, isError, isLoading } = usePortfolio();
  return (
    <section className="space-y-4">
      <PageHeader title="Portfolio" subtitle="Production operator surface" />
      <SurfacePanel title="Portfolio" data={data} isLoading={isLoading} isError={isError} />
    </section>
  );
}
