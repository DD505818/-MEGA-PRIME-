'use client';

import { PageHeader } from '@/components/layout/PageHeader';
import { RiskLimitsDisplay } from '@/components/risk/RiskLimitsDisplay';

export default function RiskPage() {
  return (
    <section className="space-y-4">
      <PageHeader title="Risk" subtitle="Production operator surface" />
      <RiskLimitsDisplay />
    </section>
  );
}
