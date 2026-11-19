import { defineConfig } from 'vite';
export default defineConfig({
  server: { host: '127.0.0.1', port: 4175, strictPort: true,
    proxy: { '/api/v1': { target: 'http://127.0.0.1:4176', changeOrigin: true } } },
  preview: { host: '127.0.0.1', port: 4175, strictPort: true },
  build: { outDir: 'dist', sourcemap: false },
});
