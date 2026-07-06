import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';

export default defineConfig({
  plugins: [sveltekit()],
  server: {
    port: 5173,
    strictPort: false
  },
  // Component tests need Svelte's browser build; keep this scoped to Vitest so
  // dev/build resolution and node-only tests keep their defaults.
  resolve: process.env.VITEST ? { conditions: ['browser'] } : undefined
});
