import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    proxy: {
      // Forward API and auth traffic to the Go server during development.
      '/api': 'http://localhost:8080',
      '/auth': 'http://localhost:8080',
    },
  },
})
