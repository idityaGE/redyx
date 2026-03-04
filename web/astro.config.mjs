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
      hmr: {
        // Astro SSR runs its own server (4321) that proxies to Vite (5173).
        // Without this, Vite's HMR WebSocket fails because the browser
        // connects to 4321 but Vite listens on 5173.
        clientPort: 4321,
      },
      proxy: {
        '/api/v1/ws': {
          target: 'http://localhost:8080',
          changeOrigin: true,
          ws: true,
        },
        '/api/v1': {
          target: 'http://localhost:8080',
          changeOrigin: true,
        },
      },
    },
  },
});
