import vue from '@vitejs/plugin-vue'
import { defineConfig } from 'vite'

// The Wails asset server serves the built files from the webview root, so
// relative asset URLs are used to stay compatible with both the packaged app
// and a plain `vite preview`.
export default defineConfig({
  base: './',
  plugins: [vue()],
  server: {
    host: '0.0.0.0',
    port: 3004,
    strictPort: true,
  },
  build: {
    target: 'es2022',
    outDir: 'dist',
    emptyOutDir: true,
    chunkSizeWarningLimit: 900,
  },
})
