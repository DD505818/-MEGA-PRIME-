import { cn } from '@/lib/utils';

const variants = {
  success: 'bg-mint/20 text-mint',
  danger: 'bg-coral/20 text-coral',
  warning: 'bg-amber/20 text-amber',
  info: 'bg-cyan/20 text-cyan',
  neutral: 'bg-slate text-lavender'
};

export function Badge({ variant, children }: { variant: keyof typeof variants; children: React.ReactNode }) {
  return <span className={cn('inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium', variants[variant])}>{children}</span>;
}
