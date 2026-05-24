import { defineConfig, loadEnv } from 'vite'
import vue from '@vitejs/plugin-vue'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = dirname(fileURLToPath(import.meta.url))

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, __dirname, '')
  // dev 代理目标（API 实际监听地址）；与 VITE_API_BASE_URL 分离，后者仅用于 OAuth 回调 URL
  const proxyTarget = (env.VITE_DEV_API_PROXY || 'http://localhost:8080').replace(/\/$/, '')

  if (mode === 'production' && !env.VITE_API_BASE_URL?.trim()) {
    console.warn(
      '[vite] VITE_API_BASE_URL is not set. Same-origin (All-in-One) builds are OK; ' +
        'split deployments (Firebase + API) require it for OAuth redirects and API calls.'
    )
  }

  return {
    plugins: [vue()],
    build: {
      chunkSizeWarningLimit: 1200,
      rollupOptions: {
        output: {
          manualChunks(id) {
            if (id.includes('node_modules/element-plus')) return 'element-plus'
            if (id.includes('node_modules/@codemirror')) return 'codemirror'
            if (id.includes('node_modules/vue') || id.includes('node_modules/vue-router')) return 'vue-vendor'
          },
        },
      },
    },
    server: {
      port: 5173,
      proxy: {
        '/api': {
          target: proxyTarget,
          changeOrigin: true
        }
      }
    }
  }
})
