import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  server: {
    host: '0.0.0.0',
    port: 3000,  // 指定前端端口
    allowedHosts: ['corain.fun'], 
    proxy: {
      '/api': {
        target: 'http://0.0.0.0:8080',  // 明确后端地址，支持 WebSocket 代理
        changeOrigin: true,
        secure: false,
        ws: true
      }
    }
  }
})
