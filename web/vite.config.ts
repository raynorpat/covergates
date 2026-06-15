import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import vuetify from 'vite-plugin-vuetify'
import { fileURLToPath, URL } from 'node:url'

// The Go backend serves dist/index.html through text/template with "." =
// config.Server.Base, and serves hashed assets at /assets/*. Vite forces a
// leading slash on `base`, which would yield `/{{.}}/assets/...` (broken once
// substituted), so we keep base at '/' and rewrite the built asset URLs to be
// prefixed with the Go template token via the gates-base-token plugin. Dev uses
// a normal root base + a proxy to the Go API.
export default defineConfig({
  base: '/',
  plugins: [
    vue(),
    vuetify({ autoImport: true }),
    {
      name: 'gates-base-token',
      apply: 'build',
      enforce: 'post',
      transformIndexHtml(html: string) {
        return html.replace(/(src|href)="\/(assets\/|favicon\.ico)/g, '$1="{{.}}/$2')
      }
    }
  ],
  resolve: {
    alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) }
  },
  server: {
    port: 8081,
    proxy: {
      '/api': { target: process.env.VITE_PROXY || 'http://localhost:8080', changeOrigin: true },
      '/login': { target: process.env.VITE_PROXY || 'http://localhost:8080', changeOrigin: true },
      '/logoff': { target: process.env.VITE_PROXY || 'http://localhost:8080', changeOrigin: true }
    }
  },
  test: {
    globals: true,
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
    server: { deps: { inline: ['vuetify'] } },
    coverage: {
      provider: 'v8',
      reporter: ['text', 'lcov'],
      reportsDirectory: 'coverage',
      include: ['src/**']
    }
  }
})
