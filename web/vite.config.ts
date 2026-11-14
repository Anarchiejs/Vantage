/// <reference types="vitest/config" />
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

// In development, /api is proxied to the Go server (`vantage serve`).
// `npm run dev:mock` serves the files in public/mocks instead (see src/api/client.ts).
export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    host: '127.0.0.1',
    proxy: { '/api': 'http://127.0.0.1:7878' },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    // The server's CSP allows fonts from 'self' only: keep font files as files, never data: URIs.
    assetsInlineLimit: (file: string) => (/\.(woff2?|ttf|otf)$/.test(file) ? false : undefined),
    rolldownOptions: {
      output: {
        // Keep echarts and CodeMirror out of the route chunks. The chart
        // entry imports only the series this UI draws, so neither vendor
        // chunk should cross Vite's 500 kB warning.
        codeSplitting: {
          groups: [
            { name: 'zrender', test: /[\\/]node_modules[\\/]zrender[\\/]/, priority: 40 },
            { name: 'echarts-charts', test: /[\\/]node_modules[\\/]echarts[\\/]lib[\\/]chart[\\/]/, priority: 30 },
            { name: 'echarts', test: /[\\/]node_modules[\\/]echarts[\\/]/, priority: 20 },
            { name: 'codemirror', test: /[\\/]node_modules[\\/](@codemirror|codemirror)[\\/]/, priority: 20 },
          ],
        },
      },
    },
  },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test/setup.ts'],
    include: ['src/**/*.test.{ts,tsx}'],
  },
})
