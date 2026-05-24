import { defineConfig, loadEnv } from 'vite'
import vue from '@vitejs/plugin-vue'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = dirname(fileURLToPath(import.meta.url))

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, __dirname, '')
  // 可选：未配置 VITE_API_BASE_URL 时 dev 代理目标；配置了 VITE_API_BASE_URL 则前端直连 API，代理可忽略
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
