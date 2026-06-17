import type { ReactNode } from 'react';
import { cn } from '@/lib/utils';

export function Card({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <div className={cn('rounded-2xl border border-slate/50 bg-aurora/70 p-5 shadow-lg backdrop-blur-3xl', className)}>
      {children}
    </div>
  );
}
