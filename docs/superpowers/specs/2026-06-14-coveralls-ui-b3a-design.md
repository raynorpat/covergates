# Sub-project B3a — Repo Admin & Onboarding UI

**Date:** 2026-06-14
**Status:** Approved design, ready for implementation planning
**Part of:** "Make covergates work like coveralls.io" — Sub-project B (UI), chunk **B3a** (B3 = B3a + B3b)

## Background

A (build model + upload), B1 (Vue 3 foundation + dashboard) and B2 (builds list, build
detail, source view) are merged into `shuenhoy/master`. B3 finishes the coveralls-style UI
and is split into two chunks:

- **B3a** (this doc): repo settings page, badge/card embed + coveralls upload guide, SCM
  account binding on the user page, settings-endpoint authorization hardening, and frontend
  CI coverage reporting.
- **B3b**: folder/tree file view, branch & PR-comparison views, parallel-jobs detail, and
  flag (`flag_name`) support (backend + UI).

### Relevant existing API

- `GET /api/v1/repos/:scm/:namespace/:name/setting` → `core.RepoSetting`.
- `POST .../setting` → update settings (under `checkLogin`; `HandleUpdateSetting` takes `SCMService`).
- `POST .../hook/create` → install webhook (`HandleHookCreate`, uses `WithRepo`).
- `GET`/`PATCH .../token` → upload token (authed + `canAccessRepo`, from A).
- `GET /api/v1/repos/:scm/:namespace/:name` → repo (has `ReportID`, `Private`, `URL`,
  `Branch`; `Token` is `json:"-"`).
- `GET /api/v1/user/scm` → `Record<scm, boolean>` (which providers the user has linked).
- Badge/card: `GET /api/v1/reports/:reportID/badge`, `.../card` (public images).
- `core.RepoSetting` JSON: `{ filters: string[], mergePR: boolean, updateAction: string, protected: boolean }`.
- `canAccessRepo(c, scmService, repo)` helper exists in `routers/api/repo` (verifies the
  context user can `client.Repositories().Find` the repo).

## Decisions (from brainstorming)

1. Two chunks; this is B3a.
2. **Harden settings authorization** in B3a: settings/hook endpoints must require SCM
   repo access (today any logged-in user passes).
3. **Include CI frontend coverage reporting** (Vitest lcov → existing covergates action,
   same hosted target as the backend Go coverage step).
4. Flags + folder tree + branch/PR views → B3b.
5. Vuetify look; Vue 3 `<script setup>` + Pinia; matches B1/B2 patterns.

## Architecture

Mostly frontend on existing endpoints, plus a small backend authorization hardening.

### File structure

Backend (modify): `routers/api/repo/repo.go` (access checks in `HandleGetSetting`,
`HandleUpdateSetting`, `HandleHookCreate`), `routers/api/api.go` (pass `SCMService` to the
settings/hook routes), `routers/api/repo/repo_test.go` (access-check tests).

Frontend (create):
- `web/src/types/setting.ts` — `RepoSetting` type.
- `web/src/lib/useRepoToken.ts` — composable: reveal/rotate token (`GET`/`PATCH .../token`).
- `web/src/views/SettingsView.vue` — settings page shell.
- `web/src/components/SettingsGeneral.vue` — protected/mergePR/filters form.
- `web/src/components/WebhookButton.vue` — webhook activation.
- `web/src/components/EmbedCard.vue` — badge/card URLs + Markdown snippets + copy.
- `web/src/components/UploadGuide.vue` — coveralls reporter setup instructions.
- `web/src/components/AccountBindings.vue` — SCM link/unlink list (used by `User.vue`).
- tests under `web/src/**/__tests__/`.

Frontend (modify):
- `web/src/stores/repository.ts` — add `fetchSetting`/`updateSetting`.
- `web/src/stores/user.ts` — add `fetchScm` (provider→bool map).
- `web/src/views/User.vue` — embed `AccountBindings`.
- `web/src/views/BuildsView.vue` — add a "Settings" gear link (to the settings route)
  shown once the repo is loaded.
- `web/src/router/index.ts` — add the settings route (`requiresAuth`).
- `web/package.json`, `web/vite.config.ts` — Vitest coverage (`@vitest/coverage-v8`, lcov).
- `.github/workflows/ci.yml` — frontend coverage + upload step.

### B3a.1 Settings authorization hardening

Add `canAccessRepo` gating:
- `HandleGetSetting(store, scmService)`: resolve repo by path, `canAccessRepo` → else 403,
  then return `store.Setting(repo)`.
- `HandleUpdateSetting`: already has `SCMService`; add the `canAccessRepo` check before
  writing (in addition to its existing `WithRepo`).
- `HandleHookCreate(hookService, scmService, repoStore)`: add the `canAccessRepo` check
  (uses the `WithRepo` repo from context) → else 403.
- `api.go`: update the three route registrations to pass `r.SCMService` where now required.
Return `403 "forbidden"` on failure, consistent with the token endpoints.

### B3a.2 Settings page

Route `/report/:scm/:namespace/:name/settings` (requiresAuth). `SettingsView.vue` loads the
repo (`GET /repos/:scm/:ns/:name`) and `GET /setting`; on 401/403 shows a "you don't have
access" notice. Sections (Vuetify cards):
- **General** (`SettingsGeneral.vue`): `v-switch` for `protected` and `mergePR`; a textarea
  or chip-input for `filters` (one regex per line). Save → `updateSetting` (`POST /setting`)
  with a success snackbar.
- **Webhook** (`WebhookButton.vue`): button → `POST /hook/create`; shows success/failure.
- **Upload token**: reveal + rotate via `useRepoToken` (`GET`/`PATCH .../token`), copy button.
- **Embed** (`EmbedCard.vue`): badge image preview + Markdown
  `[![Coverage]({base}/api/v1/reports/{reportID}/badge)]({base}/report/{scm}/{ns}/{name})`,
  and the card image/snippet; copy buttons.
- **Upload guide** (`UploadGuide.vue`): instructions to point a coveralls reporter at this
  server — set `COVERALLS_ENDPOINT={server}/api/v1` and `COVERALLS_REPO_TOKEN=<token>` (or
  the reporter's flag), with a GitHub Actions snippet. Replaces the removed CLI guide.

### B3a.3 SCM account binding

`AccountBindings.vue` (in `User.vue`): `user.fetchScm()` → `GET /user/scm` →
`{ github: bool, gitea: bool, gitlab: bool }`. For each configured provider, show linked ✓
or a **Link** button (`href` → `{base}/login/:scm?bind`). Unlinking is out of scope (no
endpoint); show only link + status.

### B3a.4 Data layer

- `stores/repository.ts`: `setting: RepoSetting | null`; `fetchSetting(repoPath)` →
  `GET {repoPath}/setting`; `updateSetting(repoPath, setting)` → `POST {repoPath}/setting`.
- `stores/user.ts`: `scm: Record<string, boolean>`; `fetchScm()` → `GET /api/v1/user/scm`.
- `types/setting.ts`: `RepoSetting { filters: string[]; mergePR: boolean; updateAction: string; protected: boolean }`.

### B3a.5 CI coverage reporting

- Add `@vitest/coverage-v8` dev dep; configure `vite.config.ts` `test.coverage`
  (`provider: 'v8'`, `reporter: ['text','lcov']`, `include: ['src/**']`).
- `package.json`: `test:unit` stays; add `test:coverage` = `vitest run --coverage`.
- `.github/workflows/ci.yml` FRONTEND: run `npm run test:coverage` then upload
  `web/coverage/lcov.info` via `covergates/github-actions@v1` (mirroring the backend Go
  coverage step; same hosted covergates.com target).

### B3a.6 Testing

- Backend: `repo_test.go` — `HandleGetSetting`/`HandleUpdateSetting`/`HandleHookCreate`
  return 403 when `canAccessRepo` fails (mock SCM `Find` error) and proceed when it passes.
- Frontend: `SettingsGeneral` (renders setting, emits save with edited values), `EmbedCard`
  (correct badge/card URLs + markdown for a given reportID/base), `useRepoToken` (reveal +
  rotate call the right endpoints), `AccountBindings` (linked vs Link button), and a
  repository-store `fetchSetting`/`updateSetting` test.

## Data flow

```
/report/:scm/:ns/:name/settings (requiresAuth)
  → GET /repos/:scm/:ns/:name           (repo: reportID, private, url)
  → GET .../setting                      (403 → no-access notice)
  General save → POST .../setting
  Webhook      → POST .../hook/create
  Token        → GET/PATCH .../token
  Embed        → static badge/card URLs from reportID
/user
  → GET /api/v1/user/scm                 (AccountBindings)
```

## Error handling

- Settings 401/403 → inline "you don't have access to this repository's settings" notice.
- Save/webhook/token failures → snackbar with the error message; state unchanged.
- `GET /user/scm` failure → bindings section shows nothing actionable (no crash).

## Out of scope (B3b / later)

- Folder/tree file view, branch & PR-comparison views, parallel-jobs detail, flags
  (`flag_name` backend + UI) — all B3b.
- Unlinking SCM accounts (no backend endpoint).
- Removing the legacy `Report`/badge `ReportID` slug (the embed UI still uses `ReportID`).
