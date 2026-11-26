/// <reference types="vitest/config" />
import { defineConfig, loadEnv } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'
import { fileURLToPath, URL } from 'node:url'

// In development the Vite server proxies API and health requests to a local
// API so the app can use its same-origin default without CORS. Point
// HEALTHHUB_API_PROXY at another API to develop against it.
export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '')
  const target = env.HEALTHHUB_API_PROXY || 'http://localhost:8080'
  return {
    plugins: [vue(), tailwindcss()],
    resolve: {
      alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) },
    },
    server: {
      port: 5173,
      proxy: {
        '/api': { target, changeOrigin: true },
        '/health': { target, changeOrigin: true },
      },
    },
    build: {
      sourcemap: false,
    },
    test: {
      environment: 'jsdom',
      include: ['src/**/*.spec.ts'],
      restoreMocks: true,
    },
  }
})
