import { defineConfig } from 'vite';

export default defineConfig({
  root: '.',
  server: { host: '127.0.0.1', port: 4175, strictPort: true,
    proxy: { '/api': { target: 'http://127.0.0.1:4176', changeOrigin: false } } },
  preview: { host: '127.0.0.1', port: 4177 },
  build: { target: 'es2022', outDir: 'dist', emptyOutDir: true },
});
