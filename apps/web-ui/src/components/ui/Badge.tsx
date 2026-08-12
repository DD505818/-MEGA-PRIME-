import type { ReactNode } from 'react';
import { cn } from '@/lib/utils/cn';

const variants = {
  success: 'bg-success/20 text-success',
  danger: 'bg-destructive/20 text-destructive',
  warning: 'bg-warning/20 text-warning',
  info: 'bg-foreground/10 text-foreground',
  neutral: 'bg-muted text-foreground'
};

export function Badge({
  variant,
  children
}: {
  variant: keyof typeof variants;
  children: ReactNode;
}) {
  return (
    <span
      className={cn(
        'inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium',
        variants[variant]
      )}
    >
      {children}
    </span>
  );
}
