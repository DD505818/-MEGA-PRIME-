/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      colors: {
        bg:     '#050a14',
        panel:  '#0f1828',
        panel2: '#121e33',
        border: '#263753',
        text:   '#e8f2ff',
        muted:  '#9fb0cc',
        green:  '#4df2b2',
        red:    '#ff617d',
        blue:   '#6dbbff',
        gold:   '#ffc27a',
        purple: '#a78bfa',
      },
      fontFamily: {
        mono: ['JetBrains Mono', 'Fira Code', 'monospace'],
        sans: ['Inter', 'system-ui', 'sans-serif'],
      },
      animation: {
        pulse2: 'pulse2 1.6s ease-in-out infinite',
        flash:  'flash 1.2s alternate infinite',
        slideIn:'slideIn 0.4s ease',
      },
      keyframes: {
        pulse2:  { '0%,100%': { opacity: '0.35' }, '50%': { opacity: '1' } },
        flash:   { '0%': { boxShadow: '0 0 0 rgba(255,97,125,.15)' }, '100%': { boxShadow: '0 0 18px rgba(255,97,125,.7)' } },
        slideIn: { from: { transform: 'translateY(14px)', opacity: '0' }, to: { transform: 'translateY(0)', opacity: '1' } },
      },
    },
  },
  plugins: [],
}
