import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import Icons from 'unplugin-icons/vite'
import IconsResolver from 'unplugin-icons/resolver'
import Components from 'unplugin-vue-components/vite'

export default defineConfig({
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
    port: 33339,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
        ws: true,
        secure: false
      }
    }
  },
  optimizeDeps: {
    exclude: ['monaco-editor']
  }
})
