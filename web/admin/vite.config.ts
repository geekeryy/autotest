import { defineConfig, loadEnv } from 'vite'
import vue from '@vitejs/plugin-vue'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { resolveDevApiProxyTarget } from './scripts/resolveDevApiProxy.mjs'

const __dirname = dirname(fileURLToPath(import.meta.url))

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, __dirname, '')
  // 未配置 VITE_API_BASE_URL 时走 /api 代理；目标默认识别局域网 IP，避免 localhost:8080 被 IDE 占用
  const proxyTarget = resolveDevApiProxyTarget(env.VITE_DEV_API_PROXY)
  if (mode === 'development') {
    console.log(`[vite] API proxy target: ${proxyTarget}`)
  }

  if (mode === 'production' && !env.VITE_API_BASE_URL?.trim()) {
    console.warn(
      '[vite] VITE_API_BASE_URL is not set. Same-origin (All-in-One) builds are OK; ' +
        'split deployments (Firebase + API) require it for OAuth redirects and API calls.'
    )
  }

  return {
    plugins: [vue()],
    resolve: {
      alias: {
        '@': resolve(__dirname, 'src'),
      },
    },
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
