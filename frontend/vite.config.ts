import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// 开发服务器：REST / WS 均反向代理到本机 Go 后端（:8080）
// 同源后浏览器请求不带跨域，WS 也经 Vite 代理升级
export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
        ws: true, // /api/v1/interviews/{id}/ws 经此代理升级
      },
    },
  },
})
