import {defineConfig} from 'vite'
import {svelte} from '@sveltejs/vite-plugin-svelte'
import path from 'path'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [svelte()],
  resolve: {
    // Vitest must resolve Svelte's browser (client) build so component tests
    // can call mount(); with the default conditions it picks the server build
    // and every mount() throws lifecycle_function_unavailable. Guarded by the
    // VITEST flag so the app build keeps Vite's normal resolution.
    ...(process.env.VITEST ? { conditions: ['browser'] } : {}),
    alias: {
      $components: path.resolve('./src/components'),
      $pages: path.resolve('./src/pages'),
      $lib: path.resolve('./src/lib'),
      '@wails': path.resolve('./wailsjs'),
    },
  },
  test: {
    globals: true,
    environment: 'jsdom',
    // Scope unit tests to src/ so the Playwright e2e specs in e2e/ (which use
    // @playwright/test's test runner, not vitest) aren't picked up here. They
    // run under `npm run e2e` instead.
    include: ['src/**/*.{test,spec}.ts'],
  },
})
