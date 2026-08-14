import {defineConfig} from 'vite'
import {svelte} from '@sveltejs/vite-plugin-svelte'
import {svelteIntl} from '@svelte-intl/svelte-intl/vite'
import path from 'path'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [svelteIntl({ localesDir: './src/translations', defaultLocale: 'en-US' }), svelte()],
  resolve: {
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
  },
})
