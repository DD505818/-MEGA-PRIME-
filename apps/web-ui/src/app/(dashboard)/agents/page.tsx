'use client';

import { PageHeader } from '@/components/layout/PageHeader';
import { SurfacePanel } from '@/components/common/SurfacePanel';
import { useAgents } from '@/hooks/useAgents';

export default function AgentsPage() {
  const { data, isError, isLoading } = useAgents();
  return (
    <section className="space-y-4">
      <PageHeader title="Agents" subtitle="Production operator surface" />
      <SurfacePanel title="Agents" data={data} isLoading={isLoading} isError={isError} />
    </section>
  );
}
