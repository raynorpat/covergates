# Coveralls UI B3a — Repo Admin & Onboarding Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the repo settings page (general settings, webhook, token, badge/card embed, coveralls upload guide), SCM account binding on the user page, settings-endpoint authorization hardening, and frontend CI coverage reporting.

**Architecture:** Mostly Vue 3 `<script setup>` views/components + Pinia store additions on existing endpoints, plus a small Go authorization hardening (gate settings reads with repo access and webhook creation with admin/creator, via helpers in `routers/api/repo`).

**Tech Stack:** Vue 3.5, Vuetify 3, Pinia, Vite, Vitest, Axios; Go 1.14 backend. Node 24/npm available.

**Conventions (CLAUDE.md):** short subsystem-prefixed commits (`web:`/`repo:`), no metadata. Frontend cmds from `web/`; Go from repo root. Backend builds with explicit paths: `go build ./core/... ./models/... ./modules/... ./routers/...` (NOT `go build ./...` — walks web/node_modules). Never commit `web/dist`, `web/node_modules`, `web/web_gen.go`. Frontend embed URLs use `window.location.origin + basePath()`.

---

## File structure

Backend (modify): `routers/api/repo/repo.go` (add `canAdminRepo`; refactor `HandleUpdateSetting`; gate `HandleGetSetting`), `routers/api/repo/hook.go` (gate `HandleHookCreate`), `routers/api/api.go` (route wiring), `routers/api/repo/repo_test.go` (tests).

Frontend (create): `web/src/types/setting.ts`, `web/src/lib/useRepoToken.ts`, `web/src/views/SettingsView.vue`, `web/src/components/{SettingsGeneral,WebhookButton,EmbedCard,UploadGuide,AccountBindings}.vue`, tests.
Frontend (modify): `web/src/stores/repository.ts`, `web/src/stores/user.ts`, `web/src/views/User.vue`, `web/src/views/BuildsView.vue`, `web/src/router/index.ts`, `web/package.json`, `web/vite.config.ts`, `.github/workflows/ci.yml`.

---

## Task 1: Backend — settings/webhook authorization hardening

**Files:**
- Modify: `routers/api/repo/repo.go`, `routers/api/repo/hook.go`, `routers/api/api.go`
- Test: `routers/api/repo/repo_test.go`

- [ ] **Step 1: Add `canAdminRepo` helper and use it in `HandleUpdateSetting`**

In `routers/api/repo/repo.go`, add after `canAccessRepo`:
```go
// canAdminRepo reports whether the current user may administer the repository
// (SCM admin, or the activator/creator). Used for settings writes and webhook setup.
func canAdminRepo(c *gin.Context, service core.SCMService, store core.RepoStore, repo *core.Repo) bool {
	user, ok := request.UserFrom(c)
	if !ok {
		return false
	}
	client, err := service.Client(repo.SCM)
	if err != nil {
		return false
	}
	if client.Repositories().IsAdmin(c.Request.Context(), user, repo.FullName()) {
		return true
	}
	creator, err := store.Creator(repo)
	return err == nil && creator.Login == user.Login
}
```
Then refactor `HandleUpdateSetting`'s inline auth (the `user, ok := ...`, `creator, err := ...`, `client, err := ...`, `if !client...IsAdmin... && user.Login != creator.Login` block) to:
```go
		repo := c.MustGet(keyRepo).(*core.Repo)
		setting := &core.RepoSetting{}
		if !canAdminRepo(c, service, store, repo) {
			c.JSON(403, setting)
			return
		}
		if err := c.BindJSON(setting); err != nil {
			c.JSON(400, setting)
			return
		}
		if err := store.UpdateSetting(repo, setting); err != nil {
			c.JSON(500, setting)
			return
		}
		c.JSON(200, setting)
```
(Removes the now-unused `ctx`, `user`, `creator`, `client` locals.)

- [ ] **Step 2: Gate `HandleGetSetting` with repo access**

Change the signature to `func HandleGetSetting(store core.RepoStore, service core.SCMService) gin.HandlerFunc` and, after `store.Find(...)` succeeds (before `store.Setting`), add:
```go
		if !canAccessRepo(c, service, repo) {
			c.JSON(403, &core.RepoSetting{})
			return
		}
```

- [ ] **Step 3: Gate `HandleHookCreate` with admin**

In `routers/api/repo/hook.go`, change `func HandleHookCreate(service core.HookService) gin.HandlerFunc` to `func HandleHookCreate(service core.HookService, scmService core.SCMService, store core.RepoStore) gin.HandlerFunc`, and after `repo, _ := c.MustGet(keyRepo).(*core.Repo)` add:
```go
		if !canAdminRepo(c, scmService, store, repo) {
			c.String(403, "forbidden")
			return
		}
```
Add imports as needed (`core`, `request` already present? add if missing — confirm by building).

- [ ] **Step 4: Update route registration in `routers/api/api.go`**

In the authenticated repo group:
```go
			g.GET("/setting", repo.HandleGetSetting(r.RepoStore, r.SCMService))
			g.POST("/setting", repo.WithRepo(r.RepoStore), repo.HandleUpdateSetting(r.RepoStore, r.SCMService))
			...
			g.POST("/hook/create", repo.WithRepo(r.RepoStore), repo.HandleHookCreate(r.HookService, r.SCMService, r.RepoStore))
```

- [ ] **Step 5: Tests in `routers/api/repo/repo_test.go`**

Add handler tests. Use the existing mock pattern (`mock.NewMockRepoStore`, `mock.NewMockSCMService`, `mock.NewMockClient`, `mock.NewMockGitRepoService`). Set a context user via a middleware that calls `request.WithUser`. Example for `HandleGetSetting`:
```go
func TestHandleGetSettingForbidden(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	store := mock.NewMockRepoStore(ctrl)
	scm := mock.NewMockSCMService(ctrl)
	client := mock.NewMockClient(ctrl)
	repos := mock.NewMockGitRepoService(ctrl)

	store.EXPECT().Find(gomock.Any()).Return(&core.Repo{Name: "r", NameSpace: "o", SCM: core.Github}, nil)
	scm.EXPECT().Client(core.Github).Return(client, nil)
	client.EXPECT().Repositories().Return(repos)
	repos.EXPECT().Find(gomock.Any(), gomock.Any(), "o/r").Return(nil, errors.New("no access"))

	r := gin.New()
	r.Use(func(c *gin.Context) { request.WithUser(c, &core.User{Login: "u"}) })
	r.GET("/repos/:scm/:namespace/:name/setting", HandleGetSetting(store, scm))
	req := httptest.NewRequest("GET", "/repos/github/o/r/setting", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 403 {
		t.Fatalf("status = %d, want 403", w.Code)
	}
}
```
Add a passing-access variant (`repos.Find` returns a repo, `store.Setting` returns a setting → 200) and a `HandleHookCreate` forbidden test (`IsAdmin` false + `Creator` mismatch → 403). Confirm the exact `SCMService`/`Client`/`GitRepoService` mock method names by reading `mock/scm_mock.go` (e.g. `IsAdmin`, `Find`).

- [ ] **Step 6: Verify**

Run: `go build ./core/... ./modules/... ./routers/...` → success.
Run: `go test ./routers/api/repo/ -count=1` → PASS.

- [ ] **Step 7: Commit**

```bash
git add routers/api/repo/repo.go routers/api/repo/hook.go routers/api/api.go routers/api/repo/repo_test.go
git commit -m "repo: gate settings reads and webhook creation by repo access"
```

---

## Task 2: Setting type + repository store methods

**Files:**
- Create: `web/src/types/setting.ts`
- Modify: `web/src/stores/repository.ts`
- Test: `web/src/stores/__tests__/repository.spec.ts`

- [ ] **Step 1: Create `web/src/types/setting.ts`**

```ts
export interface RepoSetting {
  filters: string[]
  mergePR: boolean
  updateAction: string
  protected: boolean
}
```

- [ ] **Step 2: Add store state/actions to `web/src/stores/repository.ts`**

Add `import type { RepoSetting } from '@/types/setting'`. Inside the store add a `setting` ref and two actions, and include them in the returned object:
```ts
  const setting = ref<RepoSetting | null>(null)

  async function fetchSetting(repoPath: string) {
    const { data } = await http.get<RepoSetting>(`${repoPath}/setting`)
    setting.value = data
  }

  async function updateSetting(repoPath: string, value: RepoSetting) {
    const { data } = await http.post<RepoSetting>(`${repoPath}/setting`, value)
    setting.value = data
  }
```
Return: add `setting, fetchSetting, updateSetting` to the existing `return { ... }`.

- [ ] **Step 3: Extend the store test**

Append to `web/src/stores/__tests__/repository.spec.ts` (its mock must include `post`; update the `vi.mock('@/plugins/http', ...)` to `{ default: { get: vi.fn(), patch: vi.fn(), post: vi.fn() }, ... }`):
```ts
  it('fetchSetting and updateSetting hit the setting endpoint', async () => {
    const s = { filters: ['a'], mergePR: true, updateAction: 'merge', protected: false }
    ;(http.get as any).mockResolvedValue({ data: s })
    ;(http.post as any).mockResolvedValue({ data: s })
    const store = useRepositoryStore()
    await store.fetchSetting('/api/v1/repos/github/o/r')
    expect(store.setting).toEqual(s)
    await store.updateSetting('/api/v1/repos/github/o/r', s)
    expect(http.post).toHaveBeenCalledWith('/api/v1/repos/github/o/r/setting', s)
  })
```
Ensure the file's `beforeEach` clears mocks (`vi.clearAllMocks()`); add it if absent.

- [ ] **Step 4: Verify**

Run: `cd web && npx vitest run src/stores/__tests__/repository.spec.ts` → PASS.

- [ ] **Step 5: Commit**

```bash
cd .. && git add web/src/types/setting.ts web/src/stores/repository.ts web/src/stores/__tests__/repository.spec.ts
git commit -m "web: repository setting store"
```

---

## Task 3: User store — SCM bindings

**Files:**
- Modify: `web/src/stores/user.ts`
- Test: `web/src/stores/__tests__/user.spec.ts`

- [ ] **Step 1: Add `scm` state + `fetchScm` to `web/src/stores/user.ts`**

Add a `scm` ref and action, include in return:
```ts
  const scm = ref<Record<string, boolean>>({})

  async function fetchScm() {
    try {
      const { data } = await http.get<Record<string, boolean>>('/api/v1/user/scm')
      scm.value = data ?? {}
    } catch {
      scm.value = {}
    }
  }
```
Return: add `scm, fetchScm`.

- [ ] **Step 2: Extend `web/src/stores/__tests__/user.spec.ts`**

The file's `vi.mock('@/plugins/http')` already mocks `get`. Add:
```ts
  it('fetchScm populates provider map', async () => {
    ;(http.get as any).mockResolvedValue({ data: { github: true, gitea: false } })
    const store = useUserStore()
    await store.fetchScm()
    expect(store.scm.github).toBe(true)
    expect(store.scm.gitea).toBe(false)
  })
```

- [ ] **Step 3: Verify**

Run: `cd web && npx vitest run src/stores/__tests__/user.spec.ts` → PASS.

- [ ] **Step 4: Commit**

```bash
cd .. && git add web/src/stores/user.ts web/src/stores/__tests__/user.spec.ts
git commit -m "web: user scm bindings store"
```

---

## Task 4: useRepoToken composable

**Files:**
- Create: `web/src/lib/useRepoToken.ts`, `web/src/lib/__tests__/useRepoToken.spec.ts`

- [ ] **Step 1: Write the failing test `web/src/lib/__tests__/useRepoToken.spec.ts`**

```ts
import { describe, it, expect, vi, beforeEach } from 'vitest'
import http from '@/plugins/http'
import { useRepoToken } from '@/lib/useRepoToken'

vi.mock('@/plugins/http', () => ({ default: { get: vi.fn(), patch: vi.fn() }, errorMessage: (e: unknown) => String(e) }))

const repoPath = '/api/v1/repos/github/o/r'

describe('useRepoToken', () => {
  beforeEach(() => vi.clearAllMocks())

  it('reveal fetches the token', async () => {
    ;(http.get as any).mockResolvedValue({ data: { token: 'sec' } })
    const t = useRepoToken(repoPath)
    await t.reveal()
    expect(http.get).toHaveBeenCalledWith(`${repoPath}/token`)
    expect(t.token.value).toBe('sec')
    expect(t.revealed.value).toBe(true)
  })

  it('rotate patches the token', async () => {
    ;(http.patch as any).mockResolvedValue({ data: { token: 'new' } })
    const t = useRepoToken(repoPath)
    await t.rotate()
    expect(http.patch).toHaveBeenCalledWith(`${repoPath}/token`)
    expect(t.token.value).toBe('new')
  })
})
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd web && npx vitest run src/lib/__tests__/useRepoToken.spec.ts` → FAIL.

- [ ] **Step 3: Implement `web/src/lib/useRepoToken.ts`**

```ts
import { ref } from 'vue'
import http, { errorMessage } from '@/plugins/http'

export function useRepoToken(repoPath: string) {
  const token = ref('')
  const revealed = ref(false)
  const busy = ref(false)
  const error = ref('')

  async function reveal() {
    busy.value = true
    error.value = ''
    try {
      const { data } = await http.get<{ token: string }>(`${repoPath}/token`)
      token.value = data.token
      revealed.value = true
    } catch (e) {
      error.value = errorMessage(e)
    } finally {
      busy.value = false
    }
  }

  async function rotate() {
    busy.value = true
    error.value = ''
    try {
      const { data } = await http.patch<{ token: string }>(`${repoPath}/token`)
      token.value = data.token
      revealed.value = true
    } catch (e) {
      error.value = errorMessage(e)
    } finally {
      busy.value = false
    }
  }

  return { token, revealed, busy, error, reveal, rotate }
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `cd web && npx vitest run src/lib/__tests__/useRepoToken.spec.ts` → PASS.

- [ ] **Step 5: Commit**

```bash
cd .. && git add web/src/lib/useRepoToken.ts web/src/lib/__tests__/useRepoToken.spec.ts
git commit -m "web: useRepoToken composable"
```

---

## Task 5: SettingsGeneral component

**Files:**
- Create: `web/src/components/SettingsGeneral.vue`, `web/src/components/__tests__/SettingsGeneral.spec.ts`

- [ ] **Step 1: Create `web/src/components/SettingsGeneral.vue`**

```vue
<script setup lang="ts">
import { ref, watch } from 'vue'
import type { RepoSetting } from '@/types/setting'

const props = defineProps<{ modelValue: RepoSetting; busy?: boolean }>()
const emit = defineEmits<{ (e: 'save', value: RepoSetting): void }>()

const isProtected = ref(props.modelValue.protected)
const mergePR = ref(props.modelValue.mergePR)
const filtersText = ref((props.modelValue.filters ?? []).join('\n'))

watch(() => props.modelValue, (v) => {
  isProtected.value = v.protected
  mergePR.value = v.mergePR
  filtersText.value = (v.filters ?? []).join('\n')
})

function save() {
  emit('save', {
    ...props.modelValue,
    protected: isProtected.value,
    mergePR: mergePR.value,
    filters: filtersText.value.split('\n').map((s) => s.trim()).filter(Boolean)
  })
}
</script>

<template>
  <v-card class="pa-4 mb-4">
    <div class="text-h6 mb-2">General</div>
    <v-switch v-model="isProtected" label="Protected (only authorized users may upload)" color="primary" hide-details />
    <v-switch v-model="mergePR" label="Auto-merge with previous coverage" color="primary" hide-details class="mb-2" />
    <v-textarea v-model="filtersText" label="File-name filters (one regex per line)" rows="3" auto-grow />
    <v-btn color="primary" :loading="busy" @click="save">Save</v-btn>
  </v-card>
</template>
```

- [ ] **Step 2: Write `web/src/components/__tests__/SettingsGeneral.spec.ts`**

```ts
import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import * as directives from 'vuetify/directives'
import SettingsGeneral from '@/components/SettingsGeneral.vue'

const vuetify = createVuetify({ components, directives })

describe('SettingsGeneral', () => {
  it('emits save with edited filters', async () => {
    const w = mount(SettingsGeneral, {
      props: { modelValue: { filters: ['x'], mergePR: false, updateAction: 'merge', protected: false } },
      global: { plugins: [vuetify] }
    })
    const textarea = w.find('textarea')
    await textarea.setValue('a\nb\n')
    const saveBtn = w.findAll('button').find((b) => b.text().includes('Save'))!
    await saveBtn.trigger('click')
    const ev = w.emitted('save')
    expect(ev).toBeTruthy()
    expect((ev![0][0] as any).filters).toEqual(['a', 'b'])
  })
})
```

- [ ] **Step 3: Run**

Run: `cd web && npx vitest run src/components/__tests__/SettingsGeneral.spec.ts` → PASS.

- [ ] **Step 4: Commit**

```bash
cd .. && git add web/src/components/SettingsGeneral.vue web/src/components/__tests__/SettingsGeneral.spec.ts
git commit -m "web: settings general form"
```

---

## Task 6: Webhook button, embed card, upload guide

**Files:**
- Create: `web/src/components/WebhookButton.vue`, `web/src/components/EmbedCard.vue`, `web/src/components/UploadGuide.vue`, `web/src/components/__tests__/EmbedCard.spec.ts`

- [ ] **Step 1: Create `web/src/components/WebhookButton.vue`**

```vue
<script setup lang="ts">
import { ref } from 'vue'
import http, { errorMessage } from '@/plugins/http'

const props = defineProps<{ repoPath: string }>()
const busy = ref(false)
const message = ref('')

async function create() {
  busy.value = true
  message.value = ''
  try {
    await http.post(`${props.repoPath}/hook/create`)
    message.value = 'Webhook installed.'
  } catch (e) {
    message.value = errorMessage(e)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <v-card class="pa-4 mb-4">
    <div class="text-h6 mb-2">Webhook</div>
    <p class="text-body-2 mb-3">Install a webhook so pull requests get coverage comments and status.</p>
    <v-btn color="primary" :loading="busy" @click="create">Install webhook</v-btn>
    <span v-if="message" class="ml-3 text-body-2">{{ message }}</span>
  </v-card>
</template>
```

- [ ] **Step 2: Create `web/src/components/EmbedCard.vue`**

```vue
<script setup lang="ts">
import { computed } from 'vue'
import { basePath } from '@/lib/base'

const props = defineProps<{ reportId: string; repoRoute: string }>()

const origin = computed(() => window.location.origin + basePath())
const badgeUrl = computed(() => `${origin.value}/api/v1/reports/${props.reportId}/badge`)
const cardUrl = computed(() => `${origin.value}/api/v1/reports/${props.reportId}/card`)
const reportUrl = computed(() => `${origin.value}${props.repoRoute}`)
const badgeMd = computed(() => `[![Coverage](${badgeUrl.value})](${reportUrl.value})`)
const cardMd = computed(() => `[![Coverage](${cardUrl.value})](${reportUrl.value})`)

function copy(text: string) {
  navigator.clipboard?.writeText(text)
}
</script>

<template>
  <v-card class="pa-4 mb-4">
    <div class="text-h6 mb-2">Embed</div>
    <img :src="badgeUrl" alt="coverage badge" class="mb-3" />
    <v-text-field :model-value="badgeMd" label="Badge markdown" readonly density="compact"
      append-inner-icon="mdi-content-copy" @click:append-inner="copy(badgeMd)" />
    <v-text-field :model-value="cardMd" label="Card markdown" readonly density="compact"
      append-inner-icon="mdi-content-copy" @click:append-inner="copy(cardMd)" />
  </v-card>
</template>
```

- [ ] **Step 3: Create `web/src/components/UploadGuide.vue`**

```vue
<script setup lang="ts">
import { computed } from 'vue'
import { basePath } from '@/lib/base'

const endpoint = computed(() => `${window.location.origin + basePath()}/api/v1`)
const snippet = computed(() =>
  `# CI: upload coverage with any coveralls reporter\n` +
  `export COVERALLS_ENDPOINT="${endpoint.value}"\n` +
  `export COVERALLS_REPO_TOKEN="<your repo token>"\n` +
  `coveralls report coverage.out   # or your language's coveralls uploader`
)
</script>

<template>
  <v-card class="pa-4 mb-4">
    <div class="text-h6 mb-2">Uploading coverage</div>
    <p class="text-body-2 mb-2">
      Point any coveralls-compatible reporter at this server. Set the endpoint to this
      instance and use the repository upload token above.
    </p>
    <v-sheet color="grey-lighten-4" class="pa-3" rounded>
      <pre style="white-space: pre-wrap; margin: 0">{{ snippet }}</pre>
    </v-sheet>
  </v-card>
</template>
```

- [ ] **Step 4: Write `web/src/components/__tests__/EmbedCard.spec.ts`**

```ts
import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import EmbedCard from '@/components/EmbedCard.vue'

const vuetify = createVuetify({ components })

describe('EmbedCard', () => {
  it('builds badge markdown from reportID', () => {
    const w = mount(EmbedCard, {
      props: { reportId: 'rid123', repoRoute: '/report/github/o/r' },
      global: { plugins: [vuetify] }
    })
    expect(w.html()).toContain('/api/v1/reports/rid123/badge')
    const inputs = w.findAll('input')
    const badge = inputs.find((i) => (i.element as HTMLInputElement).value.includes('/badge'))
    expect(badge).toBeTruthy()
    expect((badge!.element as HTMLInputElement).value).toContain('![Coverage]')
  })
})
```

- [ ] **Step 5: Run**

Run: `cd web && npx vitest run src/components/__tests__/EmbedCard.spec.ts` → PASS.

- [ ] **Step 6: Commit**

```bash
cd .. && git add web/src/components/WebhookButton.vue web/src/components/EmbedCard.vue web/src/components/UploadGuide.vue web/src/components/__tests__/EmbedCard.spec.ts
git commit -m "web: webhook, embed, and upload guide components"
```

---

## Task 7: SettingsView + route + repo gear link

**Files:**
- Create: `web/src/views/SettingsView.vue`
- Modify: `web/src/router/index.ts`, `web/src/views/BuildsView.vue`

- [ ] **Step 1: Create `web/src/views/SettingsView.vue`**

```vue
<script setup lang="ts">
import { onMounted, ref, computed } from 'vue'
import { useRoute } from 'vue-router'
import http, { errorMessage } from '@/plugins/http'
import { is401, isStatus, loginUrl } from '@/lib/auth'
import { useRepositoryStore } from '@/stores/repository'
import { useRepoToken } from '@/lib/useRepoToken'
import type { Repository } from '@/types'
import SettingsGeneral from '@/components/SettingsGeneral.vue'
import WebhookButton from '@/components/WebhookButton.vue'
import EmbedCard from '@/components/EmbedCard.vue'
import UploadGuide from '@/components/UploadGuide.vue'

const route = useRoute()
const store = useRepositoryStore()
const loading = ref(false)
const error = ref('')
const denied = ref(false)
const saving = ref(false)
const snackbar = ref('')
const repo = ref<Repository | null>(null)

const scm = computed(() => String(route.params.scm))
const namespace = computed(() => String(route.params.namespace))
const name = computed(() => String(route.params.name))
const repoPath = computed(() => `/api/v1/repos/${scm.value}/${namespace.value}/${name.value}`)
const repoRoute = computed(() => `/report/${scm.value}/${namespace.value}/${name.value}`)
const signInUrl = computed(() => loginUrl(route.fullPath))

const tokenCtl = useRepoToken(repoPath.value)

async function load() {
  loading.value = true
  error.value = ''
  denied.value = false
  try {
    const res = await http.get<Repository>(repoPath.value)
    repo.value = res.data
    await store.fetchSetting(repoPath.value)
  } catch (e) {
    if (is401(e)) error.value = ''
    if (is401(e) || isStatus(e, 403)) denied.value = true
    else error.value = errorMessage(e)
  } finally {
    loading.value = false
  }
}

async function save(value: any) {
  saving.value = true
  try {
    await store.updateSetting(repoPath.value, value)
    snackbar.value = 'Settings saved.'
  } catch (e) {
    snackbar.value = errorMessage(e)
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>

<template>
  <v-container class="py-6">
    <h2 class="text-h5 mb-4">{{ namespace }}/{{ name }} · Settings</h2>
    <v-progress-linear v-if="loading" indeterminate color="primary" class="mb-3" />
    <v-card v-else-if="denied" class="pa-6 text-center">
      <p class="mb-4">You don't have access to this repository's settings.</p>
      <v-btn color="primary" :href="signInUrl">Sign in</v-btn>
    </v-card>
    <v-alert v-else-if="error" type="warning" variant="tonal">{{ error }}</v-alert>
    <template v-else-if="store.setting && repo">
      <SettingsGeneral :model-value="store.setting" :busy="saving" @save="save" />
      <WebhookButton :repo-path="repoPath" />
      <v-card class="pa-4 mb-4">
        <div class="text-h6 mb-2">Upload token</div>
        <v-btn variant="text" :loading="tokenCtl.busy.value" @click="tokenCtl.revealed.value ? tokenCtl.rotate() : tokenCtl.reveal()">
          {{ tokenCtl.revealed.value ? 'Rotate token' : 'Show token' }}
        </v-btn>
        <v-text-field v-if="tokenCtl.revealed.value" :model-value="tokenCtl.token.value" label="Upload token" readonly density="compact" class="mt-2" />
      </v-card>
      <EmbedCard :report-id="repo.ReportID" :repo-route="repoRoute" />
      <UploadGuide />
    </template>
    <v-snackbar v-model="snackbar" :timeout="4000">{{ snackbar }}</v-snackbar>
  </v-container>
</template>
```

- [ ] **Step 2: Add the settings route in `web/src/router/index.ts`**

Add the import and route:
```ts
import SettingsView from '@/views/SettingsView.vue'
```
```ts
    { path: '/report/:scm/:namespace/:name/settings', name: 'settings', component: SettingsView, meta: { requiresAuth: true } },
```
(Place it among the `/report/...` routes, before the source catch-all route so it isn't swallowed; since the source route is `/builds/:number/source/:path(.*)*` it won't conflict, but keep settings adjacent to the repo route.)

- [ ] **Step 3: Add a gear link in `web/src/views/BuildsView.vue`**

In the `<h2>` header line, add a settings button next to the title:
```vue
    <div class="d-flex align-center mb-4">
      <h2 class="text-h5">{{ namespace }}/{{ name }}</h2>
      <v-spacer />
      <v-btn icon variant="text" :to="`/report/${scm}/${namespace}/${name}/settings`" aria-label="settings">
        <v-icon>mdi-cog</v-icon>
      </v-btn>
    </div>
```
(Replace the existing `<h2 class="text-h5 mb-4">{{ namespace }}/{{ name }}</h2>` line.)

- [ ] **Step 4: Verify build + tests**

Run: `cd web && npm run build` → success. `npx vitest run` → all pass.

- [ ] **Step 5: Commit**

```bash
cd .. && git add web/src/views/SettingsView.vue web/src/router/index.ts web/src/views/BuildsView.vue
git commit -m "web: settings page and route"
```

---

## Task 8: Account bindings on the user page

**Files:**
- Create: `web/src/components/AccountBindings.vue`, `web/src/components/__tests__/AccountBindings.spec.ts`
- Modify: `web/src/views/User.vue`

- [ ] **Step 1: Create `web/src/components/AccountBindings.vue`**

```vue
<script setup lang="ts">
import { onMounted, computed } from 'vue'
import { useUserStore } from '@/stores/user'
import { basePath } from '@/lib/base'

const user = useUserStore()
onMounted(() => user.fetchScm())

const providers = [
  { scm: 'github', label: 'GitHub', icon: 'mdi-github' },
  { scm: 'gitea', label: 'Gitea', icon: 'mdi-git' },
  { scm: 'gitlab', label: 'GitLab', icon: 'mdi-gitlab' }
]

const rows = computed(() => providers.map((p) => ({ ...p, linked: !!user.scm[p.scm] })))
function linkUrl(scm: string) {
  return `${basePath()}/login/${scm}?bind`
}
</script>

<template>
  <v-card class="pa-4">
    <div class="text-h6 mb-2">Linked accounts</div>
    <v-list>
      <v-list-item v-for="p in rows" :key="p.scm" :title="p.label" :prepend-icon="p.icon">
        <template #append>
          <v-icon v-if="p.linked" color="success" aria-label="linked">mdi-check-circle</v-icon>
          <v-btn v-else size="small" variant="tonal" :href="linkUrl(p.scm)">Link</v-btn>
        </template>
      </v-list-item>
    </v-list>
  </v-card>
</template>
```

- [ ] **Step 2: Embed in `web/src/views/User.vue`**

Add the import and render it under the profile card. Add to `<script setup>`:
```ts
import AccountBindings from '@/components/AccountBindings.vue'
```
And in the template, after the profile `</v-card>`'s column, add another column:
```vue
      <v-col cols="12" md="6">
        <AccountBindings />
      </v-col>
```
(Place inside the existing `<v-row justify="center">`.)

- [ ] **Step 3: Write `web/src/components/__tests__/AccountBindings.spec.ts`**

```ts
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import { setActivePinia, createPinia } from 'pinia'
import http from '@/plugins/http'
import AccountBindings from '@/components/AccountBindings.vue'

vi.mock('@/plugins/http', () => ({ default: { get: vi.fn() }, errorMessage: (e: unknown) => String(e) }))

const vuetify = createVuetify({ components })

describe('AccountBindings', () => {
  beforeEach(() => { vi.clearAllMocks(); setActivePinia(createPinia()) })

  it('shows linked check vs Link button', async () => {
    ;(http.get as any).mockResolvedValue({ data: { github: true, gitea: false } })
    const w = mount(AccountBindings, { global: { plugins: [vuetify] } })
    await flushPromises()
    expect(w.text()).toContain('GitHub')
    expect(w.find('[aria-label="linked"]').exists()).toBe(true)
    expect(w.text()).toContain('Link')
  })
})
```

- [ ] **Step 4: Run**

Run: `cd web && npx vitest run src/components/__tests__/AccountBindings.spec.ts` → PASS.

- [ ] **Step 5: Commit**

```bash
cd .. && git add web/src/components/AccountBindings.vue web/src/components/__tests__/AccountBindings.spec.ts web/src/views/User.vue
git commit -m "web: scm account bindings on user page"
```

---

## Task 9: CI coverage reporting + final verification

**Files:**
- Modify: `web/package.json`, `web/vite.config.ts`, `.github/workflows/ci.yml`

- [ ] **Step 1: Add the coverage dev dep + script**

```bash
cd web && npm install -D @vitest/coverage-v8
```
In `web/package.json` `scripts`, add: `"test:coverage": "vitest run --coverage"`.

- [ ] **Step 2: Configure coverage in `web/vite.config.ts`**

In the `test` block, add a `coverage` key:
```ts
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
```

- [ ] **Step 3: Add the upload step to `.github/workflows/ci.yml`**

After the `Frontend Build` step in the FRONTEND branch, add:
```yaml
      - name: Frontend Coverage
        if: env.BUILD_TYPE == 'FRONTEND'
        run: |
          cd web
          npm run test:coverage

      - name: Jest Report
        if: env.BUILD_TYPE == 'FRONTEND'
        uses: covergates/github-actions@v1
        with:
          report-id: "bsi5dvi23akg00a0tgl0"
          report-type: "lcov"
          report-file: "./web/coverage/lcov.info"
          pull-request: "true"
```

- [ ] **Step 4: Verify coverage generates**

Run: `cd web && npm run test:coverage` → tests pass and `web/coverage/lcov.info` is produced. Confirm `web/.gitignore` ignores `coverage/` (B1 added `coverage/`); if not, add it. Do NOT commit `web/coverage`.

- [ ] **Step 5: Full verification**

```bash
cd web && npx vitest run && npm run lint && npm run build
cd .. && go build ./core/... ./modules/... ./routers/... && go test ./routers/api/repo/ -count=1
```
Expected: all green. `git status` shows no `web/coverage`, `web/dist`, `web/node_modules`, `web/web_gen.go`.

- [ ] **Step 6: Commit**

```bash
git add web/package.json web/package-lock.json web/vite.config.ts .github/workflows/ci.yml
git commit -m "web: frontend coverage reporting in ci"
```

---

## Self-Review notes (spec coverage)

- **B3a.1 authz hardening** → Task 1 (canAdminRepo + GetSetting/HookCreate gates; UpdateSetting already admin-gated, refactored to the helper).
- **B3a.2 settings page** → Tasks 5 (general), 6 (webhook/embed/guide), 7 (SettingsView assembly + route + gear link); token via Task 4 composable.
- **B3a.3 SCM binding** → Task 8.
- **B3a.4 data layer** → Tasks 2 (repository setting), 3 (user scm).
- **B3a.5 CI coverage** → Task 9.
- **B3a.6 testing** → tests in Tasks 1,2,3,4,5,6,8.

Deferred to B3b: folder tree, branch/PR views, parallel-jobs detail, flags backend+UI.
