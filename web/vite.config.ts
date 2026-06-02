import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  server: {
    host: '0.0.0.0',
    port: 9981,
    allowedHosts: ['www.yxdthird.top', 'yxdthird.top', 'localhost'],
    proxy: {
      '/api': {
        target: 'http://localhost:9981',
        changeOrigin: true,
      },
      '/health': {
        target: 'http://localhost:9981',
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: 'dist',
    assetsDir: 'assets',
    emptyOutDir: true,
    // 目标现代浏览器，减少 polyfill
    target: 'es2020',
    // 启用 CSS 代码分割
    cssCodeSplit: true,
    // 低于此大小的资源内联为 base64
    assetsInlineLimit: 4096,
    // 调整 chunk 大小警告阈值
    chunkSizeWarningLimit: 600,
    rollupOptions: {
      output: {
        // 手动分包策略
        manualChunks: {
          // Vue 全家桶
          'vendor-vue': ['vue', 'vue-router', 'pinia'],
          // Arco Design UI
          'vendor-arco': ['@arco-design/web-vue'],
          // 图表 (按需加载，放一起)
          'vendor-charts': ['echarts', 'vue-echarts', 'chart.js'],
          // Xterm 终端
          'vendor-xterm': ['@xterm/xterm', '@xterm/addon-fit', '@xterm/addon-search', '@xterm/addon-web-links'],
          // 工具库
          'vendor-utils': ['axios', 'dompurify', 'marked'],
        },
        // 入口 chunk 命名
        entryFileNames: 'assets/[name]-[hash:8].js',
        chunkFileNames: (chunkInfo) => {
          // vendor chunks 使用简短 hash
          if (chunkInfo.name.startsWith('vendor-')) {
            return 'assets/[name]-[hash:8].js'
          }
          return 'assets/[name]-[hash:8].js'
        },
        assetFileNames: 'assets/[name]-[hash:8].[ext]',
      },
    },
    // 压缩配置 (terser 比 esbuild 压缩率更好)
    minify: 'terser',
    terserOptions: {
      compress: {
        drop_debugger: true,
        pure_funcs: ['console.debug'],
      },
    },
    // 生成 sourcemap 仅在生产调试时开启
    sourcemap: false,
  },
})
