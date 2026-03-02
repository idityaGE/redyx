import { defineConfig } from 'astro/config';
import svelte from '@astrojs/svelte';
import node from '@astrojs/node';
import tailwindcss from '@tailwindcss/vite';

export default defineConfig({
  output: 'server',
  adapter: node({ mode: 'standalone' }),
  integrations: [svelte()],
  vite: {
    plugins: [tailwindcss()],
    server: {
      proxy: {
        '/api/v1': {
          target: 'http://localhost:8080',
          changeOrigin: true,
        },
      },
    },
  },
});
