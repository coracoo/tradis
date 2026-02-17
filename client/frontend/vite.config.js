import { defineConfig, loadEnv } from 'vite'
import vue from '@vitejs/plugin-vue'
import Icons from 'unplugin-icons/vite'
import IconsResolver from 'unplugin-icons/resolver'
import Components from 'unplugin-vue-components/vite'

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '')
  const frontendPort = Number.parseInt(env.FRONTEND_PORT || process.env.FRONTEND_PORT || '33339', 10)
  const backendPort = Number.parseInt(env.BACKEND_PORT || process.env.BACKEND_PORT || '8080', 10)

  return {
    plugins: [
      vue(),
      Components({
        resolvers: [
          IconsResolver({
            prefix: 'Icon',
            enabledCollections: ['ep', 'mdi', 'lucide', 'simple-icons']
          })
        ]
      }),
      Icons({ autoInstall: true })
    ],
    base: '/',
    server: {
      host: '0.0.0.0',
      port: Number.isFinite(frontendPort) ? frontendPort : 33339,
      proxy: {
        '/api': {
          target: `http://localhost:${Number.isFinite(backendPort) ? backendPort : 8080}`,
          changeOrigin: true,
          ws: true,
          secure: false
        }
      }
    },
    optimizeDeps: {
      exclude: ['monaco-editor']
    }
  }
})
