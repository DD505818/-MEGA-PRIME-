import type { Config } from 'tailwindcss';
import animate from 'tailwindcss-animate';
import typography from '@tailwindcss/typography';

const config: Config = {
  darkMode: ['class'],
  content: ['./src/**/*.{ts,tsx,mdx}'],
  theme: {
    extend: {
      colors: {
        background: 'hsl(222 47% 8%)',
        foreground: 'hsl(210 40% 96%)',
        card: 'hsl(222 47% 11%)',
        muted: 'hsl(215 15% 20%)',
        success: '#22c55e',
        warning: '#f59e0b',
        destructive: '#ef4444'
      }
    }
  },
  plugins: [animate, typography]
};

export default config;
