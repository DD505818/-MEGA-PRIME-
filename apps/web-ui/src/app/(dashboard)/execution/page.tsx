'use client';

import { PageHeader } from '@/components/layout/PageHeader';
import { SurfacePanel } from '@/components/common/SurfacePanel';
import { useExecution } from '@/hooks/useExecution';

export default function ExecutionPage() {
  const { data, isError, isLoading } = useExecution();
  return (
    <section className="space-y-4">
      <PageHeader title="Execution" subtitle="Production operator surface" />
      <SurfacePanel title="Execution" data={data} isLoading={isLoading} isError={isError} />
    </section>
  );
}
