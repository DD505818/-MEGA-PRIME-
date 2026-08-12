import { type ButtonHTMLAttributes } from 'react';
import { cn } from '@/lib/utils/cn';

type Variant = 'primary' | 'destructive' | 'outline' | 'ghost';

const variants: Record<Variant, string> = {
  primary: 'bg-foreground text-background hover:bg-foreground/80',
  destructive: 'bg-destructive text-white hover:bg-destructive/80',
  outline: 'border border-muted text-foreground hover:bg-card',
  ghost: 'text-foreground hover:bg-card'
};

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant;
  size?: 'sm' | 'md' | 'lg';
}

export function Button({
  variant = 'primary',
  size = 'md',
  className,
  children,
  ...props
}: ButtonProps) {
  return (
    <button
      className={cn(
        'inline-flex items-center justify-center rounded-full font-medium transition-all duration-150 active:scale-[0.98] disabled:opacity-50',
        variants[variant],
        { sm: 'h-8 px-3 text-xs', md: 'h-10 px-4 text-sm', lg: 'h-12 px-6 text-base' }[size],
        className
      )}
      {...props}
    >
      {children}
    </button>
  );
}
