import type { ReactNode } from 'react';
import { cn } from '@/lib/utils/cn';

export function Card({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <div
      className={cn(
        'rounded-2xl border border-muted/50 bg-card/70 p-5 shadow-lg backdrop-blur-3xl',
        className
      )}
    >
      {children}
    </div>
  );
}
