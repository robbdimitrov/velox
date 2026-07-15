import adapter from '@sveltejs/adapter-node';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';

/** @type {import('@sveltejs/kit').Config} */
const config = {
  preprocess: vitePreprocess(),
  kit: {
    adapter: adapter(),
    csp: {
      mode: 'nonce',
      directives: {
        'default-src': ['self'],
        'script-src': ['self'],
        'style-src': ['self', 'https://fonts.googleapis.com'],
        // Seat map (SeatCanvas.svelte) and health-panel gauges compute
        // per-render pixel sizes and cursor state as inline style attributes;
        // that content can't be hashed or known ahead of time, so this
        // trades attribute-level styling only, not script-src or style-src.
        'style-src-attr': ['unsafe-inline'],
        'img-src': ['self'],
        'connect-src': ['self'],
        'font-src': ['self', 'https://fonts.gstatic.com'],
        'object-src': ['none'],
        'base-uri': ['self'],
        'form-action': ['self'],
        'frame-ancestors': ['none'],
        'frame-src': ['none']
      }
    }
  }
};

export default config;
