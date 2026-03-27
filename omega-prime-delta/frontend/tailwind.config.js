/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{js,ts,jsx,tsx}'],
  theme: {
    extend: {
      colors: {
        primary: '#0ea5e9',
        accent: '#22d3ee',
        surface: '#0f172a',
      },
      boxShadow: {
        glow: '0 0 20px rgba(34, 211, 238, 0.35)',
      },
    },
  },
  plugins: [],
}
