import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  test: {
    environment: 'happy-dom',
    setupFiles: ['./src/test/setup.ts'],
    globals: true,
    exclude: ['**/node_modules/**', '**/e2e/**'],
    server: {
      deps: {
        // zod's package exports list a raw-TS "@zod/source" condition first;
        // under the bun runtime an externalized import resolves to that source,
        // where the `z` export comes back undefined. Inlining makes vitest
        // transform zod through vite (which resolves the compiled entry),
        // matching the production build.
        inline: ['zod'],
      },
    },
    coverage: {
      // istanbul (source instrumentation), not v8: the v8 provider relies on
      // the V8 inspector coverage API (node:inspector), which the bun runtime
      // does not implement, so `bun --bun vitest --coverage` fails with
      // "Coverage APIs are not supported". istanbul works under both runtimes.
      provider: 'istanbul',
      reporter: ['text', 'lcov'],
      reportsDirectory: './coverage',
      thresholds: {
        lines: 75,
        functions: 80,
      },
    },
  },
})
