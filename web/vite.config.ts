import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

const backendUrl = process.env.KVELMO_BACKEND_URL || 'http://localhost:6337'

export default defineConfig({
  base: './',
  plugins: [react(), tailwindcss()],
  build: {
    rolldownOptions: {
      output: {
        manualChunks(id) {
          if (id.includes('node_modules')) {
            if (id.includes('react-dom') || id.includes('/react/') || id.includes('zustand')) {
              return 'vendor'
            }
            if (id.includes('react-diff-viewer-continued')) {
              return 'diff'
            }
          }
        }
      }
    }
  },
  server: {
    port: 5173,
    strictPort: false, // Allow fallback to the next available port
    // Chokidar opens an fd per watched file. Without aggressive ignoring the
    // repo blows past the default ulimit -n and the dev server dies with
    // EMFILE during startup. Set VITE_WATCH_POLLING=1 to bypass fs.watch
    // entirely (slower, used by make web-e2e on systems with tight fd caps).
    watch: {
      usePolling: process.env.VITE_WATCH_POLLING === '1',
      ignored: [
        '**/node_modules/**',
        '**/dist/**',
        '**/.git/**',
        '**/build/**',
        '**/target/**',
        '**/prototype/**',
        '**/src-tauri/target/**',
        '**/src-tauri/gen/**',
        '**/coverage/**',
        '**/playwright-report/**',
        '**/test-results/**',
        '**/.playwright-cli/**',
        '**/.playwright-mcp/**',
      ],
    },
    proxy: {
      '/api': {
        target: backendUrl,
        changeOrigin: true
      },
      '/ws': {
        target: backendUrl,
        changeOrigin: true,
        ws: true
      }
    }
  }
})
