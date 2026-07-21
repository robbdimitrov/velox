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
        'style-src': ['self'],
        // Seat maps and health gauges compute live inline style attributes;
        // keep the exception scoped to attributes, not scripts or stylesheets.
        'style-src-attr': ['unsafe-inline'],
        'img-src': ['self'],
        'connect-src': ['self'],
        'font-src': ['self'],
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
