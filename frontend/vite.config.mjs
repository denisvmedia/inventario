import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import path from 'path'

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src')
    }
  },
  server: {
    proxy: {
      '/api': {
        target: 'http://localhost:3333',
        changeOrigin: true
      }
    }
  },
  build: {
    rollupOptions: {
      output: {
        // Ensure assets are properly named
        assetFileNames: (assetInfo) => {
          if (assetInfo.name && assetInfo.name.includes('pdf.worker')) {
            return 'assets/pdf.worker.min-[hash].mjs'
          }
          return 'assets/[name]-[hash][extname]'
        }
      }
    }
  },
  optimizeDeps: {
    // Exclude PDF worker from dependency optimization
    exclude: ['pdfjs-dist/build/pdf.worker.min.mjs']
  }
})
