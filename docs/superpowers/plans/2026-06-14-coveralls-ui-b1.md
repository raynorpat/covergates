# Coveralls UI B1 — Vue 3 Foundation + Dashboard Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the Vue 2 frontend with a Vue 3 / Vuetify 3 app (Vite + Pinia + `<script setup>` + TS) providing the app shell, auth/user bootstrap, and the repository dashboard (activate + upload token), served by the existing Go/togo backend.

**Architecture:** A fresh Vue 3 SPA under `web/`, built by Vite into `dist/` (still embedded by togo and served by the Go backend). Pinia stores own data (`user`, `repository`); views are thin and use the shared Axios client. The backend keeps templating `index.html` (Go `{{.}}` → `config.Server.Base`) and is updated to serve Vite's `/assets/*` output. The deep `/report/*` pages are stubbed for B2/B3.

**Tech Stack:** Vue 3.4, Vuetify 3, Vue Router 4, Pinia 2, Vite 5, TypeScript, Vitest + @vue/test-utils 2, Axios. Node 24/npm 11 available locally.

**Conventions (CLAUDE.md):**
- Commit messages: short, **subsystem-prefixed** (`web:`), one–two lines, **no metadata/trailers**.
- Surgical changes; match existing patterns; simplicity first.
- Run frontend commands from `D:/raynorpat/covergates/web`. Run Go commands from repo root.
- A gitignored `web/web_gen.go` stub exists so `go build ./...` works locally; never commit it.

**Base-path mechanism (used throughout):** The backend serves the SPA by running `dist/index.html` through Go `text/template` with `.` = `config.Server.Base`, and serves hashed assets at `/assets/*`. Therefore:
- `index.html` contains `<script>window.VUE_BASE = '{{.}}'</script>`; the SPA reads `window.VUE_BASE` at runtime for router history base + API base (JS bundles are NOT templated, so the base cannot come from `import.meta.env.BASE_URL`).
- Vite production `base` is set to the template token so asset URLs in `index.html` get the `{{.}}` prefix (substituted at serve time). **Primary:** `base: '{{.}}/'`. Task 1 verifies the built `index.html` actually contains `{{.}}/assets/...`; if Vite mangles it, apply the documented fallback in Task 1.
- B1 uses **static (eager) route imports** (no lazy `() => import()`), so no dynamic chunk URLs need runtime base resolution.

---

## File structure (web/)

Created/replaced:
- `web/package.json` — Vue 3 deps + scripts (replace).
- `web/vite.config.ts`, `web/vitest.config.ts` (or merged), `web/tsconfig.json`, `web/tsconfig.node.json`, `web/env.d.ts` — tooling.
- `web/index.html` — Vite entry at `web/` root (with `VUE_BASE` + Go token).
- `web/public/` — `favicon.ico`, `logo.png` (copied from old `web/public`).
- `web/src/main.ts` — app bootstrap.
- `web/src/App.vue` — root.
- `web/src/plugins/vuetify.ts` — Vuetify 3 + theme.
- `web/src/plugins/http.ts` — Axios instance + base + error helper.
- `web/src/lib/base.ts` — reads `window.VUE_BASE`.
- `web/src/router/index.ts` — routes + guards.
- `web/src/stores/user.ts`, `web/src/stores/repository.ts` — Pinia stores.
- `web/src/types/index.ts` — `User`, `Repository` types.
- `web/src/layouts/DefaultLayout.vue` — shell.
- `web/src/components/AccountButton.vue`, `RepoList.vue`, `RepoListItem.vue`.
- `web/src/views/Home.vue`, `Login.vue`, `Dashboard.vue`, `User.vue`, `RepoPlaceholder.vue`.
- `web/src/test/setup.ts` — Vitest setup (Vuetify + ResizeObserver).
- `web/src/**/__tests__/*.spec.ts` — tests.

Deleted: the entire old Vue 2 `web/src` tree, `web/vue.config.js`, `web/babel.config.js`, `web/jest.config.js`, `web/.browserslistrc`, `web/tsconfig.json` (old), `web/jsconfig.json`, `web/.eslintrc.js` (replaced), `web/public/index.html` (moved to `web/index.html`).

Modified (backend): `routers/web/web.go` (serve `/assets/*`).

---

## Task 1: Vite + Vue 3 scaffold and base-path mechanism

**Files:**
- Replace: `web/package.json`
- Create: `web/vite.config.ts`, `web/tsconfig.json`, `web/tsconfig.node.json`, `web/env.d.ts`, `web/index.html`, `web/src/main.ts`, `web/src/App.vue`, `web/src/lib/base.ts`
- Delete: `web/vue.config.js`, `web/babel.config.js`, `web/jsconfig.json`, `web/.browserslistrc`, `web/public/index.html`

- [ ] **Step 1: Replace `web/package.json`**

```json
{
  "name": "covergates",
  "version": "0.1.0",
  "private": true,
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "vue-tsc --noEmit && vite build",
    "preview": "vite preview",
    "test:unit": "vitest run",
    "lint": "eslint . --ext .vue,.ts"
  },
  "dependencies": {
    "axios": "^1.7.7",
    "highlight.js": "^11.10.0",
    "pinia": "^2.2.4",
    "vue": "^3.5.12",
    "vue-router": "^4.4.5",
    "vuetify": "^3.7.3"
  },
  "devDependencies": {
    "@mdi/font": "^7.4.47",
    "@vitejs/plugin-vue": "^5.1.4",
    "@vue/test-utils": "^2.4.6",
    "eslint": "^8.57.1",
    "eslint-plugin-vue": "^9.28.0",
    "jsdom": "^25.0.1",
    "sass": "^1.80.3",
    "typescript": "^5.6.3",
    "vite": "^5.4.9",
    "vite-plugin-vuetify": "^2.0.4",
    "vitest": "^2.1.3",
    "vue-tsc": "^2.1.6",
    "@vue/eslint-config-typescript": "^14.1.3"
  }
}
```

- [ ] **Step 2: Create `web/vite.config.ts`**

```ts
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import vuetify from 'vite-plugin-vuetify'
import { fileURLToPath, URL } from 'node:url'

// The Go backend serves dist/index.html through text/template with "." =
// config.Server.Base, and serves hashed assets at /assets/*. In production we
// prefix asset URLs with the Go template token so the base path is injected at
// serve time. Dev uses a normal root base + a proxy to the Go API.
export default defineConfig(({ mode }) => ({
  base: mode === 'production' ? '{{.}}/' : '/',
  plugins: [vue(), vuetify({ autoImport: true })],
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
    server: { deps: { inline: ['vuetify'] } }
  }
}))
```

- [ ] **Step 3: Create `web/tsconfig.json`**

```json
{
  "compilerOptions": {
    "target": "ESNext",
    "module": "ESNext",
    "moduleResolution": "Bundler",
    "strict": true,
    "jsx": "preserve",
    "resolveJsonModule": true,
    "esModuleInterop": true,
    "lib": ["ESNext", "DOM", "DOM.Iterable"],
    "skipLibCheck": true,
    "noEmit": true,
    "types": ["vitest/globals", "vuetify"],
    "baseUrl": ".",
    "paths": { "@/*": ["src/*"] }
  },
  "include": ["src/**/*.ts", "src/**/*.vue", "env.d.ts"],
  "references": [{ "path": "./tsconfig.node.json" }]
}
```

- [ ] **Step 4: Create `web/tsconfig.node.json`**

```json
{
  "compilerOptions": {
    "composite": true,
    "module": "ESNext",
    "moduleResolution": "Bundler",
    "skipLibCheck": true,
    "types": ["node"]
  },
  "include": ["vite.config.ts"]
}
```

- [ ] **Step 5: Create `web/env.d.ts`**

```ts
/// <reference types="vite/client" />
/// <reference types="vuetify" />

declare module '*.vue' {
  import type { DefineComponent } from 'vue'
  const component: DefineComponent<{}, {}, any>
  export default component
}

declare global {
  interface Window {
    VUE_BASE?: string
  }
}
export {}
```

- [ ] **Step 6: Create `web/index.html`** (Vite entry at web/ root)

```html
<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <link rel="icon" href="{{.}}/favicon.ico" />
    <title>Covergates</title>
    <link rel="stylesheet" href="https://fonts.googleapis.com/css?family=Roboto:100,300,400,500,700,900" />
    <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/@mdi/font@7.4.47/css/materialdesignicons.min.css" />
    <script>
      window.VUE_BASE = '{{.}}'
    </script>
  </head>
  <body>
    <div id="app"></div>
    <script type="module" src="/src/main.ts"></script>
  </body>
</html>
```

- [ ] **Step 7: Create `web/src/lib/base.ts`**

```ts
// Runtime base path, injected into index.html by the Go template (window.VUE_BASE).
// Empty string when hosted at root. JS bundles are not templated, so the base must
// be read at runtime, never from import.meta.env.BASE_URL.
export function basePath(): string {
  const raw = window.VUE_BASE ?? ''
  // Guard against the un-substituted template token during local `vite preview`.
  if (raw.includes('{{')) return ''
  return raw.replace(/\/$/, '')
}
```

- [ ] **Step 8: Create `web/src/App.vue`**

```vue
<script setup lang="ts">
import DefaultLayout from '@/layouts/DefaultLayout.vue'
</script>

<template>
  <DefaultLayout />
</template>
```

- [ ] **Step 9: Create a temporary minimal `web/src/main.ts`** (router/stores added in later tasks; keep it compiling now)

```ts
import { createApp } from 'vue'
import App from './App.vue'
import vuetify from './plugins/vuetify'
import { createPinia } from 'pinia'
import router from './router'

createApp(App).use(createPinia()).use(router).use(vuetify).mount('#app')
```

NOTE: `main.ts` imports `./plugins/vuetify` and `./router`, created in Tasks 2 and 5. To keep Task 1 self-contained and buildable, also create minimal stubs now and flesh them out later: see Steps 10–11.

- [ ] **Step 10: Create minimal `web/src/plugins/vuetify.ts` stub** (full theme in Task 2)

```ts
import 'vuetify/styles'
import { createVuetify } from 'vuetify'

export default createVuetify()
```

- [ ] **Step 11: Create minimal `web/src/router/index.ts` stub** (full routes in Task 5)

```ts
import { createRouter, createWebHistory } from 'vue-router'
import { basePath } from '@/lib/base'

export default createRouter({
  history: createWebHistory(basePath() + '/'),
  routes: [{ path: '/', name: 'home', component: () => import('@/App.vue') }]
})
```

NOTE: this stub uses a placeholder route to compile; Task 5 replaces it (and removes the lazy import). Also create a minimal `DefaultLayout.vue` so `App.vue` compiles:

- [ ] **Step 12: Create minimal `web/src/layouts/DefaultLayout.vue` stub** (full shell in Task 8)

```vue
<script setup lang="ts"></script>

<template>
  <v-app>
    <v-main>
      <router-view />
    </v-main>
  </v-app>
</template>
```

- [ ] **Step 13: Delete old build config**

```bash
git rm web/vue.config.js web/babel.config.js web/jsconfig.json web/.browserslistrc web/public/index.html
```

- [ ] **Step 14: Install and build; verify the base mechanism**

```bash
cd web && npm install && npm run build
```
Expected: build succeeds, produces `web/dist/index.html` and `web/dist/assets/*`.
Then inspect: `grep -o '{{\.}}/assets/[^"]*' dist/index.html | head` — Expected: asset URLs prefixed with `{{.}}/assets/...`, and `window.VUE_BASE = '{{.}}'` present in `dist/index.html`.

**Fallback (only if `base: '{{.}}/'` is rejected/mangled by Vite):** set `base: '/'` in `vite.config.ts` and add a small plugin to prefix built asset URLs. Insert into `plugins`:
```ts
{
  name: 'gates-base-token',
  apply: 'build',
  enforce: 'post',
  transformIndexHtml(html: string) {
    return html.replace(/(src|href)="\/(assets\/|favicon\.ico)/g, '$1="{{.}}/$2')
  }
}
```
Re-run `npm run build` and re-verify the grep. Use whichever variant produces `{{.}}/assets/...` in `dist/index.html`.

- [ ] **Step 15: Commit**

```bash
cd .. && git add web/package.json web/vite.config.ts web/tsconfig.json web/tsconfig.node.json web/env.d.ts web/index.html web/src/main.ts web/src/App.vue web/src/lib/base.ts web/src/plugins/vuetify.ts web/src/router/index.ts web/src/layouts/DefaultLayout.vue web/package-lock.json
git rm -r --cached web/vue.config.js web/babel.config.js web/jsconfig.json web/.browserslistrc web/public/index.html 2>/dev/null; true
git commit -m "web: scaffold Vue 3 + Vite build"
```
(Do not `git add web/web_gen.go` or `web/dist` or `web/node_modules` — ensure `web/.gitignore` ignores `dist` and `node_modules`; add `dist` to `web/.gitignore` if missing.)

---

## Task 2: Vuetify 3 theme

**Files:**
- Replace: `web/src/plugins/vuetify.ts`

- [ ] **Step 1: Write the full Vuetify plugin** (ports the current cyan/grey palette)

```ts
import 'vuetify/styles'
import { createVuetify } from 'vuetify'
import { aliases, mdi } from 'vuetify/iconsets/mdi'

export default createVuetify({
  icons: { defaultSet: 'mdi', aliases, sets: { mdi } },
  theme: {
    defaultTheme: 'light',
    themes: {
      light: {
        colors: {
          primary: '#00838F',    // cyan darken-3
          secondary: '#212121',  // grey darken-4
          accent: '#546E7A',     // blue-grey darken-1
          error: '#B71C1C'       // red darken-4
        }
      }
    }
  }
})
```

- [ ] **Step 2: Build to verify**

Run: `cd web && npm run build`
Expected: success.

- [ ] **Step 3: Commit**

```bash
cd .. && git add web/src/plugins/vuetify.ts && git commit -m "web: vuetify 3 theme"
```

---

## Task 3: HTTP client and shared types

**Files:**
- Create: `web/src/plugins/http.ts`, `web/src/types/index.ts`

- [ ] **Step 1: Create `web/src/types/index.ts`**

```ts
export type SCM = 'github' | 'gitea' | 'gitlab'

export interface User {
  login: string
  avatar?: string
  email?: string
}

export interface Repository {
  ID: number
  URL: string
  ReportID: string
  NameSpace: string
  Name: string
  Branch: string
  Private: boolean
  SCM: SCM
}
```
NOTE: field names match the Go `core.Repo`/`core.User` JSON (Go exports `ReportID`, `NameSpace`, etc.; `Token` is intentionally absent — it is `json:"-"` and fetched separately).

- [ ] **Step 2: Create `web/src/plugins/http.ts`**

```ts
import axios from 'axios'
import { basePath } from '@/lib/base'

const http = axios.create({ baseURL: basePath() })

export default http

// Extract a human-readable message from an Axios error.
export function errorMessage(err: unknown): string {
  if (axios.isAxiosError(err)) {
    const data = err.response?.data as { message?: string } | string | undefined
    if (typeof data === 'string' && data) return data
    if (data && typeof data === 'object' && data.message) return data.message
    return err.message
  }
  return String(err)
}
```

- [ ] **Step 3: Type-check**

Run: `cd web && npx vue-tsc --noEmit`
Expected: no errors.

- [ ] **Step 4: Commit**

```bash
cd .. && git add web/src/plugins/http.ts web/src/types/index.ts && git commit -m "web: http client and shared types"
```

---

## Task 4: Backend serves Vite asset output

**Files:**
- Modify: `routers/web/web.go`

- [ ] **Step 1: Replace the static asset routes**

In `routers/web/web.go` `RegisterRoutes`, replace the four Vue-CLI asset routes with the Vite `assets` route. Change:
```go
	e.GET("/favicon.ico", h)
	e.GET("/logo.png", h)
	e.GET("/js/*filepath", h)
	e.GET("/css/*filepath", h)
	e.GET("/img/*filepath", h)
	e.GET("/fonts/*filepath", h)
```
to:
```go
	e.GET("/favicon.ico", h)
	e.GET("/logo.png", h)
	e.GET("/assets/*filepath", h)
```

- [ ] **Step 2: Verify backend builds**

Run (repo root): `go build ./...`
Expected: success (the gitignored `web/web_gen.go` stub satisfies the import locally).

- [ ] **Step 3: Commit**

```bash
git add routers/web/web.go && git commit -m "web: serve vite assets directory"
```

---

## Task 5: Router, guards, and stub/static views

**Files:**
- Replace: `web/src/router/index.ts`
- Create: `web/src/views/Home.vue`, `web/src/views/RepoPlaceholder.vue`

- [ ] **Step 1: Create `web/src/views/Home.vue`** (simple landing)

```vue
<script setup lang="ts">
import { useRouter } from 'vue-router'
const router = useRouter()
</script>

<template>
  <v-container class="py-12">
    <v-row justify="center">
      <v-col cols="12" md="8" class="text-center">
        <h1 class="text-h3 mb-4">Covergates</h1>
        <p class="text-body-1 mb-8">Self-hosted coverage reports, coveralls-compatible.</p>
        <v-btn color="primary" size="large" @click="router.push('/repos')">Get started</v-btn>
      </v-col>
    </v-row>
  </v-container>
</template>
```

- [ ] **Step 2: Create `web/src/views/RepoPlaceholder.vue`** (stub until B2)

```vue
<script setup lang="ts">
import { useRoute } from 'vue-router'
const route = useRoute()
</script>

<template>
  <v-container class="py-12 text-center">
    <h2 class="text-h5 mb-2">{{ route.params.namespace }}/{{ route.params.name }}</h2>
    <p class="text-body-2">Build reports for this repository are coming soon.</p>
  </v-container>
</template>
```

- [ ] **Step 3: Replace `web/src/router/index.ts`** (static imports, auth guard)

```ts
import { createRouter, createWebHistory } from 'vue-router'
import { basePath } from '@/lib/base'
import { useUserStore } from '@/stores/user'
import Home from '@/views/Home.vue'
import Login from '@/views/Login.vue'
import Dashboard from '@/views/Dashboard.vue'
import User from '@/views/User.vue'
import RepoPlaceholder from '@/views/RepoPlaceholder.vue'

const router = createRouter({
  history: createWebHistory(basePath() + '/'),
  routes: [
    { path: '/', name: 'home', component: Home },
    { path: '/login', name: 'login', component: Login },
    { path: '/repos', name: 'dashboard', component: Dashboard, meta: { requiresAuth: true } },
    { path: '/user', name: 'user', component: User, meta: { requiresAuth: true } },
    {
      path: '/report/:scm/:namespace/:name',
      name: 'repo',
      component: RepoPlaceholder,
      meta: { requiresAuth: true }
    },
    { path: '/:pathMatch(.*)*', redirect: '/' }
  ]
})

router.beforeEach((to) => {
  if (!to.meta.requiresAuth) return true
  const user = useUserStore()
  if (user.isAuthenticated) return true
  return { name: 'login', query: { redirect: to.fullPath } }
})

export default router
```
NOTE: `Login.vue`, `Dashboard.vue`, `User.vue` are created in Tasks 9/11/10; `useUserStore` in Task 6. This task's build runs after those exist — order the commits so the tree compiles, OR temporarily stub the missing imports. Since subagent-driven execution runs tasks in order, create the three view files as minimal stubs here if they don't exist yet:

- [ ] **Step 4: Create minimal stubs** for `web/src/views/Login.vue`, `web/src/views/Dashboard.vue`, `web/src/views/User.vue` (replaced in later tasks):

```vue
<script setup lang="ts"></script>
<template><v-container /></template>
```
(Write the same content to all three files.)

- [ ] **Step 5: Build**

Run: `cd web && npm run build`
Expected: success. (`useUserStore` import will fail until Task 6 — if executing strictly in order, do Task 6 before building; otherwise create the store stub from Task 6 Step 1 now.)

- [ ] **Step 6: Commit**

```bash
cd .. && git add web/src/router/index.ts web/src/views/Home.vue web/src/views/RepoPlaceholder.vue web/src/views/Login.vue web/src/views/Dashboard.vue web/src/views/User.vue
git commit -m "web: router, guards, home and placeholder views"
```

---

## Task 6: User store

**Files:**
- Create: `web/src/stores/user.ts`, `web/src/test/setup.ts`, `web/src/stores/__tests__/user.spec.ts`

- [ ] **Step 1: Create `web/src/test/setup.ts`**

```ts
import { vi } from 'vitest'

// Vuetify components query layout APIs jsdom lacks.
globalThis.ResizeObserver = class {
  observe() {}
  unobserve() {}
  disconnect() {}
} as unknown as typeof ResizeObserver

if (!window.matchMedia) {
  window.matchMedia = vi.fn().mockImplementation((query: string) => ({
    matches: false, media: query, onchange: null,
    addEventListener: () => {}, removeEventListener: () => {},
    addListener: () => {}, removeListener: () => {}, dispatchEvent: () => false
  }))
}
```

- [ ] **Step 2: Write the failing test `web/src/stores/__tests__/user.spec.ts`**

```ts
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import http from '@/plugins/http'
import { useUserStore } from '@/stores/user'

vi.mock('@/plugins/http', () => ({ default: { get: vi.fn() }, errorMessage: (e: unknown) => String(e) }))

describe('user store', () => {
  beforeEach(() => setActivePinia(createPinia()))

  it('fetch() populates current and isAuthenticated', async () => {
    ;(http.get as any).mockResolvedValue({ data: { login: 'alice' } })
    const store = useUserStore()
    expect(store.isAuthenticated).toBe(false)
    await store.fetch()
    expect(store.current?.login).toBe('alice')
    expect(store.isAuthenticated).toBe(true)
  })

  it('fetch() leaves user null on error', async () => {
    ;(http.get as any).mockRejectedValue(new Error('401'))
    const store = useUserStore()
    await store.fetch()
    expect(store.current).toBeNull()
    expect(store.isAuthenticated).toBe(false)
  })
})
```

- [ ] **Step 3: Run to verify it fails**

Run: `cd web && npx vitest run src/stores/__tests__/user.spec.ts`
Expected: FAIL (cannot resolve `@/stores/user`).

- [ ] **Step 4: Create `web/src/stores/user.ts`**

```ts
import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import http from '@/plugins/http'
import type { User } from '@/types'

export const useUserStore = defineStore('user', () => {
  const current = ref<User | null>(null)
  const isAuthenticated = computed(() => current.value !== null)

  async function fetch() {
    try {
      const { data } = await http.get<User>('/api/v1/user')
      current.value = data && data.login ? data : null
    } catch {
      current.value = null
    }
  }

  return { current, isAuthenticated, fetch }
})
```

- [ ] **Step 5: Run to verify it passes**

Run: `cd web && npx vitest run src/stores/__tests__/user.spec.ts`
Expected: PASS (2 tests).

- [ ] **Step 6: Commit**

```bash
cd .. && git add web/src/stores/user.ts web/src/test/setup.ts web/src/stores/__tests__/user.spec.ts
git commit -m "web: user store"
```

---

## Task 7: Repository store

**Files:**
- Create: `web/src/stores/repository.ts`, `web/src/stores/__tests__/repository.spec.ts`

- [ ] **Step 1: Write the failing test `web/src/stores/__tests__/repository.spec.ts`**

```ts
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import http from '@/plugins/http'
import { useRepositoryStore } from '@/stores/repository'

vi.mock('@/plugins/http', () => ({ default: { get: vi.fn(), patch: vi.fn() }, errorMessage: (e: unknown) => String(e) }))

describe('repository store', () => {
  beforeEach(() => setActivePinia(createPinia()))

  it('fetchList() populates list', async () => {
    ;(http.get as any).mockResolvedValue({ data: [{ ID: 1, Name: 'r', NameSpace: 'o' }] })
    const store = useRepositoryStore()
    await store.fetchList()
    expect(store.list).toHaveLength(1)
    expect(store.list[0].Name).toBe('r')
  })

  it('synchronize() PATCHes then refetches', async () => {
    ;(http.patch as any).mockResolvedValue({})
    ;(http.get as any).mockResolvedValue({ data: [] })
    const store = useRepositoryStore()
    await store.synchronize()
    expect(http.patch).toHaveBeenCalledWith('/api/v1/user/repos')
    expect(http.get).toHaveBeenCalledWith('/api/v1/user/repos')
  })
})
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd web && npx vitest run src/stores/__tests__/repository.spec.ts`
Expected: FAIL (cannot resolve `@/stores/repository`).

- [ ] **Step 3: Create `web/src/stores/repository.ts`**

```ts
import { defineStore } from 'pinia'
import { ref } from 'vue'
import http from '@/plugins/http'
import type { Repository } from '@/types'

export const useRepositoryStore = defineStore('repository', () => {
  const list = ref<Repository[]>([])

  async function fetchList() {
    const { data } = await http.get<Repository[]>('/api/v1/user/repos')
    list.value = data ?? []
  }

  async function synchronize() {
    await http.patch('/api/v1/user/repos')
    await fetchList()
  }

  return { list, fetchList, synchronize }
})
```

- [ ] **Step 4: Run to verify it passes**

Run: `cd web && npx vitest run src/stores/__tests__/repository.spec.ts`
Expected: PASS (2 tests).

- [ ] **Step 5: Commit**

```bash
cd .. && git add web/src/stores/repository.ts web/src/stores/__tests__/repository.spec.ts
git commit -m "web: repository store"
```

---

## Task 8: App shell (DefaultLayout + AccountButton)

**Files:**
- Replace: `web/src/layouts/DefaultLayout.vue`
- Create: `web/src/components/AccountButton.vue`, `web/src/components/__tests__/AccountButton.spec.ts`
- Update: `web/src/main.ts` (bootstrap user before mount)

- [ ] **Step 1: Create `web/src/components/AccountButton.vue`**

```vue
<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { useUserStore } from '@/stores/user'
import { basePath } from '@/lib/base'

const user = useUserStore()
const router = useRouter()
const authed = computed(() => user.isAuthenticated)

function login() { router.push('/login') }
function logout() { window.location.href = basePath() + '/logoff' }
</script>

<template>
  <v-menu v-if="authed">
    <template #activator="{ props }">
      <v-btn icon v-bind="props" aria-label="account">
        <v-avatar size="32">
          <v-img v-if="user.current?.avatar" :src="user.current.avatar" />
          <v-icon v-else>mdi-account-circle</v-icon>
        </v-avatar>
      </v-btn>
    </template>
    <v-list>
      <v-list-item title="Settings" @click="router.push('/user')" />
      <v-list-item title="Logout" @click="logout" />
    </v-list>
  </v-menu>
  <v-btn v-else variant="text" @click="login">Login</v-btn>
</template>
```

- [ ] **Step 2: Replace `web/src/layouts/DefaultLayout.vue`**

```vue
<script setup lang="ts">
import { useRouter } from 'vue-router'
import AccountButton from '@/components/AccountButton.vue'
const router = useRouter()
</script>

<template>
  <v-app>
    <v-app-bar color="primary" density="comfortable" flat>
      <v-app-bar-title style="cursor: pointer" @click="router.push('/')">Covergates</v-app-bar-title>
      <v-btn variant="text" @click="router.push('/repos')">Repositories</v-btn>
      <AccountButton />
    </v-app-bar>
    <v-main>
      <router-view />
    </v-main>
    <v-footer color="secondary" class="justify-center text-caption">
      <span>Covergates</span>
    </v-footer>
  </v-app>
</template>
```

- [ ] **Step 3: Update `web/src/main.ts`** to bootstrap the user before mount

```ts
import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import router from './router'
import vuetify from './plugins/vuetify'
import { useUserStore } from './stores/user'

const app = createApp(App)
const pinia = createPinia()
app.use(pinia).use(router).use(vuetify)

// Resolve auth state before the first navigation guard runs.
const user = useUserStore(pinia)
user.fetch().finally(() => {
  app.mount('#app')
})
```

- [ ] **Step 4: Write `web/src/components/__tests__/AccountButton.spec.ts`**

```ts
import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import { setActivePinia, createPinia } from 'pinia'
import { createRouter, createMemoryHistory } from 'vue-router'
import AccountButton from '@/components/AccountButton.vue'
import { useUserStore } from '@/stores/user'

const vuetify = createVuetify({ components })
const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/', component: { template: '<div/>' } }] })

function mountButton() {
  return mount(AccountButton, { global: { plugins: [vuetify, router] } })
}

describe('AccountButton', () => {
  beforeEach(() => setActivePinia(createPinia()))

  it('shows Login when unauthenticated', () => {
    const w = mountButton()
    expect(w.text()).toContain('Login')
  })

  it('shows account menu when authenticated', async () => {
    const store = useUserStore()
    store.current = { login: 'alice' }
    const w = mountButton()
    expect(w.text()).not.toContain('Login')
    expect(w.find('[aria-label="account"]').exists()).toBe(true)
  })
})
```

- [ ] **Step 5: Run tests**

Run: `cd web && npx vitest run src/components/__tests__/AccountButton.spec.ts`
Expected: PASS (2 tests).

- [ ] **Step 6: Build + commit**

```bash
cd web && npm run build && cd ..
git add web/src/layouts/DefaultLayout.vue web/src/components/AccountButton.vue web/src/components/__tests__/AccountButton.spec.ts web/src/main.ts
git commit -m "web: app shell and account button"
```

---

## Task 9: Login view

**Files:**
- Replace: `web/src/views/Login.vue`

- [ ] **Step 1: Write `web/src/views/Login.vue`**

```vue
<script setup lang="ts">
import { useRoute } from 'vue-router'
import { basePath } from '@/lib/base'
import type { SCM } from '@/types'

const route = useRoute()

function loginUrl(scm: SCM): string {
  const redirect = (route.query.redirect as string) || '/'
  const params = new URLSearchParams({ redirect })
  return `${basePath()}/login/${scm}?${params.toString()}`
}

const providers: { scm: SCM; label: string; icon: string }[] = [
  { scm: 'github', label: 'GitHub', icon: 'mdi-github' },
  { scm: 'gitea', label: 'Gitea', icon: 'mdi-git' },
  { scm: 'gitlab', label: 'GitLab', icon: 'mdi-gitlab' }
]
</script>

<template>
  <v-container class="py-12">
    <v-row justify="center">
      <v-col cols="12" sm="6" md="4">
        <v-card class="pa-6 text-center">
          <h2 class="text-h5 mb-6">Sign in</h2>
          <v-btn
            v-for="p in providers"
            :key="p.scm"
            block
            class="mb-3"
            color="primary"
            :prepend-icon="p.icon"
            :href="loginUrl(p.scm)"
          >{{ p.label }}</v-btn>
        </v-card>
      </v-col>
    </v-row>
  </v-container>
</template>
```

NOTE: the backend OAuth entrypoint is `GET /login/:scm` (the backend's web router handles `/login`); the `?bind` flow used by account linking is added in B3.

- [ ] **Step 2: Build**

Run: `cd web && npm run build`
Expected: success.

- [ ] **Step 3: Commit**

```bash
cd .. && git add web/src/views/Login.vue && git commit -m "web: login view"
```

---

## Task 10: User (account) view

**Files:**
- Replace: `web/src/views/User.vue`

- [ ] **Step 1: Write `web/src/views/User.vue`**

```vue
<script setup lang="ts">
import { onMounted } from 'vue'
import { useUserStore } from '@/stores/user'

const user = useUserStore()
onMounted(() => { if (!user.current) user.fetch() })
</script>

<template>
  <v-container class="py-8">
    <v-row justify="center">
      <v-col cols="12" md="6">
        <v-card class="pa-6">
          <div class="d-flex align-center mb-4">
            <v-avatar size="64" class="mr-4">
              <v-img v-if="user.current?.avatar" :src="user.current.avatar" />
              <v-icon v-else size="64">mdi-account-circle</v-icon>
            </v-avatar>
            <div>
              <div class="text-h6">{{ user.current?.login }}</div>
              <div class="text-body-2">{{ user.current?.email }}</div>
            </div>
          </div>
        </v-card>
      </v-col>
    </v-row>
  </v-container>
</template>
```
NOTE: SCM account-binding management (`GET /api/v1/user/scm`, link buttons) is deferred to B3; B1 shows the profile.

- [ ] **Step 2: Build + commit**

```bash
cd web && npm run build && cd ..
git add web/src/views/User.vue && git commit -m "web: account view"
```

---

## Task 11: Dashboard, RepoList, RepoListItem (activate + token)

**Files:**
- Replace: `web/src/views/Dashboard.vue`
- Create: `web/src/components/RepoList.vue`, `web/src/components/RepoListItem.vue`
- Create: `web/src/components/__tests__/RepoListItem.spec.ts`, `web/src/views/__tests__/Dashboard.spec.ts`

- [ ] **Step 1: Create `web/src/components/RepoListItem.vue`**

```vue
<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import http, { errorMessage } from '@/plugins/http'
import type { Repository } from '@/types'

const props = defineProps<{ repo: Repository }>()
const emit = defineEmits<{ (e: 'activated'): void; (e: 'error', msg: string): void }>()
const router = useRouter()

const busy = ref(false)
const token = ref('')
const showToken = ref(false)

const repoPath = () => `/api/v1/repos/${props.repo.SCM}/${props.repo.NameSpace}/${props.repo.Name}`

async function activate() {
  busy.value = true
  try {
    try {
      await http.get(repoPath())
    } catch {
      await http.post('/api/v1/repos', props.repo)
    }
    await http.patch(`${repoPath()}/report`)
    emit('activated')
  } catch (e) {
    emit('error', errorMessage(e))
  } finally {
    busy.value = false
  }
}

async function revealToken() {
  busy.value = true
  try {
    const { data } = await http.get<{ token: string }>(`${repoPath()}/token`)
    token.value = data.token
    showToken.value = true
  } catch (e) {
    emit('error', errorMessage(e))
  } finally {
    busy.value = false
  }
}

async function rotateToken() {
  busy.value = true
  try {
    const { data } = await http.patch<{ token: string }>(`${repoPath()}/token`)
    token.value = data.token
    showToken.value = true
  } catch (e) {
    emit('error', errorMessage(e))
  } finally {
    busy.value = false
  }
}

function copyToken() {
  if (token.value) navigator.clipboard?.writeText(token.value)
}

function open() {
  router.push(`/report/${props.repo.SCM}/${props.repo.NameSpace}/${props.repo.Name}`)
}
</script>

<template>
  <v-list-item>
    <template #prepend>
      <v-icon>{{ repo.Private ? 'mdi-lock' : 'mdi-source-repository' }}</v-icon>
    </template>
    <v-list-item-title>{{ repo.NameSpace }}/{{ repo.Name }}</v-list-item-title>
    <v-list-item-subtitle>{{ repo.URL }}</v-list-item-subtitle>

    <template #append>
      <template v-if="repo.ReportID">
        <v-btn variant="text" :loading="busy" @click="showToken ? rotateToken() : revealToken()">
          {{ showToken ? 'Rotate token' : 'Show token' }}
        </v-btn>
        <v-btn icon variant="text" aria-label="open" @click="open"><v-icon>mdi-chevron-right</v-icon></v-btn>
      </template>
      <v-btn v-else color="primary" variant="tonal" :loading="busy" @click="activate">Activate</v-btn>
    </template>
  </v-list-item>

  <v-list-item v-if="showToken">
    <v-text-field
      :model-value="token"
      label="Upload token"
      readonly
      density="compact"
      append-inner-icon="mdi-content-copy"
      @click:append-inner="copyToken"
    />
  </v-list-item>
</template>
```

- [ ] **Step 2: Create `web/src/components/RepoList.vue`**

```vue
<script setup lang="ts">
import RepoListItem from '@/components/RepoListItem.vue'
import type { Repository } from '@/types'

defineProps<{ repos: Repository[] }>()
const emit = defineEmits<{ (e: 'activated'): void; (e: 'error', msg: string): void }>()
</script>

<template>
  <v-list lines="two">
    <RepoListItem
      v-for="repo in repos"
      :key="repo.SCM + repo.NameSpace + repo.Name"
      :repo="repo"
      @activated="emit('activated')"
      @error="(m) => emit('error', m)"
    />
  </v-list>
</template>
```

- [ ] **Step 3: Replace `web/src/views/Dashboard.vue`**

```vue
<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRepositoryStore } from '@/stores/repository'
import { errorMessage } from '@/plugins/http'
import RepoList from '@/components/RepoList.vue'

const store = useRepositoryStore()
const search = ref('')
const busy = ref(false)
const snackbar = ref('')

const filtered = computed(() => {
  const q = search.value.toLowerCase()
  return store.list
    .filter((r) => `${r.NameSpace}/${r.Name}`.toLowerCase().includes(q))
    .sort((a, b) => Number(Boolean(b.ReportID)) - Number(Boolean(a.ReportID)))
})

async function load() {
  busy.value = true
  try { await store.fetchList() } catch (e) { snackbar.value = errorMessage(e) } finally { busy.value = false }
}

async function sync() {
  busy.value = true
  try { await store.synchronize() } catch (e) { snackbar.value = errorMessage(e) } finally { busy.value = false }
}

onMounted(load)
</script>

<template>
  <v-container class="py-6">
    <div class="d-flex align-center mb-4 ga-3">
      <v-text-field v-model="search" label="Search repositories" density="compact" hide-details prepend-inner-icon="mdi-magnify" />
      <v-btn color="primary" :loading="busy" prepend-icon="mdi-sync" @click="sync">Sync</v-btn>
    </div>
    <v-progress-linear v-if="busy" indeterminate color="primary" class="mb-2" />
    <v-card>
      <RepoList :repos="filtered" @activated="load" @error="(m) => (snackbar = m)" />
    </v-card>
    <v-snackbar v-model="snackbar" :timeout="4000" color="error">{{ snackbar }}</v-snackbar>
  </v-container>
</template>
```

- [ ] **Step 4: Write `web/src/components/__tests__/RepoListItem.spec.ts`**

```ts
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import { createRouter, createMemoryHistory } from 'vue-router'
import http from '@/plugins/http'
import RepoListItem from '@/components/RepoListItem.vue'
import type { Repository } from '@/types'

vi.mock('@/plugins/http', () => ({
  default: { get: vi.fn(), post: vi.fn(), patch: vi.fn() },
  errorMessage: (e: unknown) => String(e)
}))

const vuetify = createVuetify({ components })
const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/', component: { template: '<div/>' } }, { path: '/report/:scm/:namespace/:name', component: { template: '<div/>' } }] })

const baseRepo: Repository = { ID: 1, URL: 'u', ReportID: '', NameSpace: 'o', Name: 'r', Branch: 'main', Private: false, SCM: 'github' }

function mountItem(repo: Repository) {
  return mount(RepoListItem, { props: { repo }, global: { plugins: [vuetify, router] } })
}

describe('RepoListItem', () => {
  beforeEach(() => vi.clearAllMocks())

  it('activate: GET ok then PATCH report, emits activated', async () => {
    ;(http.get as any).mockResolvedValue({})
    ;(http.patch as any).mockResolvedValue({})
    const w = mountItem({ ...baseRepo })
    await w.find('button').trigger('click')
    await Promise.resolve(); await Promise.resolve()
    expect(http.patch).toHaveBeenCalledWith('/api/v1/repos/github/o/r/report')
    expect(w.emitted('activated')).toBeTruthy()
  })

  it('activate: GET 404 then POST create then PATCH report', async () => {
    ;(http.get as any).mockRejectedValue(new Error('404'))
    ;(http.post as any).mockResolvedValue({})
    ;(http.patch as any).mockResolvedValue({})
    const w = mountItem({ ...baseRepo })
    await w.find('button').trigger('click')
    await Promise.resolve(); await Promise.resolve(); await Promise.resolve()
    expect(http.post).toHaveBeenCalledWith('/api/v1/repos', expect.objectContaining({ Name: 'r' }))
    expect(http.patch).toHaveBeenCalledWith('/api/v1/repos/github/o/r/report')
  })

  it('activated repo reveals token via GET /token', async () => {
    ;(http.get as any).mockResolvedValue({ data: { token: 'secret123' } })
    const w = mountItem({ ...baseRepo, ReportID: 'abc' })
    const showBtn = w.findAll('button').find((b) => b.text().includes('Show token'))!
    await showBtn.trigger('click')
    await Promise.resolve(); await Promise.resolve()
    expect(http.get).toHaveBeenCalledWith('/api/v1/repos/github/o/r/token')
    expect(w.html()).toContain('secret123')
  })
})
```

- [ ] **Step 5: Write `web/src/views/__tests__/Dashboard.spec.ts`**

```ts
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import * as directives from 'vuetify/directives'
import { createRouter, createMemoryHistory } from 'vue-router'
import { setActivePinia, createPinia } from 'pinia'
import http from '@/plugins/http'
import Dashboard from '@/views/Dashboard.vue'

vi.mock('@/plugins/http', () => ({ default: { get: vi.fn(), patch: vi.fn() }, errorMessage: (e: unknown) => String(e) }))

const vuetify = createVuetify({ components, directives })
const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/', component: { template: '<div/>' } }, { path: '/report/:scm/:namespace/:name', component: { template: '<div/>' } }] })

describe('Dashboard', () => {
  beforeEach(() => setActivePinia(createPinia()))

  it('lists repos and filters by search', async () => {
    ;(http.get as any).mockResolvedValue({ data: [
      { ID: 1, URL: 'u', ReportID: 'x', NameSpace: 'o', Name: 'alpha', Branch: 'm', Private: false, SCM: 'github' },
      { ID: 2, URL: 'u', ReportID: '', NameSpace: 'o', Name: 'beta', Branch: 'm', Private: false, SCM: 'github' }
    ] })
    const w = mount(Dashboard, { global: { plugins: [vuetify, router] } })
    await flushPromises()
    expect(w.text()).toContain('o/alpha')
    expect(w.text()).toContain('o/beta')

    const input = w.find('input')
    await input.setValue('alpha')
    expect(w.text()).toContain('o/alpha')
    expect(w.text()).not.toContain('o/beta')
  })
})
```

- [ ] **Step 6: Run the tests**

Run: `cd web && npx vitest run src/components/__tests__/RepoListItem.spec.ts src/views/__tests__/Dashboard.spec.ts`
Expected: PASS. (If Vuetify list/text-field interaction needs directives, the Dashboard test already registers them; add `directives` to other mounts if a test reports a missing directive.)

- [ ] **Step 7: Build + commit**

```bash
cd web && npm run build && cd ..
git add web/src/views/Dashboard.vue web/src/components/RepoList.vue web/src/components/RepoListItem.vue web/src/components/__tests__/RepoListItem.spec.ts web/src/views/__tests__/Dashboard.spec.ts
git commit -m "web: repo dashboard with activate and token"
```

---

## Task 12: Remove old Vue 2 sources, update CI, final verification

**Files:**
- Delete: any remaining old Vue 2 files under `web/src` not already replaced (old `views/`, `components/`, `store/`, `mixins/`, `plugins/`, `router/fetchers.ts`, `server.ts`, `vue.ts`, `shims-*.d.ts`, `types/*` Vue 2 decls, `__examples__/`, old `__tests__/`).
- Modify: `.github/workflows/ci.yml`
- Modify: `web/.gitignore` (ensure `dist`, `node_modules` ignored)

- [ ] **Step 1: Identify and delete leftover Vue 2 sources**

List what remains from the old app and remove it:
```bash
cd D:/raynorpat/covergates
git ls-files web/src | grep -vE '^web/src/(main\.ts|App\.vue|env\.d\.ts|lib/base\.ts|plugins/(vuetify|http)\.ts|router/index\.ts|stores/|types/index\.ts|layouts/|components/(AccountButton|RepoList|RepoListItem)\.vue|components/__tests__/|views/(Home|Login|Dashboard|User|RepoPlaceholder)\.vue|views/__tests__/|test/setup\.ts)'
```
`git rm` every path that command prints (these are the old Vue 2 files). Then also remove stale root configs if still present: `git rm web/.eslintrc.js web/.prettierrc.js web/tsconfig.json` only if they are the OLD versions — keep the new `web/tsconfig.json` created in Task 1 (do not delete it). Re-create `web/.eslintrc.cjs` minimal:

```cjs
module.exports = {
  root: true,
  extends: ['plugin:vue/vue3-recommended', '@vue/eslint-config-typescript'],
  parserOptions: { ecmaVersion: 'latest' }
}
```

- [ ] **Step 2: Ensure `web/.gitignore` ignores build output**

Confirm `web/.gitignore` contains `node_modules` and `/dist` (the old one has `/dist` and `node_modules`). Add `web_gen.go` already covered. No change if present.

- [ ] **Step 3: Update `.github/workflows/ci.yml` FRONTEND job**

The `FRONTEND` matrix branch runs `npm install`, `npm run lint`, `npm run test:unit`. These now invoke Vite/Vitest via the new `package.json` scripts — no workflow YAML change needed for those steps. Remove the **Jest Report** coverage-upload step (it uses the legacy upload action and `./web/coverage/lcov.info`, which Vitest does not produce by default). Delete this block from `.github/workflows/ci.yml`:
```yaml
      - name: Jest Report
        if: env.BUILD_TYPE == 'FRONTEND'
        uses: covergates/github-actions@v1
        with:
          report-id: "bsi5dvi23akg00a0tgl0"
          report-type: "lcov"
          report-file: "./web/coverage/lcov.info"
          pull-request: "true"
```
(Frontend coverage reporting via the coveralls reporter is restored in B3.)

- [ ] **Step 4: Full frontend verification**

```bash
cd web
npm install
npm run test:unit
npm run build
```
Expected: all Vitest tests pass; `vite build` succeeds; `dist/index.html` contains `{{.}}/assets/...` and `window.VUE_BASE = '{{.}}'`.

- [ ] **Step 5: Backend still builds**

Run (repo root): `go build ./...`
Expected: success.

- [ ] **Step 6: Manual smoke test (optional, recommended)**

```bash
# terminal 1: backend (sqlite)
go run ./cmd/server
# terminal 2: frontend dev (proxies /api and /login to :8080)
cd web && npm run dev
# open http://localhost:8081 — landing page; /login shows OAuth buttons;
# after login, /repos lists repos, Activate works, Show token reveals the token.
```

- [ ] **Step 7: Commit**

```bash
cd .. && git add -A
git commit -m "web: remove vue 2 app and update frontend ci"
```
(Verify `git status` shows no `web/web_gen.go`, `web/dist`, or `web/node_modules` staged.)

---

## Self-Review notes (spec coverage)

- **B1.1 tooling** → Task 1 (Vite/TS/scaffold/base mechanism), Task 12 (remove old, CI).
- **B1.2 shell & theme** → Task 2 (theme), Task 8 (layout/AccountButton).
- **B1.3 auth & user bootstrap** → Task 6 (user store), Task 8 (bootstrap in main.ts), Task 9 (login), Task 5 (guard).
- **B1.4 dashboard (activate + token)** → Task 7 (repository store), Task 11 (Dashboard/RepoList/RepoListItem, token via GET/PATCH /token).
- **B1.5 routing** → Task 5.
- **B1.6 replace old app** → Tasks 1 & 12.
- **B1.7 testing** → store tests (6, 7), component tests (8, 11), CI (12).
- Backend `/assets` serving (needed by the migration) → Task 4.

Deferred per spec: builds/detail/source (B2); branch/PR/jobs, full settings, SCM-binding management, upload guide, frontend coverage reporting (B3).
