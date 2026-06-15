# Sub-project B1 — Vue 3 / Vuetify 3 Frontend Foundation + Dashboard

**Date:** 2026-06-14
**Status:** Approved design, ready for implementation planning
**Part of:** "Make covergates work like coveralls.io" — Sub-project B (UI), chunk **B1 of B1/B2/B3**

## Background

Sub-project A replaced covergates' coverage ingestion with a coveralls-compatible
Build/Job model and upload (`POST /api/v1/jobs`, `POST /webhook`), plus build read
endpoints (`GET /api/v1/repos/:scm/:namespace/:name/builds[/:number]`) and a secret
per-repo upload `Token`. The legacy `Report` read endpoints still exist but receive no
new data.

Sub-project B redesigns the frontend to look and work like coveralls.io on top of the
Build model. Per decomposition, B is delivered as three stacked sub-projects, each its
own spec → plan → build, in order:

| | Delivers |
|---|---|
| **B1** (this doc) | New Vue 3 stack + app shell + auth/user bootstrap + repo dashboard (activate + upload token) |
| B2 | Builds list, build detail, per-line source view |
| B3 | Branch & PR views, parallel-jobs/flags display, settings polish + coveralls upload guide |

### Current frontend (being replaced)

Vue 2.6 + Vuetify 2 + Vuex 3 + Vue Router 3 + TypeScript class components
(`vue-class-component` / `vue-property-decorator`), built with Vue CLI 4, tested with
Jest. The SPA is embedded into the Go backend by `togo` (`web/web.go` →
`go generate ./web` produces the gitignored `web/web_gen.go`) and served from the `web`
Go package. The backend injects the configurable base path by running `web/index.html`
through a Go template (`routers/web/static.go` `HandleIndex`, executed with
`config.Server.Base`); the current Vue CLI build emits assets under
`publicPath: '{{.}}'` so the template substitution resolves asset URLs.

## Decisions (from brainstorming)

1. **Visual approach:** match coveralls' workflow/page structure, but keep Vuetify
   (Material) styling — not a pixel-perfect reproduction of coveralls' bespoke CSS.
2. **Framework:** upgrade to **Vue 3 / Vuetify 3**.
3. **Stack:** **Vite** build, **Pinia** state, **Composition API with `<script setup>`** +
   TypeScript, **Vue Router 4**, **Vuetify 3**. Tests with **Vitest** + `@vue/test-utils` v2.
4. **Delivery:** sequence B1 → B2 → B3 (full scope, staged). This spec is B1.
5. Because the data model changed (`report` → `build`), the frontend is a clean rewrite —
   no migrate-then-redesign double pass.

## Architecture

The Vue 2 app under `web/src` is replaced by a Vue 3 app of the same responsibility:
an SPA served by the Go backend via togo. B1 establishes the stack and ships the app
shell, authentication/user bootstrap, and the repository dashboard. The deep `/report/*`
pages are added in B2/B3; B1 leaves a placeholder repo page.

### Directory / file structure (`web/`)

- `vite.config.ts` — Vite config: Vue + Vuetify plugins, `base` for togo serving, dev
  proxy for `/api` and `/login`, build `outDir` = `dist` (togo input unchanged).
- `index.html` — at `web/` root (Vite convention), containing the Go-template base token
  so the backend's `HandleIndex` substitution keeps working.
- `package.json` — Vue 3 / Vuetify 3 / Pinia / Router 4 / Vite / Vitest deps; scripts
  `dev`, `build`, `test:unit` (vitest), `lint`.
- `tsconfig.json` / `env.d.ts` — Vue 3 + Vite TS config and SFC shims.
- `src/main.ts` — create app, install Vuetify, Pinia, Router; bootstrap user; mount.
- `src/App.vue` — root, renders the layout + `<router-view>`.
- `src/plugins/vuetify.ts` — Vuetify 3 instance + theme (port current palette:
  primary cyan-darken-3, secondary grey-darken-4, accent blue-grey-darken-1, error
  red-darken-4), MDI icon set.
- `src/plugins/http.ts` — Axios instance (base URL from runtime base, shared error helper).
- `src/router/index.ts` — Router 4 routes + guards.
- `src/stores/user.ts` — Pinia user store.
- `src/stores/repository.ts` — Pinia repository store.
- `src/layouts/DefaultLayout.vue` — app bar + footer shell.
- `src/components/AccountButton.vue` — account menu.
- `src/views/Home.vue` — landing page.
- `src/views/Login.vue` — OAuth login route.
- `src/views/Dashboard.vue` — repo list/dashboard.
- `src/views/User.vue` — account / SCM bindings.
- `src/views/RepoPlaceholder.vue` — stub for `/report/:scm/:namespace/:name` until B2.
- `src/components/RepoList.vue` / `RepoListItem.vue` — repo list + activate/token row.
- `src/types/*.ts` — `Repository`, `User`, `Token` TS types (Build types added in B2).
- `src/assets/` — keep `logo`, `styles/variables.scss` (coverage colors, used in B2).
- Reusable-later (kept, wired in B2): a highlight.js helper for source rendering.

Each Pinia store owns one domain (user, repository) with a small, typed action surface.
Views are thin; data access goes through stores; HTTP goes through the shared Axios
instance. Components stay focused (list vs item vs account menu).

### B1.1 Tooling & stack

- Vite + `@vitejs/plugin-vue` + `vite-plugin-vuetify` (auto-import/treeshake Vuetify 3).
- `base`: production build uses the Go-template token (current behavior: `'{{.}}'`) so
  `HandleIndex` substitutes `config.Server.Base`. If Vite rejects the literal token, fall
  back to relative base (`base: './'`) and adjust `index.html`/router accordingly; the
  plan selects and verifies one mechanism. Dev uses `base: '/'`.
- Dev server: `server.proxy` routes `/api` and `/login` to the backend
  (`http://localhost:8080` by default, overridable via env), replacing `VUE_APP_PROXY`.
- Router 4 in history mode; base path from `import.meta.env.BASE_URL`.
- Remove Vue-2-only deps: `vue-class-component`, `vue-property-decorator`,
  `vue2-perfect-scrollbar`, `vue-countup`, `@vue/cli-*`, `jest` and its preset.

### B1.2 App shell & theme

`DefaultLayout.vue`: Vuetify 3 `v-app` → `v-app-bar` (logo avatar, "Repositories"
text button → `/repos`, `AccountButton` at right) → `v-main` `<router-view>` → footer
(copyright + Docs/GitHub links). Native scrolling (drop perfect-scrollbar). Responsive:
app bar collapses sensibly on mobile. Theme ported in `plugins/vuetify.ts`.

### B1.3 Auth & user bootstrap

- `stores/user.ts`: state `current: User | null`; action `fetch()` →
  `GET /api/v1/user`; getter `isAuthenticated`. Called once in `main.ts` before mount
  (or in the layout) so guards can read auth state.
- `views/Login.vue` at route `/login`: three OAuth buttons with `href` to
  `${base}/login/:scm` (github/gitea/gitlab); honors `?redirect=`.
- `AccountButton.vue`: avatar menu → "Settings" (`/user`), and "Logout" (link to
  `${base}/logoff`) / "Login" (`/login`) depending on auth.
- Router guard: routes flagged `requiresAuth` redirect unauthenticated users to
  `/login?redirect=<path>`.

### B1.4 Repo dashboard

- `stores/repository.ts`: state `list: Repository[]`; actions `fetchList()`
  (`GET /api/v1/user/repos`), `synchronize()` (`PATCH /api/v1/user/repos` then refetch).
- `views/Dashboard.vue` at `/repos` (requiresAuth): search field (client-side filter by
  name/namespace), **Sync** button, and `RepoList`. Activated repos (have a `ReportID`)
  sort first.
- `RepoListItem.vue`: shows SCM icon + `namespace/name` + URL.
  - Not activated → **Activate** button: `GET /api/v1/repos/:scm/:ns/:name`; on 404
    `POST /api/v1/repos`; then `PATCH /api/v1/repos/:scm/:ns/:name/report`. On success the
    row becomes activated.
  - Activated → links to `/report/:scm/:ns/:name`, and exposes the **upload token**:
    fetch it on demand via `GET /api/v1/repos/:scm/:ns/:name/token` → `{ "token": "..." }`
    and **rotate** via `PATCH /api/v1/repos/:scm/:ns/:name/token` → `{ "token": "..." }`,
    with copy-to-clipboard. Both endpoints are authenticated and SCM-access-checked.
    The secret token is `json:"-"` and is **never** included in any repo payload
    (`/user/repos`, `/repos/:scm/:ns/:name`) — it is only ever returned by these two
    token endpoints. (These backend endpoints already exist; B1 is frontend-only here.)

### B1.5 Routing

Routes: `/` → `Home`; `/login` → `Login`; `/repos` → `Dashboard` (requiresAuth);
`/user` → `User` (requiresAuth); `/report/:scm/:namespace/:name` → `RepoPlaceholder`
(a "coming soon / activated" stub replaced in B2); catch-all → `/`.

### B1.6 Replace the old app

Delete the Vue 2 sources and config; add the Vue 3 app in `web/`. Keep `web/web.go`
(togo generate directives) and the togo serving contract unchanged (still emits `dist/`).
Preserve `web/public` assets as needed (favicon, logo) under Vite's `public/`. Keep
`src/assets/styles/variables.scss` coverage colors and a highlight helper for B2.

### B1.7 Testing

Vitest + `@vue/test-utils` v2 (+ jsdom). Cover:
- `stores/user.ts`: `fetch()` sets `current`; `isAuthenticated`.
- `stores/repository.ts`: `fetchList()` populates `list`; `synchronize()` calls PATCH then
  refetches.
- `Dashboard.vue`: search filters the list; renders `RepoList`.
- `RepoListItem.vue`: activate flow (mocked Axios: 404 → POST → PATCH) and token rotate.
- `AccountButton.vue`: shows Login vs Logout based on auth state.

CI (`.github/workflows/ci.yml`) `FRONTEND` job: `npm run test:unit` now runs Vitest;
`npm run build` runs Vite. The Jest-based `Jest Report` coverage upload step is updated
or removed (it currently uses the legacy upload action — full CI coverage reporting is
addressed in B3; B1 only ensures the frontend job builds and tests pass).

## Data flow

```
app start → user store fetch() → GET /api/v1/user
                                      │ 401 → guard redirects to /login
/repos → repository store fetchList() → GET /api/v1/user/repos → RepoList
  activate → GET /repos/:scm/:ns/:name → (404) POST /repos → PATCH .../report
  token    → GET /repos/:scm/:ns/:name/token (reveal) ; PATCH .../token (rotate)
  Sync     → PATCH /api/v1/user/repos → refetch list
```

## Error handling

- Failed `GET /api/v1/user` (401/network): treat as unauthenticated; guards send
  protected routes to `/login`. Public routes (`/`, `/login`) still render.
- Activate/sync/token errors: surface a Vuetify snackbar/inline message via the shared
  Axios error helper; leave UI state unchanged.
- Unknown routes: redirect to `/`.

## Out of scope (B2/B3)

- Builds list, build detail, per-line source view (B2).
- Branch views, PR-comparison views, parallel-jobs/flags UI (B3).
- Full repo settings (protected/webhook/auto-merge/filters), badge/card embed UI, and
  the coveralls upload guide (B3).
- Per-repo coverage badges on the dashboard (needs a per-repo latest-build summary;
  revisit when that data is available).
- Removing the legacy backend `Report` read endpoints (handled once the UI no longer
  references them).
