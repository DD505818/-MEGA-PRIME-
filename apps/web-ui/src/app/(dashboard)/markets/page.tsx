'use client';

import { PageHeader } from '@/components/layout/PageHeader';
import { SurfacePanel } from '@/components/common/SurfacePanel';
import { useMarketData } from '@/hooks/useMarketData';

export default function MarketsPage() {
  const { data, isError, isLoading } = useMarketData();
  return (
    <section className="space-y-4">
      <PageHeader title="Markets" subtitle="Production operator surface" />
      <SurfacePanel title="Markets" data={data} isLoading={isLoading} isError={isError} />
    </section>
  );
}
