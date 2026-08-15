import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

function manualChunks(id: string) {
  if (!id.includes('node_modules')) return undefined
  if (id.includes('/react/') || id.includes('/react-dom/') || id.includes('/react-router') || id.includes('/scheduler/')) {
    return 'react-vendor'
  }
  if (id.includes('/lightweight-charts/') || id.includes('/recharts/')) {
    return 'charts'
  }
  if (id.includes('/three/')) {
    return 'three'
  }
  return undefined
}

export default defineConfig({
  plugins: [react()],
  server: {
    port: 3000,
    proxy: {
      '/api': {
        target: process.env.VITE_GATEWAY_URL || 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: 'dist',
    sourcemap: false,
    rollupOptions: {
      output: {
        manualChunks,
      },
    },
  },
})
