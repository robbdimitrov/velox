import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';

export default defineConfig({
  plugins: [sveltekit()],
  server: {
    port: 5173,
    strictPort: false
  },
  // Vitest otherwise resolves Svelte's server-side build for component
  // render()s, since Vite defaults to non-browser conditions outside a real
  // browser/dev-server context. Scoped to test runs only (process.env.VITEST)
  // so dev/build resolution is unaffected. Component test files opt into the
  // jsdom environment individually via a `// @vitest-environment jsdom`
  // directive, so plain-logic tests keep running in the lighter default
  // (node) environment.
  resolve: process.env.VITEST ? { conditions: ['browser'] } : undefined
});
