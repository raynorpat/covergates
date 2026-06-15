# Coveralls Notifications & Checks — C1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** After a build finalizes, compute a pass/fail/error coverage verdict from the repo's policy settings and post a `coverage/covergates` commit status to the build's commit on the SCM.

**Architecture:** A new `modules/notify` package: a pure `Evaluate` verdict function, a best-effort `NotifyService` dispatcher that fans out to `core.Notifier`s, and a commit-status notifier that posts via a new `GitRepoService.CreateStatus` method (backed by go-scm `Repositories.CreateStatus`). Wired into the build upload/webhook handlers right after `Finalize`, using the repo creator's SCM token.

**Tech Stack:** Go 1.14, Gin, GORM, google/wire (hand-edited `wire_gen.go`), gomock (hand-edited mocks under `mock/`), `github.com/drone/go-scm` v1.27.0, `sirupsen/logrus`; Vue 3 + Vuetify 3 + Vitest frontend.

---

## Environment notes (read first)

- **No CGO locally:** `models` tests need `go-sqlite3` (CGO) and cannot run here. Build/vet with `CGO_ENABLED=0`. Run Go tests from the **repo root** `D:\raynorpat\covergates` (cwd persists between Bash calls — a stray `cd web` breaks relative package paths).
- **Frontend** commands run from `web/`: `npx vitest run`, `npm run lint`, `npm run build`.
- **Coverage units:** `core.Build.Coverage` and `CoverageChange` are ratios in `[0,1]`. The two new settings are **whole percent** (`0` disables the check).
- **Mocks are committed** (`mock/*.go`, MockGen-style but hand-edited). Adding a method to a mocked interface requires editing the corresponding mock in the same task or the package stops compiling.

## File Structure

**Create:**
- `core/notify.go` — `StatusState`, `Status`, `Verdict`, `Notifier`, `NotifyService` interfaces.
- `modules/notify/verdict.go` — pure `Evaluate(build, setting) *core.Verdict`.
- `modules/notify/verdict_test.go` — table tests.
- `modules/notify/service.go` — `Service` (dispatcher) implementing `core.NotifyService`.
- `modules/notify/service_test.go` — dispatch + best-effort tests.
- `modules/notify/status.go` — `StatusNotifier` implementing `core.Notifier`.
- `modules/notify/status_test.go` — notifier tests via SCM mock.
- `mock/notify_mock.go` — `MockNotifyService`, `MockNotifier`.
- `web/src/components/SettingsChecks.vue` — Checks settings card.
- `web/src/components/__tests__/SettingsChecks.spec.ts` — UI test.

**Modify:**
- `core/repo.go` — two new `RepoSetting` fields.
- `core/scm.go` — add `CreateStatus` to `GitRepoService`.
- `modules/scm/repo.go` — implement `CreateStatus` + `toSCMState`.
- `modules/scm/repo_test.go` — `toSCMState` mapping test (new test in existing file).
- `mock/scm_mock.go` — add `CreateStatus` to `MockGitRepoService`.
- `cmd/server/inject_service.go` — `provideNotifyService` + add to `serviceSet`.
- `cmd/server/wire_gen.go` — construct `notifyService`, pass to `provideRouter`.
- `cmd/server/inject_router.go` — `provideRouter` gains `notifyService` param + sets field.
- `routers/init.go` — `Routers` gains `NotifyService`; pass to `api.Router`.
- `routers/api/api.go` — `Router` gains `NotifyService`; pass to handlers.
- `routers/api/build/build.go` — `HandleJobs`/`HandleWebhook` gain `core.NotifyService` param + call `Notify` after finalize.
- `routers/api/build/build_test.go` — pass a `MockNotifyService` to handlers; expect `Notify`.
- `web/src/types/setting.ts` — two new numeric fields.
- `web/src/views/SettingsView.vue` — render `SettingsChecks`.

---

## Task 1: Verdict types + Evaluate

**Files:**
- Create: `core/notify.go`
- Modify: `core/repo.go` (RepoSetting fields)
- Create: `modules/notify/verdict.go`
- Test: `modules/notify/verdict_test.go`

- [ ] **Step 1: Add the two settings fields**

In `core/repo.go`, add to the `RepoSetting` struct (after `Protected`):

```go
// RepoSetting to customize repository
type RepoSetting struct {
	Filters          FileNameFilters    `json:"filters"`
	MergePullRequest bool               `json:"mergePR"`
	UpdateAction     ReportUpdateAction `json:"updateAction"`
	// Protected project from unauthorized user upload report
	Protected bool `json:"protected"`
	// CoverageMinimum is the minimum total coverage percent (0..100); 0 disables the check.
	CoverageMinimum float64 `json:"coverageMinimum"`
	// CoverageDecreaseThreshold is the max allowed coverage drop in percentage
	// points vs the base build; 0 means any decrease fails.
	CoverageDecreaseThreshold float64 `json:"coverageDecreaseThreshold"`
}
```

- [ ] **Step 2: Create core notification types**

Create `core/notify.go`:

```go
package core

import "context"

//go:generate mockgen -package mock -destination ../mock/notify_mock.go . NotifyService,Notifier

// StatusState is a commit-status / verdict state.
type StatusState string

// Status states.
const (
	StatusPending StatusState = "pending"
	StatusSuccess StatusState = "success"
	StatusFailure StatusState = "failure"
	StatusError   StatusState = "error"
)

// Status is a commit status to post to an SCM.
type Status struct {
	State  StatusState
	Label  string // status context, e.g. "coverage/covergates"
	Desc   string // short human description
	Target string // URL the status links to (build detail page)
}

// Verdict is the evaluated outcome of a finalized build under the repo's policy.
type Verdict struct {
	State       StatusState
	Coverage    float64 // ratio 0..1
	Change      float64 // ratio delta vs base (negative = decrease)
	HasBase     bool
	Description string
}

// Notifier delivers a build's verdict over one channel (status, comment, email, ...).
type Notifier interface {
	Notify(ctx context.Context, repo *Repo, build *Build, verdict *Verdict) error
}

// NotifyService computes the verdict for a finalized build and dispatches notifiers.
type NotifyService interface {
	Notify(ctx context.Context, repo *Repo, build *Build) error
}
```

- [ ] **Step 3: Write the failing verdict test**

Create `modules/notify/verdict_test.go`:

```go
package notify

import (
	"strings"
	"testing"

	"github.com/covergates/covergates/core"
)

func TestEvaluate(t *testing.T) {
	cases := []struct {
		name      string
		build     *core.Build
		setting   *core.RepoSetting
		wantState core.StatusState
		wantDesc  string // substring
	}{
		{
			name:      "errored build",
			build:     &core.Build{Status: core.BuildErrored, Coverage: 0.5},
			setting:   &core.RepoSetting{},
			wantState: core.StatusError,
			wantDesc:  "build failed",
		},
		{
			name:      "no base no minimum passes",
			build:     &core.Build{Status: core.BuildDone, Coverage: 0.853},
			setting:   &core.RepoSetting{},
			wantState: core.StatusSuccess,
			wantDesc:  "85.3%",
		},
		{
			name:      "base no decrease passes",
			build:     &core.Build{Status: core.BuildDone, Coverage: 0.90, CoverageChange: 0.05, BaseBuildID: 7},
			setting:   &core.RepoSetting{},
			wantState: core.StatusSuccess,
			wantDesc:  "+5.0%",
		},
		{
			name:      "any decrease fails with default threshold",
			build:     &core.Build{Status: core.BuildDone, Coverage: 0.84, CoverageChange: -0.012, BaseBuildID: 7},
			setting:   &core.RepoSetting{},
			wantState: core.StatusFailure,
			wantDesc:  "decreased",
		},
		{
			name:      "decrease within threshold passes",
			build:     &core.Build{Status: core.BuildDone, Coverage: 0.84, CoverageChange: -0.01, BaseBuildID: 7},
			setting:   &core.RepoSetting{CoverageDecreaseThreshold: 2},
			wantState: core.StatusSuccess,
			wantDesc:  "-1.0%",
		},
		{
			name:      "below minimum fails",
			build:     &core.Build{Status: core.BuildDone, Coverage: 0.40},
			setting:   &core.RepoSetting{CoverageMinimum: 80},
			wantState: core.StatusFailure,
			wantDesc:  "below the minimum",
		},
		{
			name:      "minimum takes precedence over decrease",
			build:     &core.Build{Status: core.BuildDone, Coverage: 0.40, CoverageChange: -0.10, BaseBuildID: 7},
			setting:   &core.RepoSetting{CoverageMinimum: 80, CoverageDecreaseThreshold: 5},
			wantState: core.StatusFailure,
			wantDesc:  "below the minimum",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := Evaluate(tc.build, tc.setting)
			if v.State != tc.wantState {
				t.Fatalf("state = %q, want %q", v.State, tc.wantState)
			}
			if !strings.Contains(v.Description, tc.wantDesc) {
				t.Fatalf("description %q does not contain %q", v.Description, tc.wantDesc)
			}
		})
	}
}
```

- [ ] **Step 4: Run the test to verify it fails**

Run: `cd /d/raynorpat/covergates && CGO_ENABLED=0 go test ./modules/notify/`
Expected: FAIL — `undefined: Evaluate` (package does not compile yet).

- [ ] **Step 5: Implement Evaluate**

Create `modules/notify/verdict.go`:

```go
package notify

import (
	"fmt"

	"github.com/covergates/covergates/core"
)

// Evaluate derives the commit-status verdict for a finalized build under the
// repository's coverage policy. Coverage percentages are whole numbers; the
// build's Coverage/CoverageChange are ratios in [0,1].
func Evaluate(build *core.Build, setting *core.RepoSetting) *core.Verdict {
	v := &core.Verdict{
		Coverage: build.Coverage,
		Change:   build.CoverageChange,
		HasBase:  build.BaseBuildID > 0,
	}
	if build.Status == core.BuildErrored {
		v.State = core.StatusError
		v.Description = "Coverage build failed"
		return v
	}

	covPct := build.Coverage * 100
	v.State = core.StatusSuccess

	if setting != nil && setting.CoverageMinimum > 0 && covPct < setting.CoverageMinimum {
		v.State = core.StatusFailure
		v.Description = fmt.Sprintf("Coverage %.1f%% is below the minimum %.1f%%", covPct, setting.CoverageMinimum)
		return v
	}

	if v.HasBase {
		decreasePct := -build.CoverageChange * 100 // positive when coverage dropped
		threshold := 0.0
		if setting != nil {
			threshold = setting.CoverageDecreaseThreshold
		}
		if decreasePct > threshold {
			v.State = core.StatusFailure
			v.Description = fmt.Sprintf("Coverage decreased %.1f%% to %.1f%%", decreasePct, covPct)
			return v
		}
	}

	v.Description = describePass(covPct, build.CoverageChange, v.HasBase)
	return v
}

func describePass(covPct, change float64, hasBase bool) string {
	if hasBase {
		return fmt.Sprintf("Coverage: %.1f%% (%+.1f%%)", covPct, change*100)
	}
	return fmt.Sprintf("Coverage: %.1f%%", covPct)
}
```

- [ ] **Step 6: Run the test to verify it passes**

Run: `cd /d/raynorpat/covergates && CGO_ENABLED=0 go test ./modules/notify/`
Expected: PASS (`ok ... modules/notify`).

- [ ] **Step 7: Commit**

```bash
cd /d/raynorpat/covergates
git add core/notify.go core/repo.go modules/notify/verdict.go modules/notify/verdict_test.go
git commit -m "notify: coverage verdict policy and core types"
```

---

## Task 2: SCM commit-status method

**Files:**
- Modify: `core/scm.go` (add `CreateStatus` to `GitRepoService`)
- Modify: `modules/scm/repo.go` (implement `CreateStatus`, `toSCMState`)
- Modify: `mock/scm_mock.go` (add `CreateStatus` to `MockGitRepoService`)
- Test: `modules/scm/repo_test.go` (add `toSCMState` test)

- [ ] **Step 1: Add the interface method**

In `core/scm.go`, add to the `GitRepoService` interface (after `IsAdmin`):

```go
	IsAdmin(ctx context.Context, user *User, name string) bool
	// CreateStatus posts a commit status to the given ref (commit SHA).
	CreateStatus(ctx context.Context, user *User, repo, ref string, status *Status) error
```

- [ ] **Step 2: Write the failing mapping test**

Add to `modules/scm/repo_test.go` (new test function; keep existing tests):

```go
func TestToSCMState(t *testing.T) {
	cases := []struct {
		in   core.StatusState
		want scm.State
	}{
		{core.StatusSuccess, scm.StateSuccess},
		{core.StatusFailure, scm.StateFailure},
		{core.StatusError, scm.StateError},
		{core.StatusPending, scm.StatePending},
		{core.StatusState("garbage"), scm.StatePending},
	}
	for _, tc := range cases {
		if got := toSCMState(tc.in); got != tc.want {
			t.Fatalf("toSCMState(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
```

If `modules/scm/repo_test.go` does not already import `scm "github.com/drone/go-scm/scm"` and `core`, add them to its import block.

- [ ] **Step 3: Run the test to verify it fails**

Run: `cd /d/raynorpat/covergates && CGO_ENABLED=0 go test ./modules/scm/ -run TestToSCMState`
Expected: FAIL — `undefined: toSCMState`.

- [ ] **Step 4: Implement CreateStatus and toSCMState**

Append to `modules/scm/repo.go` (the file already declares `type repoService struct` and imports `scm "github.com/drone/go-scm/scm"` and `core`):

```go
func (service *repoService) CreateStatus(ctx context.Context, user *core.User, repo, ref string, status *core.Status) error {
	ctx = withUser(ctx, service.scm, user)
	input := &scm.StatusInput{
		State:  toSCMState(status.State),
		Label:  status.Label,
		Title:  status.Label,
		Desc:   status.Desc,
		Target: status.Target,
	}
	_, _, err := service.client.Repositories.CreateStatus(ctx, repo, ref, input)
	return err
}

func toSCMState(s core.StatusState) scm.State {
	switch s {
	case core.StatusSuccess:
		return scm.StateSuccess
	case core.StatusFailure:
		return scm.StateFailure
	case core.StatusError:
		return scm.StateError
	default:
		return scm.StatePending
	}
}
```

If `modules/scm/repo.go`'s `repoService` methods use a different receiver name or the struct embeds the client differently, match the existing `func (service *repoService) ...` pattern and the existing `service.client` / `service.scm` / `withUser(ctx, service.scm, user)` usage exactly (see `CreateHook`/`Find` in the same file).

- [ ] **Step 5: Add CreateStatus to the mock**

In `mock/scm_mock.go`, after the existing `MockGitRepoService` `IsAdmin` method + recorder, add:

```go
// CreateStatus mocks base method.
func (m *MockGitRepoService) CreateStatus(arg0 context.Context, arg1 *core.User, arg2, arg3 string, arg4 *core.Status) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateStatus", arg0, arg1, arg2, arg3, arg4)
	ret0, _ := ret[0].(error)
	return ret0
}

// CreateStatus indicates an expected call of CreateStatus.
func (mr *MockGitRepoServiceMockRecorder) CreateStatus(arg0, arg1, arg2, arg3, arg4 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateStatus", reflect.TypeOf((*MockGitRepoService)(nil).CreateStatus), arg0, arg1, arg2, arg3, arg4)
}
```

- [ ] **Step 6: Run tests and build**

Run:
```
cd /d/raynorpat/covergates && CGO_ENABLED=0 go test ./modules/scm/ -run TestToSCMState && CGO_ENABLED=0 go build ./core/... ./modules/scm/... ./mock/...
```
Expected: `ok ... modules/scm` and a clean build (the mock compiles with the new method).

- [ ] **Step 7: Commit**

```bash
cd /d/raynorpat/covergates
git add core/scm.go modules/scm/repo.go modules/scm/repo_test.go mock/scm_mock.go
git commit -m "scm: add commit status CreateStatus to git repo service"
```

---

## Task 3: NotifyService dispatcher + mock

**Files:**
- Create: `modules/notify/service.go`
- Create: `mock/notify_mock.go`
- Test: `modules/notify/service_test.go`

- [ ] **Step 1: Write the failing dispatch test**

Create `modules/notify/service_test.go`:

```go
package notify

import (
	"context"
	"errors"
	"testing"

	"github.com/covergates/covergates/core"
	"github.com/covergates/covergates/mock"
	"github.com/golang/mock/gomock"
)

type recordingNotifier struct {
	called  bool
	verdict *core.Verdict
	err     error
}

func (n *recordingNotifier) Notify(_ context.Context, _ *core.Repo, _ *core.Build, v *core.Verdict) error {
	n.called = true
	n.verdict = v
	return n.err
}

func TestServiceDispatchesAndSwallowsErrors(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repos := mock.NewMockRepoStore(ctrl)
	repo := &core.Repo{ID: 1}
	build := &core.Build{Status: core.BuildDone, Coverage: 0.5, CoverageChange: -0.1, BaseBuildID: 3}
	repos.EXPECT().Setting(repo).Return(&core.RepoSetting{}, nil)

	failing := &recordingNotifier{err: errors.New("boom")}
	ok := &recordingNotifier{}
	svc := &Service{Repos: repos, Notifiers: []core.Notifier{failing, ok}}

	if err := svc.Notify(context.Background(), repo, build); err != nil {
		t.Fatalf("Notify returned error: %v", err)
	}
	if !failing.called || !ok.called {
		t.Fatal("expected all notifiers to be called")
	}
	if ok.verdict == nil || ok.verdict.State != core.StatusFailure {
		t.Fatalf("expected failure verdict passed to notifier, got %+v", ok.verdict)
	}
}

func TestServiceTreatsSettingErrorAsEmpty(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repos := mock.NewMockRepoStore(ctrl)
	repo := &core.Repo{ID: 1}
	build := &core.Build{Status: core.BuildDone, Coverage: 0.9}
	repos.EXPECT().Setting(repo).Return(nil, errors.New("db down"))

	n := &recordingNotifier{}
	svc := &Service{Repos: repos, Notifiers: []core.Notifier{n}}
	if err := svc.Notify(context.Background(), repo, build); err != nil {
		t.Fatalf("Notify returned error: %v", err)
	}
	if n.verdict == nil || n.verdict.State != core.StatusSuccess {
		t.Fatalf("expected success verdict, got %+v", n.verdict)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `cd /d/raynorpat/covergates && CGO_ENABLED=0 go test ./modules/notify/ -run TestService`
Expected: FAIL — `undefined: Service`.

- [ ] **Step 3: Implement the dispatcher**

Create `modules/notify/service.go`:

```go
package notify

import (
	"context"

	"github.com/covergates/covergates/core"
	log "github.com/sirupsen/logrus"
)

// Service computes a build's verdict and dispatches it to each notifier.
// Notification is best-effort: a notifier error is logged, never returned.
type Service struct {
	Repos     core.RepoStore
	Notifiers []core.Notifier
}

// Notify evaluates the build under the repo's policy and runs every notifier.
func (s *Service) Notify(ctx context.Context, repo *core.Repo, build *core.Build) error {
	setting, err := s.Repos.Setting(repo)
	if err != nil || setting == nil {
		setting = &core.RepoSetting{}
	}
	verdict := Evaluate(build, setting)
	for _, n := range s.Notifiers {
		if err := n.Notify(ctx, repo, build, verdict); err != nil {
			log.Errorf("notify: notifier failed: %v", err)
		}
	}
	return nil
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `cd /d/raynorpat/covergates && CGO_ENABLED=0 go test ./modules/notify/ -run TestService`
Expected: PASS.

- [ ] **Step 5: Create the NotifyService/Notifier mock**

Create `mock/notify_mock.go` (gomock-style, matching the other mocks in `mock/`):

```go
// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/covergates/covergates/core (interfaces: NotifyService,Notifier)

package mock

import (
	context "context"
	reflect "reflect"

	core "github.com/covergates/covergates/core"
	gomock "github.com/golang/mock/gomock"
)

// MockNotifyService is a mock of NotifyService interface.
type MockNotifyService struct {
	ctrl     *gomock.Controller
	recorder *MockNotifyServiceMockRecorder
}

// MockNotifyServiceMockRecorder is the mock recorder for MockNotifyService.
type MockNotifyServiceMockRecorder struct {
	mock *MockNotifyService
}

// NewMockNotifyService creates a new mock instance.
func NewMockNotifyService(ctrl *gomock.Controller) *MockNotifyService {
	mock := &MockNotifyService{ctrl: ctrl}
	mock.recorder = &MockNotifyServiceMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockNotifyService) EXPECT() *MockNotifyServiceMockRecorder {
	return m.recorder
}

// Notify mocks base method.
func (m *MockNotifyService) Notify(arg0 context.Context, arg1 *core.Repo, arg2 *core.Build) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Notify", arg0, arg1, arg2)
	ret0, _ := ret[0].(error)
	return ret0
}

// Notify indicates an expected call of Notify.
func (mr *MockNotifyServiceMockRecorder) Notify(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Notify", reflect.TypeOf((*MockNotifyService)(nil).Notify), arg0, arg1, arg2)
}

// MockNotifier is a mock of Notifier interface.
type MockNotifier struct {
	ctrl     *gomock.Controller
	recorder *MockNotifierMockRecorder
}

// MockNotifierMockRecorder is the mock recorder for MockNotifier.
type MockNotifierMockRecorder struct {
	mock *MockNotifier
}

// NewMockNotifier creates a new mock instance.
func NewMockNotifier(ctrl *gomock.Controller) *MockNotifier {
	mock := &MockNotifier{ctrl: ctrl}
	mock.recorder = &MockNotifierMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockNotifier) EXPECT() *MockNotifierMockRecorder {
	return m.recorder
}

// Notify mocks base method.
func (m *MockNotifier) Notify(arg0 context.Context, arg1 *core.Repo, arg2 *core.Build, arg3 *core.Verdict) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Notify", arg0, arg1, arg2, arg3)
	ret0, _ := ret[0].(error)
	return ret0
}

// Notify indicates an expected call of Notify.
func (mr *MockNotifierMockRecorder) Notify(arg0, arg1, arg2, arg3 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Notify", reflect.TypeOf((*MockNotifier)(nil).Notify), arg0, arg1, arg2, arg3)
}
```

- [ ] **Step 6: Build the mock package**

Run: `cd /d/raynorpat/covergates && CGO_ENABLED=0 go build ./mock/... ./modules/notify/...`
Expected: clean build.

- [ ] **Step 7: Commit**

```bash
cd /d/raynorpat/covergates
git add modules/notify/service.go modules/notify/service_test.go mock/notify_mock.go
git commit -m "notify: best-effort dispatch service and mocks"
```

---

## Task 4: Commit-status notifier

**Files:**
- Create: `modules/notify/status.go`
- Test: `modules/notify/status_test.go`

- [ ] **Step 1: Write the failing notifier test**

Create `modules/notify/status_test.go`:

```go
package notify

import (
	"context"
	"testing"

	"github.com/covergates/covergates/config"
	"github.com/covergates/covergates/core"
	"github.com/covergates/covergates/mock"
	"github.com/golang/mock/gomock"
)

func testConfig() *config.Config {
	cfg := &config.Config{}
	cfg.Server.Addr = "http://localhost:8080"
	return cfg
}

func TestStatusNotifierPostsStatus(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	scmService := mock.NewMockSCMService(ctrl)
	client := mock.NewMockClient(ctrl)
	repos := mock.NewMockGitRepoService(ctrl)
	store := mock.NewMockRepoStore(ctrl)

	repo := &core.Repo{ID: 1, NameSpace: "octocat", Name: "hello", SCM: core.Github}
	build := &core.Build{Number: 12, Commit: "abc123"}
	verdict := &core.Verdict{State: core.StatusSuccess, Description: "Coverage: 90.0%"}

	store.EXPECT().Creator(repo).Return(&core.User{Login: "octocat"}, nil)
	scmService.EXPECT().Client(repo.SCM).Return(client, nil)
	client.EXPECT().Repositories().Return(repos)
	repos.EXPECT().CreateStatus(
		gomock.Any(),
		gomock.Any(),
		repo.FullName(),
		"abc123",
		gomock.AssignableToTypeOf(&core.Status{}),
	).DoAndReturn(func(_ context.Context, _ *core.User, _ , _ string, s *core.Status) error {
		if s.State != core.StatusSuccess {
			t.Errorf("state = %q, want success", s.State)
		}
		if s.Label != "coverage/covergates" {
			t.Errorf("label = %q", s.Label)
		}
		if s.Target == "" {
			t.Error("expected non-empty target URL")
		}
		return nil
	})

	n := &StatusNotifier{SCM: scmService, Repos: store, Config: testConfig()}
	if err := n.Notify(context.Background(), repo, build, verdict); err != nil {
		t.Fatalf("Notify error: %v", err)
	}
}

func TestStatusNotifierSkipsBlankCommit(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	n := &StatusNotifier{
		SCM:    mock.NewMockSCMService(ctrl),
		Repos:  mock.NewMockRepoStore(ctrl),
		Config: testConfig(),
	}
	build := &core.Build{Number: 1, Commit: ""}
	if err := n.Notify(context.Background(), &core.Repo{}, build, &core.Verdict{}); err != nil {
		t.Fatalf("expected nil error for blank commit, got %v", err)
	}
}
```

Note: `core.Github` is the GitHub provider constant (`core/const.go`); `Server.URL()` with only `Addr` set returns `http://localhost:8080`, so the status target is non-empty.

- [ ] **Step 2: Run the test to verify it fails**

Run: `cd /d/raynorpat/covergates && CGO_ENABLED=0 go test ./modules/notify/ -run TestStatusNotifier`
Expected: FAIL — `undefined: StatusNotifier`.

- [ ] **Step 3: Implement the notifier**

Create `modules/notify/status.go`:

```go
package notify

import (
	"context"
	"fmt"

	"github.com/covergates/covergates/config"
	"github.com/covergates/covergates/core"
)

// StatusNotifier posts a commit status reflecting the coverage verdict, using
// the repository creator's SCM token.
type StatusNotifier struct {
	SCM    core.SCMService
	Repos  core.RepoStore
	Config *config.Config
}

// Notify posts the verdict as a commit status on the build's commit.
func (n *StatusNotifier) Notify(ctx context.Context, repo *core.Repo, build *core.Build, verdict *core.Verdict) error {
	if build.Commit == "" {
		return nil
	}
	user, err := n.Repos.Creator(repo)
	if err != nil {
		return err
	}
	client, err := n.SCM.Client(repo.SCM)
	if err != nil {
		return err
	}
	status := &core.Status{
		State:  verdict.State,
		Label:  "coverage/covergates",
		Desc:   verdict.Description,
		Target: n.buildURL(repo, build),
	}
	return client.Repositories().CreateStatus(ctx, user, repo.FullName(), build.Commit, status)
}

func (n *StatusNotifier) buildURL(repo *core.Repo, build *core.Build) string {
	return fmt.Sprintf("%s/report/%s/%s/builds/%d", n.Config.Server.URL(), repo.SCM, repo.FullName(), build.Number)
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `cd /d/raynorpat/covergates && CGO_ENABLED=0 go test ./modules/notify/`
Expected: PASS (all notify tests).

- [ ] **Step 5: Commit**

```bash
cd /d/raynorpat/covergates
git add modules/notify/status.go modules/notify/status_test.go
git commit -m "notify: commit status notifier"
```

---

## Task 5: Wire NotifyService into build handlers

**Files:**
- Modify: `routers/api/build/build.go`
- Modify: `routers/api/build/build_test.go`
- Modify: `routers/api/api.go`
- Modify: `routers/init.go`
- Modify: `cmd/server/inject_router.go`
- Modify: `cmd/server/inject_service.go`
- Modify: `cmd/server/wire_gen.go`

- [ ] **Step 1: Add NotifyService to the handlers and call it after finalize**

In `routers/api/build/build.go`:

Change `HandleJobs`'s signature to add a `notifyService core.NotifyService` parameter (after `buildService core.BuildService`):

```go
func HandleJobs(
	cfg *config.Config,
	repoStore core.RepoStore,
	buildStore core.BuildStore,
	buildService core.BuildService,
	notifyService core.NotifyService,
) gin.HandlerFunc {
```

In its `if !payload.Parallel { ... }` block, after the successful `Finalize`, add the notify call:

```go
		if !payload.Parallel {
			if err := buildService.Finalize(c.Request.Context(), repo, build); err != nil {
				c.JSON(500, gin.H{"message": err.Error()})
				return
			}
			_ = notifyService.Notify(c.Request.Context(), repo, build)
		}
```

Change `HandleWebhook`'s signature to add the parameter (after `buildService core.BuildService`):

```go
func HandleWebhook(
	repoStore core.RepoStore,
	buildStore core.BuildStore,
	buildService core.BuildService,
	notifyService core.NotifyService,
) gin.HandlerFunc {
```

And after its successful `Finalize`:

```go
		if err := buildService.Finalize(c.Request.Context(), repo, build); err != nil {
			c.JSON(500, gin.H{"message": err.Error()})
			return
		}
		_ = notifyService.Notify(c.Request.Context(), repo, build)
		c.JSON(200, gin.H{"done": true})
```

- [ ] **Step 2: Update the handler tests to supply the mock and expect Notify**

In `routers/api/build/build_test.go`:

For `TestHandleJobsSingleFinalizes` (the non-parallel finalize case): construct `notify := mock.NewMockNotifyService(ctrl)`, expect a call, and pass it to `HandleJobs`:

```go
	notify := mock.NewMockNotifyService(ctrl)
	notify.EXPECT().Notify(gomock.Any(), repo, gomock.Any()).Return(nil)
	...
	r.POST("/api/v1/jobs", HandleJobs(newConfig(), repos, builds, svc, notify))
```

For `TestHandleJobsParallelDoesNotFinalize` and `TestHandleJobsBadToken` (no finalize): construct the mock with **no** `Notify` expectation and pass it:

```go
	notify := mock.NewMockNotifyService(ctrl)
	...
	r.POST("/api/v1/jobs", HandleJobs(newConfig(), repos, builds, svc, notify))
```

For `TestHandleWebhookDone`: expect `Notify` and pass the mock:

```go
	notify := mock.NewMockNotifyService(ctrl)
	notify.EXPECT().Notify(gomock.Any(), repo, gomock.Any()).Return(nil)
	...
	r.POST("/webhook", HandleWebhook(repos, builds, svc, notify))
```

Apply the same `HandleWebhook(repos, builds, svc, notify)` signature update to any other webhook test cases in the file (e.g. error/not-found cases construct the mock with no `Notify` expectation). Search the file for `HandleJobs(` and `HandleWebhook(` and update every call site.

- [ ] **Step 3: Thread NotifyService through the API router**

In `routers/api/api.go`, add to the `Router` struct (in the service group, after `BuildService`):

```go
	BuildService  core.BuildService
	NotifyService core.NotifyService
```

Update the two route registrations to pass it:

```go
	g.POST("/jobs", build.HandleJobs(r.Config, r.RepoStore, r.BuildStore, r.BuildService, r.NotifyService))
	...
	e.POST("/webhook", build.HandleWebhook(r.RepoStore, r.BuildStore, r.BuildService, r.NotifyService))
```

- [ ] **Step 4: Thread NotifyService through routers.Routers**

In `routers/init.go`, add to the `Routers` struct (after `BuildService`):

```go
	BuildService core.BuildService
	NotifyService core.NotifyService
```

And in the `apiRoute := &api.Router{ ... }` literal, add:

```go
		BuildService:    r.BuildService,
		NotifyService:   r.NotifyService,
```

- [ ] **Step 5: Add the provider and pass it through provideRouter**

In `cmd/server/inject_service.go`:

Add `provideNotifyService` to the `serviceSet` `wire.NewSet(...)` list (after `provideBuildService`):

```go
	provideBuildService,
	provideNotifyService,
```

Add the import for the notify module to the import block:

```go
	notifymod "github.com/covergates/covergates/modules/notify"
```

Add the provider function (after `provideBuildService`):

```go
func provideNotifyService(
	config *config.Config,
	scmService core.SCMService,
	repoStore core.RepoStore,
) core.NotifyService {
	return &notifymod.Service{
		Repos: repoStore,
		Notifiers: []core.Notifier{
			&notifymod.StatusNotifier{
				SCM:    scmService,
				Repos:  repoStore,
				Config: config,
			},
		},
	}
}
```

In `cmd/server/inject_router.go`, add `notifyService core.NotifyService` to `provideRouter`'s parameters (after `buildService core.BuildService`) and set it on the returned struct:

```go
	buildService core.BuildService,
	notifyService core.NotifyService,
) *routers.Routers {
	return &routers.Routers{
		...
		BuildService:    buildService,
		NotifyService:   notifyService,
	}
}
```

- [ ] **Step 6: Update the hand-edited wire_gen.go**

In `cmd/server/wire_gen.go`, after the `buildService := provideBuildService(...)` line, add:

```go
	notifyService := provideNotifyService(config2, scmService, repoStore)
```

and add `notifyService` to the `provideRouter(...)` call's argument list, in the same position as the new parameter (immediately after `buildService`):

```go
	routers := provideRouter(session, config2, loginMiddleware, scmService, chartService, reportService, repoService, hookService, oAuthService, userStore, reportStore, repoStore, oAuthStore, buildStore, buildService, notifyService)
```

(`config2` is the existing `*config.Config` variable name in `wire_gen.go` — match whatever that file already calls it.)

- [ ] **Step 7: Build everything and run the build router tests**

Run:
```
cd /d/raynorpat/covergates && CGO_ENABLED=0 go build ./... && CGO_ENABLED=0 go test ./routers/api/build/ ./modules/notify/ ./modules/scm/
```
Expected: clean build; `ok` for all three packages.

Note: `go build ./...` may surface the known `web/node_modules` Go-source issue under go1.14. If so, build the relevant packages explicitly instead: `CGO_ENABLED=0 go build ./core/... ./modules/... ./routers/... ./cmd/... ./mock/...`.

- [ ] **Step 8: Commit**

```bash
cd /d/raynorpat/covergates
git add routers/api/build/build.go routers/api/build/build_test.go routers/api/api.go routers/init.go cmd/server/inject_router.go cmd/server/inject_service.go cmd/server/wire_gen.go
git commit -m "notify: wire notification dispatch into build handlers"
```

---

## Task 6: Frontend Checks settings card

**Files:**
- Modify: `web/src/types/setting.ts`
- Create: `web/src/components/SettingsChecks.vue`
- Modify: `web/src/views/SettingsView.vue`
- Test: `web/src/components/__tests__/SettingsChecks.spec.ts`

- [ ] **Step 1: Add the new fields to the RepoSetting type**

Edit `web/src/types/setting.ts`:

```ts
export interface RepoSetting {
  filters: string[]
  mergePR: boolean
  updateAction: string
  protected: boolean
  coverageMinimum: number
  coverageDecreaseThreshold: number
}
```

- [ ] **Step 2: Write the failing component test**

Create `web/src/components/__tests__/SettingsChecks.spec.ts`:

```ts
import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import SettingsChecks from '@/components/SettingsChecks.vue'
import type { RepoSetting } from '@/types/setting'

const vuetify = createVuetify({ components })

const setting: RepoSetting = {
  filters: [], mergePR: false, updateAction: '', protected: false,
  coverageMinimum: 80, coverageDecreaseThreshold: 2
}

describe('SettingsChecks', () => {
  it('emits save with edited check thresholds', async () => {
    const w = mount(SettingsChecks, {
      props: { modelValue: setting },
      global: { plugins: [vuetify] }
    })
    const comp: any = w.vm
    comp.minimum = 90
    comp.decrease = 5
    comp.save()
    const events = w.emitted('save') as RepoSetting[][]
    expect(events).toBeTruthy()
    expect(events[0][0].coverageMinimum).toBe(90)
    expect(events[0][0].coverageDecreaseThreshold).toBe(5)
    // unrelated fields preserved
    expect(events[0][0].protected).toBe(false)
  })
})
```

- [ ] **Step 3: Run the test to verify it fails**

Run: `cd /d/raynorpat/covergates/web && npx vitest run src/components/__tests__/SettingsChecks.spec.ts`
Expected: FAIL — cannot resolve `@/components/SettingsChecks.vue`.

- [ ] **Step 4: Implement the component**

Create `web/src/components/SettingsChecks.vue` (mirrors `SettingsGeneral.vue`'s prop/emit/save pattern):

```vue
<script setup lang="ts">
import { ref, watch } from 'vue'
import type { RepoSetting } from '@/types/setting'

const props = defineProps<{ modelValue: RepoSetting; busy?: boolean }>()
const emit = defineEmits<{ (e: 'save', value: RepoSetting): void }>()

const minimum = ref(props.modelValue.coverageMinimum ?? 0)
const decrease = ref(props.modelValue.coverageDecreaseThreshold ?? 0)

watch(() => props.modelValue, (v) => {
  minimum.value = v.coverageMinimum ?? 0
  decrease.value = v.coverageDecreaseThreshold ?? 0
})

function save() {
  emit('save', {
    ...props.modelValue,
    coverageMinimum: Number(minimum.value) || 0,
    coverageDecreaseThreshold: Number(decrease.value) || 0
  })
}
</script>

<template>
  <v-card class="pa-4 mb-4">
    <div class="text-h6 mb-2">Checks</div>
    <p class="text-body-2 text-medium-emphasis mb-3">
      Set the commit-status policy posted to pull requests. Use 0 to disable a check.
    </p>
    <v-text-field
      v-model.number="minimum"
      type="number"
      label="Minimum coverage % (0 = no minimum)"
      density="compact"
    />
    <v-text-field
      v-model.number="decrease"
      type="number"
      label="Max coverage decrease % (0 = any decrease fails)"
      density="compact"
    />
    <v-btn color="primary" :loading="busy" @click="save">Save</v-btn>
  </v-card>
</template>
```

- [ ] **Step 5: Run the test to verify it passes**

Run: `cd /d/raynorpat/covergates/web && npx vitest run src/components/__tests__/SettingsChecks.spec.ts`
Expected: PASS (1 test).

- [ ] **Step 6: Render the card in the settings view**

In `web/src/views/SettingsView.vue`:

Add the import next to the other component imports:

```ts
import SettingsChecks from '@/components/SettingsChecks.vue'
```

In the template, add the card right after `<SettingsGeneral ... />` (reusing the same `save` handler and `saving` flag):

```html
      <SettingsGeneral :model-value="store.setting" :busy="saving" @save="save" />
      <SettingsChecks :model-value="store.setting" :busy="saving" @save="save" />
```

- [ ] **Step 7: Run the full frontend test suite**

Run: `cd /d/raynorpat/covergates/web && npx vitest run`
Expected: all suites pass (existing + the new SettingsChecks test).

- [ ] **Step 8: Commit**

```bash
cd /d/raynorpat/covergates
git add web/src/types/setting.ts web/src/components/SettingsChecks.vue web/src/components/__tests__/SettingsChecks.spec.ts web/src/views/SettingsView.vue
git commit -m "web: repo checks settings card (coverage thresholds)"
```

---

## Task 7: Full verification

**Files:** none (verification only)

- [ ] **Step 1: Backend build + targeted tests**

Run:
```
cd /d/raynorpat/covergates && CGO_ENABLED=0 go vet ./core/... ./modules/notify/... ./modules/scm/... ./routers/api/build/... ./cmd/... 2>&1 | tail -20
CGO_ENABLED=0 go test ./modules/notify/ ./modules/scm/ ./routers/api/build/ ./modules/build/
```
Expected: vet clean; all four packages `ok`. (`models` is intentionally excluded — it requires CGO/sqlite and cannot run in this environment.)

- [ ] **Step 2: Frontend lint, test, build**

Run:
```
cd /d/raynorpat/covergates/web && npm run lint && npx vitest run && npm run build
```
Expected: lint clean; all Vitest suites pass; production build succeeds (the pre-existing chunk-size warning is benign).

- [ ] **Step 3: Final commit (only if any verification fix was needed)**

```bash
cd /d/raynorpat/covergates
git add -A
git commit -m "notify: C1 verification fixes"
```

(If no changes were required, skip this commit.)

---

## Done criteria

- A finalized non-parallel job upload and a parallel webhook both trigger a `coverage/covergates` commit status on the build's commit, with state derived from the repo's `coverageMinimum` / `coverageDecreaseThreshold` policy (or `error` for errored builds).
- Notification failures never affect the upload/webhook HTTP response.
- The repo Settings page exposes the two threshold fields, saved via the existing setting endpoint.
- All targeted Go packages and the full frontend suite pass; `go vet` is clean.
