# Coveralls Notifications & Checks — C2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** After a pull-request build finalizes, post (and refresh) a single markdown coverage comment on the PR — a pass/fail summary plus a changed-files coverage table.

**Architecture:** A second `core.Notifier` (`CommentNotifier`) added to the C1 dispatch slice. It reads its own enable flag from `RepoStore.Setting`, fetches the PR's changed files via the SCM, renders markdown, and keeps one rolling comment per PR by tracking the comment ID in a new `PullRequestComment` table (go-scm has no comment edit, so delete-old + create-new).

**Tech Stack:** Go 1.14, Gin, GORM, google/wire (hand-edited `wire_gen.go`), gomock (hand-edited mocks under `mock/`), `github.com/drone/go-scm` v1.27.0, `sirupsen/logrus`; Vue 3 + Vuetify 3 + Vitest frontend.

---

## Environment notes (read first)

- **No CGO locally:** `models` tests need `go-sqlite3` (CGO) and cannot run here. Build/vet with `CGO_ENABLED=0`. Run Go commands from the **repo root** `D:\raynorpat\covergates`; cwd persists between Bash calls, so never `cd` into a subdir for go commands. Frontend commands run from `web/`.
- **Coverage units:** `core.Build.Coverage`/`CoverageChange` are ratios in `[0,1]`. Comment percentages render as `%.1f%%`.
- **Mocks are committed** (`mock/*.go`, MockGen-style, hand-edited). `mock/repo_mock.go` and `mock/scm_mock.go` use `m.ctrl.T.Helper()` in both the method and recorder, and **comments without trailing periods** (e.g. `// FindHook mocks base method`). Match that exact style.
- **Builds dispatched to notifiers** already have `SourceFiles`, `Coverage`, `CoverageChange`, `BaseBuildNumber`, `PullRequest`, `Status` populated by `modules/build.Service.Finalize`.

## Background from C1 (do not change)

- `core.Notifier.Notify(ctx, repo, build, verdict) error` and `core.NotifyService` live in `core/notify.go`. `modules/notify.Service{Repos, Notifiers}` dispatches best-effort. `StatusNotifier` is the first notifier; `provideNotifyService` in `cmd/server/inject_service.go` constructs the slice.
- `core.Verdict{State, Coverage, Change, HasBase, Description}` and `core.StatusState` constants (`StatusSuccess/StatusFailure/StatusError/StatusPending`).
- `StatusNotifier` builds its target URL as `fmt.Sprintf("%s/report/%s/%s/builds/%d", cfg.Server.URL(), repo.SCM, repo.FullName(), build.Number)`.

## File Structure

**Create:**
- `models/pull_request_comment.go` — `PullRequestComment` model + the two `RepoStore` methods. (Keeps `models/repo.go` from growing; one file, one responsibility.)
- `modules/notify/comment.go` — `CommentNotifier` + `coverageByPath` + `commentBody`.
- `modules/notify/comment_body_test.go` — pure markdown/coverage tests.
- `modules/notify/comment_test.go` — notifier behavior tests.

**Modify:**
- `core/repo.go` — `RepoSetting.DisablePRComment` field; two new `RepoStore` interface methods.
- `core/scm.go` — add `PullRequestService` to the `//go:generate` mock list (doc only).
- `models/models.go` — add `&PullRequestComment{}` to the migrate list.
- `mock/repo_mock.go` — `FindPullRequestComment`, `UpdatePullRequestComment` on `MockRepoStore`.
- `mock/scm_mock.go` — new `MockPullRequestService`.
- `cmd/server/inject_service.go` — append `CommentNotifier` to the notifier slice.
- `web/src/types/setting.ts` — `disablePRComment: boolean`.
- `web/src/components/SettingsChecks.vue` — "Comment on pull requests" switch.
- `web/src/components/__tests__/SettingsChecks.spec.ts` — assert the toggle.
- `web/src/components/__tests__/SettingsGeneral.spec.ts`, `web/src/stores/__tests__/repository.spec.ts` — add the new field to `RepoSetting` fixtures.

---

## Task 1: Comment-ID model + RepoStore methods + RepoStore mock

**Files:**
- Modify: `core/repo.go` (interface methods)
- Create: `models/pull_request_comment.go`
- Modify: `models/models.go` (migrate list)
- Modify: `mock/repo_mock.go`

- [ ] **Step 1: Add the two methods to the RepoStore interface**

In `core/repo.go`, add to the `RepoStore` interface (after `FindHook`/`UpdateHook`):

```go
	// FindPullRequestComment returns the tracked SCM comment ID for a repo's PR,
	// or 0 (with nil error) when none has been recorded.
	FindPullRequestComment(repoID uint, number int) (int, error)
	// UpdatePullRequestComment records the SCM comment ID for a repo's PR.
	UpdatePullRequestComment(repoID uint, number, commentID int) error
```

- [ ] **Step 2: Create the model + store methods**

Create `models/pull_request_comment.go`:

```go
package models

import (
	"errors"

	"gorm.io/gorm"
)

// PullRequestComment tracks the latest coverage comment posted to a pull request,
// so a new build can delete the previous comment before posting a fresh one.
type PullRequestComment struct {
	gorm.Model
	RepoID    uint `gorm:"index:idx_pr_comment,unique"`
	Number    int  `gorm:"index:idx_pr_comment,unique"`
	CommentID int
}

// FindPullRequestComment returns the tracked comment ID, or 0 if none recorded.
func (store *RepoStore) FindPullRequestComment(repoID uint, number int) (int, error) {
	session := store.DB.Session()
	c := &PullRequestComment{}
	err := session.Where(&PullRequestComment{RepoID: repoID, Number: number}).First(c).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return c.CommentID, nil
}

// UpdatePullRequestComment records the comment ID for a repo's pull request.
func (store *RepoStore) UpdatePullRequestComment(repoID uint, number, commentID int) error {
	session := store.DB.Session()
	c := &PullRequestComment{RepoID: repoID, Number: number}
	if err := session.Where(c).FirstOrCreate(c).Error; err != nil {
		return err
	}
	c.CommentID = commentID
	return session.Save(c).Error
}
```

(`RepoStore` and its `DB core.DatabaseService` field are defined in `models/repo.go`; `store.DB.Session()` returns `*gorm.DB` — same pattern as `FindHook`/`UpdateHook` there.)

- [ ] **Step 3: Register the model for migration**

In `models/models.go`, add `&PullRequestComment{},` to the `tables` slice passed to `AutoMigrate` (alongside `&RepoHook{}`).

- [ ] **Step 4: Build to verify `RepoStore` satisfies the interface**

Run: `cd /d/raynorpat/covergates && CGO_ENABLED=0 go build ./core/... ./models/...`
Expected: clean. (If `*RepoStore` no longer satisfies `core.RepoStore`, the build fails — that's the check.)

- [ ] **Step 5: Add the two methods to MockRepoStore**

In `mock/repo_mock.go`, after the `FindHook`/`UpdateHook` mock methods, add (match the existing style — `m.ctrl.T.Helper()`, comments WITHOUT trailing periods):

```go
// FindPullRequestComment mocks base method
func (m *MockRepoStore) FindPullRequestComment(arg0 uint, arg1 int) (int, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "FindPullRequestComment", arg0, arg1)
	ret0, _ := ret[0].(int)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// FindPullRequestComment indicates an expected call of FindPullRequestComment
func (mr *MockRepoStoreMockRecorder) FindPullRequestComment(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "FindPullRequestComment", reflect.TypeOf((*MockRepoStore)(nil).FindPullRequestComment), arg0, arg1)
}

// UpdatePullRequestComment mocks base method
func (m *MockRepoStore) UpdatePullRequestComment(arg0 uint, arg1, arg2 int) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdatePullRequestComment", arg0, arg1, arg2)
	ret0, _ := ret[0].(error)
	return ret0
}

// UpdatePullRequestComment indicates an expected call of UpdatePullRequestComment
func (mr *MockRepoStoreMockRecorder) UpdatePullRequestComment(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdatePullRequestComment", reflect.TypeOf((*MockRepoStore)(nil).UpdatePullRequestComment), arg0, arg1, arg2)
}
```

- [ ] **Step 6: Build the mock package**

Run: `cd /d/raynorpat/covergates && CGO_ENABLED=0 go build ./mock/...`
Expected: clean.

- [ ] **Step 7: Commit**

```bash
cd /d/raynorpat/covergates
git add core/repo.go models/pull_request_comment.go models/models.go mock/repo_mock.go
git commit -m "notify: track PR comment id for rolling coverage comments"
```

---

## Task 2: RepoSetting.DisablePRComment field

**Files:**
- Modify: `core/repo.go`

- [ ] **Step 1: Add the field**

In `core/repo.go`, add to the `RepoSetting` struct (after `CoverageDecreaseThreshold`):

```go
	// DisablePRComment turns off the coverage comment posted to pull requests.
	// Zero value (false) means comments are enabled.
	DisablePRComment bool `json:"disablePRComment"`
```

- [ ] **Step 2: Build to verify**

Run: `cd /d/raynorpat/covergates && CGO_ENABLED=0 go build ./core/...`
Expected: clean.

- [ ] **Step 3: Commit**

```bash
cd /d/raynorpat/covergates
git add core/repo.go
git commit -m "notify: add DisablePRComment repo setting"
```

---

## Task 3: MockPullRequestService

**Files:**
- Modify: `core/scm.go` (go:generate doc line)
- Modify: `mock/scm_mock.go`

- [ ] **Step 1: Add PullRequestService to the go:generate list**

In `core/scm.go`, change the generate directive to include `PullRequestService`:

```go
//go:generate mockgen -package mock -destination ../mock/scm_mock.go . SCMService,Client,GitRepoService,UserService,ContentService,GitService,WebhookService,PullRequestService
```

- [ ] **Step 2: Add MockPullRequestService to the mock file**

In `mock/scm_mock.go`, append a new mock type (match the file's existing style: `m.ctrl.T.Helper()`, comments WITHOUT trailing periods, recorder pattern). The interface is `core.PullRequestService` with `Find`, `CreateComment`, `RemoveComment`, `ListChanges`:

```go
// MockPullRequestService is a mock of PullRequestService interface
type MockPullRequestService struct {
	ctrl     *gomock.Controller
	recorder *MockPullRequestServiceMockRecorder
}

// MockPullRequestServiceMockRecorder is the mock recorder for MockPullRequestService
type MockPullRequestServiceMockRecorder struct {
	mock *MockPullRequestService
}

// NewMockPullRequestService creates a new mock instance
func NewMockPullRequestService(ctrl *gomock.Controller) *MockPullRequestService {
	mock := &MockPullRequestService{ctrl: ctrl}
	mock.recorder = &MockPullRequestServiceMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockPullRequestService) EXPECT() *MockPullRequestServiceMockRecorder {
	return m.recorder
}

// CreateComment mocks base method
func (m *MockPullRequestService) CreateComment(arg0 context.Context, arg1 *core.User, arg2 string, arg3 int, arg4 string) (int, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateComment", arg0, arg1, arg2, arg3, arg4)
	ret0, _ := ret[0].(int)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateComment indicates an expected call of CreateComment
func (mr *MockPullRequestServiceMockRecorder) CreateComment(arg0, arg1, arg2, arg3, arg4 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateComment", reflect.TypeOf((*MockPullRequestService)(nil).CreateComment), arg0, arg1, arg2, arg3, arg4)
}

// Find mocks base method
func (m *MockPullRequestService) Find(arg0 context.Context, arg1 *core.User, arg2 string, arg3 int) (*core.PullRequest, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Find", arg0, arg1, arg2, arg3)
	ret0, _ := ret[0].(*core.PullRequest)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Find indicates an expected call of Find
func (mr *MockPullRequestServiceMockRecorder) Find(arg0, arg1, arg2, arg3 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Find", reflect.TypeOf((*MockPullRequestService)(nil).Find), arg0, arg1, arg2, arg3)
}

// ListChanges mocks base method
func (m *MockPullRequestService) ListChanges(arg0 context.Context, arg1 *core.User, arg2 string, arg3 int) ([]*core.FileChange, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListChanges", arg0, arg1, arg2, arg3)
	ret0, _ := ret[0].([]*core.FileChange)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListChanges indicates an expected call of ListChanges
func (mr *MockPullRequestServiceMockRecorder) ListChanges(arg0, arg1, arg2, arg3 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListChanges", reflect.TypeOf((*MockPullRequestService)(nil).ListChanges), arg0, arg1, arg2, arg3)
}

// RemoveComment mocks base method
func (m *MockPullRequestService) RemoveComment(arg0 context.Context, arg1 *core.User, arg2 string, arg3, arg4 int) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RemoveComment", arg0, arg1, arg2, arg3, arg4)
	ret0, _ := ret[0].(error)
	return ret0
}

// RemoveComment indicates an expected call of RemoveComment
func (mr *MockPullRequestServiceMockRecorder) RemoveComment(arg0, arg1, arg2, arg3, arg4 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RemoveComment", reflect.TypeOf((*MockPullRequestService)(nil).RemoveComment), arg0, arg1, arg2, arg3, arg4)
}
```

(`context`, `core`, `gomock`, `reflect` are already imported in `mock/scm_mock.go`.)

- [ ] **Step 3: Build the mock package**

Run: `cd /d/raynorpat/covergates && CGO_ENABLED=0 go build ./mock/...`
Expected: clean.

Quick interface-conformance check (optional sanity): the mock must satisfy `core.PullRequestService`. The notifier in Task 5 will use it; if a signature is off, Task 5's test won't compile.

- [ ] **Step 4: Commit**

```bash
cd /d/raynorpat/covergates
git add core/scm.go mock/scm_mock.go
git commit -m "notify: add MockPullRequestService for comment tests"
```

---

## Task 4: Markdown body + per-file coverage (pure helpers)

**Files:**
- Create: `modules/notify/comment.go` (helpers only this task)
- Test: `modules/notify/comment_body_test.go`

- [ ] **Step 1: Write the failing tests**

Create `modules/notify/comment_body_test.go`:

```go
package notify

import (
	"strings"
	"testing"

	"github.com/covergates/covergates/core"
)

func ptr(i int) *int { return &i }

func TestCoverageByPath(t *testing.T) {
	build := &core.Build{SourceFiles: []*core.SourceFile{
		{Name: "a.go", Coverage: []*int{ptr(1), ptr(0)}}, // 1/2 = 0.5
		{Name: "b.go", Coverage: []*int{ptr(2), ptr(3)}}, // 2/2 = 1.0
		{Name: "c.go", Coverage: []*int{nil, nil}},       // no relevant lines -> -1
	}}
	cov := coverageByPath(build)
	if cov["a.go"] != 0.5 {
		t.Errorf("a.go = %v, want 0.5", cov["a.go"])
	}
	if cov["b.go"] != 1.0 {
		t.Errorf("b.go = %v, want 1.0", cov["b.go"])
	}
	if cov["c.go"] != -1 {
		t.Errorf("c.go = %v, want -1", cov["c.go"])
	}
}

func TestCommentBody(t *testing.T) {
	repo := &core.Repo{NameSpace: "o", Name: "r", SCM: core.Github}
	build := &core.Build{Number: 12, BaseBuildNumber: 9}
	verdict := &core.Verdict{State: core.StatusFailure, Description: "Coverage decreased 1.0% to 84.0%"}
	changes := []*core.FileChange{
		{Path: "a.go"},
		{Path: "c.go"},
		{Path: "gone.go", Deleted: true},
	}
	cov := map[string]float64{"a.go": 0.853, "c.go": -1}
	body := commentBody(repo, build, verdict, changes, cov, "http://h/report/github/o/r/builds/12")

	for _, want := range []string{
		"## Coverage ❌ failed",
		"Coverage decreased 1.0% to 84.0%",
		"[Build #12](http://h/report/github/o/r/builds/12)",
		"base #9",
		"### Changed files",
		"| a.go | 85.3% |",
		"| c.go | — |",
		"| gone.go | deleted |",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q\n---\n%s", want, body)
		}
	}
}

func TestCommentBodyNoChangesNoBase(t *testing.T) {
	repo := &core.Repo{NameSpace: "o", Name: "r", SCM: core.Github}
	build := &core.Build{Number: 3, BaseBuildNumber: 0}
	verdict := &core.Verdict{State: core.StatusSuccess, Description: "Coverage: 90.0%"}
	body := commentBody(repo, build, verdict, nil, map[string]float64{}, "http://h/x")

	if !strings.Contains(body, "## Coverage ✅ passed") {
		t.Errorf("missing success header:\n%s", body)
	}
	if strings.Contains(body, "base #") {
		t.Errorf("should not mention base when BaseBuildNumber is 0:\n%s", body)
	}
	if strings.Contains(body, "### Changed files") {
		t.Errorf("should not render changed-files section when there are no changes:\n%s", body)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd /d/raynorpat/covergates && CGO_ENABLED=0 go test ./modules/notify/ -run 'TestCoverageByPath|TestCommentBody'`
Expected: FAIL — `undefined: coverageByPath` / `commentBody`.

- [ ] **Step 3: Implement the helpers**

Create `modules/notify/comment.go` with ONLY the helpers for now (the `CommentNotifier` struct comes in Task 5):

```go
package notify

import (
	"fmt"
	"strings"

	"github.com/covergates/covergates/core"
)

// coverageByPath maps each build source file to its coverage ratio in [0,1],
// or -1 when the file has no relevant (executable) lines.
func coverageByPath(build *core.Build) map[string]float64 {
	m := make(map[string]float64, len(build.SourceFiles))
	for _, f := range build.SourceFiles {
		relevant, covered := 0, 0
		for _, hit := range f.Coverage {
			if hit == nil {
				continue
			}
			relevant++
			if *hit > 0 {
				covered++
			}
		}
		if relevant == 0 {
			m[f.Name] = -1
			continue
		}
		m[f.Name] = float64(covered) / float64(relevant)
	}
	return m
}

// commentBody renders the PR coverage comment markdown.
func commentBody(repo *core.Repo, build *core.Build, verdict *core.Verdict,
	changes []*core.FileChange, cov map[string]float64, target string) string {
	var b strings.Builder
	emoji, label := statusBadge(verdict.State)
	fmt.Fprintf(&b, "## Coverage %s %s\n", emoji, label)
	fmt.Fprintf(&b, "%s\n\n", verdict.Description)
	fmt.Fprintf(&b, "[Build #%d](%s)", build.Number, target)
	if build.BaseBuildNumber > 0 {
		fmt.Fprintf(&b, " · base #%d", build.BaseBuildNumber)
	}
	b.WriteString("\n")

	if len(changes) > 0 {
		b.WriteString("\n### Changed files\n")
		b.WriteString("| File | Coverage |\n| --- | --- |\n")
		for _, ch := range changes {
			fmt.Fprintf(&b, "| %s | %s |\n", ch.Path, coverageCell(ch, cov))
		}
	}
	return b.String()
}

func statusBadge(state core.StatusState) (emoji, label string) {
	switch state {
	case core.StatusSuccess:
		return "✅", "passed"
	case core.StatusFailure:
		return "❌", "failed"
	case core.StatusError:
		return "⚠️", "errored"
	default:
		return "⏳", "pending"
	}
}

func coverageCell(ch *core.FileChange, cov map[string]float64) string {
	if ch.Deleted {
		return "deleted"
	}
	if ratio, ok := cov[ch.Path]; ok && ratio >= 0 {
		return fmt.Sprintf("%.1f%%", ratio*100)
	}
	return "—"
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `cd /d/raynorpat/covergates && CGO_ENABLED=0 go test ./modules/notify/ -run 'TestCoverageByPath|TestCommentBody'`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
cd /d/raynorpat/covergates
git add modules/notify/comment.go modules/notify/comment_body_test.go
git commit -m "notify: PR coverage comment markdown builder"
```

---

## Task 5: CommentNotifier

**Files:**
- Modify: `modules/notify/comment.go` (add the struct + Notify)
- Test: `modules/notify/comment_test.go`

- [ ] **Step 1: Write the failing notifier tests**

Create `modules/notify/comment_test.go`:

```go
package notify

import (
	"context"
	"errors"
	"testing"

	"github.com/covergates/covergates/config"
	"github.com/covergates/covergates/core"
	"github.com/covergates/covergates/mock"
	"github.com/golang/mock/gomock"
)

func commentConfig() *config.Config {
	cfg := &config.Config{}
	cfg.Server.Addr = "http://localhost:8080"
	return cfg
}

func prBuild() *core.Build {
	return &core.Build{
		Number: 12, PullRequest: 7, Commit: "abc", BaseBuildNumber: 9,
		SourceFiles: []*core.SourceFile{{Name: "a.go", Coverage: []*int{ptr(1), ptr(1)}}},
	}
}

func TestCommentNotifierSkipsNonPR(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	n := &CommentNotifier{
		SCM:    mock.NewMockSCMService(ctrl),
		Repos:  mock.NewMockRepoStore(ctrl),
		Config: commentConfig(),
	}
	build := &core.Build{Number: 1, PullRequest: 0}
	if err := n.Notify(context.Background(), &core.Repo{}, build, &core.Verdict{}); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestCommentNotifierSkipsWhenDisabled(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repos := mock.NewMockRepoStore(ctrl)
	repo := &core.Repo{ID: 1, NameSpace: "o", Name: "r", SCM: core.Github}
	repos.EXPECT().Setting(repo).Return(&core.RepoSetting{DisablePRComment: true}, nil)
	n := &CommentNotifier{SCM: mock.NewMockSCMService(ctrl), Repos: repos, Config: commentConfig()}
	if err := n.Notify(context.Background(), repo, prBuild(), &core.Verdict{}); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestCommentNotifierFirstComment(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	scmService := mock.NewMockSCMService(ctrl)
	client := mock.NewMockClient(ctrl)
	prs := mock.NewMockPullRequestService(ctrl)
	repos := mock.NewMockRepoStore(ctrl)

	repo := &core.Repo{ID: 1, NameSpace: "o", Name: "r", SCM: core.Github}
	user := &core.User{Login: "octocat"}
	verdict := &core.Verdict{State: core.StatusSuccess, Description: "Coverage: 100.0%"}

	repos.EXPECT().Setting(repo).Return(&core.RepoSetting{}, nil)
	repos.EXPECT().Creator(repo).Return(user, nil)
	scmService.EXPECT().Client(repo.SCM).Return(client, nil)
	client.EXPECT().PullRequests().Return(prs).AnyTimes()
	prs.EXPECT().ListChanges(gomock.Any(), user, "o/r", 7).Return([]*core.FileChange{{Path: "a.go"}}, nil)
	repos.EXPECT().FindPullRequestComment(uint(1), 7).Return(0, nil)
	prs.EXPECT().CreateComment(gomock.Any(), user, "o/r", 7, gomock.Any()).Return(55, nil)
	repos.EXPECT().UpdatePullRequestComment(uint(1), 7, 55).Return(nil)
	// No RemoveComment expected on first comment.

	n := &CommentNotifier{SCM: scmService, Repos: repos, Config: commentConfig()}
	if err := n.Notify(context.Background(), repo, prBuild(), verdict); err != nil {
		t.Fatalf("Notify error: %v", err)
	}
}

func TestCommentNotifierReplacesPrevious(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	scmService := mock.NewMockSCMService(ctrl)
	client := mock.NewMockClient(ctrl)
	prs := mock.NewMockPullRequestService(ctrl)
	repos := mock.NewMockRepoStore(ctrl)

	repo := &core.Repo{ID: 1, NameSpace: "o", Name: "r", SCM: core.Github}
	user := &core.User{Login: "octocat"}

	repos.EXPECT().Setting(repo).Return(&core.RepoSetting{}, nil)
	repos.EXPECT().Creator(repo).Return(user, nil)
	scmService.EXPECT().Client(repo.SCM).Return(client, nil)
	client.EXPECT().PullRequests().Return(prs).AnyTimes()
	prs.EXPECT().ListChanges(gomock.Any(), user, "o/r", 7).Return(nil, nil)
	repos.EXPECT().FindPullRequestComment(uint(1), 7).Return(42, nil)
	prs.EXPECT().RemoveComment(gomock.Any(), user, "o/r", 7, 42).Return(errors.New("gone")) // swallowed
	prs.EXPECT().CreateComment(gomock.Any(), user, "o/r", 7, gomock.Any()).Return(56, nil)
	repos.EXPECT().UpdatePullRequestComment(uint(1), 7, 56).Return(nil)

	n := &CommentNotifier{SCM: scmService, Repos: repos, Config: commentConfig()}
	if err := n.Notify(context.Background(), repo, prBuild(), &core.Verdict{State: core.StatusSuccess, Description: "x"}); err != nil {
		t.Fatalf("Notify error: %v", err)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd /d/raynorpat/covergates && CGO_ENABLED=0 go test ./modules/notify/ -run TestCommentNotifier`
Expected: FAIL — `undefined: CommentNotifier`.

- [ ] **Step 3: Implement the notifier**

Add to `modules/notify/comment.go` (keep the helpers from Task 4; add the imports `context` and `config`):

```go
// CommentNotifier posts a rolling coverage comment to a build's pull request,
// using the repository creator's SCM token. go-scm has no comment edit, so it
// deletes the previously tracked comment before posting a fresh one.
type CommentNotifier struct {
	SCM    core.SCMService
	Repos  core.RepoStore
	Config *config.Config
}

// Notify posts/refreshes the PR coverage comment. No-op for non-PR builds or
// when DisablePRComment is set.
func (n *CommentNotifier) Notify(ctx context.Context, repo *core.Repo, build *core.Build, verdict *core.Verdict) error {
	if build.PullRequest <= 0 {
		return nil
	}
	setting, err := n.Repos.Setting(repo)
	if err == nil && setting != nil && setting.DisablePRComment {
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
	prs := client.PullRequests()
	changes, err := prs.ListChanges(ctx, user, repo.FullName(), build.PullRequest)
	if err != nil {
		return err
	}
	body := commentBody(repo, build, verdict, changes, coverageByPath(build), n.target(repo, build))

	if prev, err := n.Repos.FindPullRequestComment(repo.ID, build.PullRequest); err == nil && prev > 0 {
		// Best-effort delete of the previous comment; it may already be gone.
		_ = prs.RemoveComment(ctx, user, repo.FullName(), build.PullRequest, prev)
	}
	id, err := prs.CreateComment(ctx, user, repo.FullName(), build.PullRequest, body)
	if err != nil {
		return err
	}
	return n.Repos.UpdatePullRequestComment(repo.ID, build.PullRequest, id)
}

func (n *CommentNotifier) target(repo *core.Repo, build *core.Build) string {
	return fmt.Sprintf("%s/report/%s/%s/builds/%d", n.Config.Server.URL(), repo.SCM, repo.FullName(), build.Number)
}
```

Add `"context"` and `"github.com/covergates/covergates/config"` to the file's import block (it currently imports `fmt`, `strings`, `core`).

- [ ] **Step 4: Run to verify it passes**

Run: `cd /d/raynorpat/covergates && CGO_ENABLED=0 go test ./modules/notify/`
Expected: PASS (all notify tests — C1 + comment body + notifier).

- [ ] **Step 5: Commit**

```bash
cd /d/raynorpat/covergates
git add modules/notify/comment.go modules/notify/comment_test.go
git commit -m "notify: pull request coverage comment notifier"
```

---

## Task 6: Wire CommentNotifier into the dispatcher

**Files:**
- Modify: `cmd/server/inject_service.go`

- [ ] **Step 1: Append the notifier**

In `cmd/server/inject_service.go`, in `provideNotifyService`, add `CommentNotifier` to the `Notifiers` slice (after the existing `StatusNotifier`):

```go
	return &notifymod.Service{
		Repos: repoStore,
		Notifiers: []core.Notifier{
			&notifymod.StatusNotifier{
				SCM:    scmService,
				Repos:  repoStore,
				Config: config,
			},
			&notifymod.CommentNotifier{
				SCM:    scmService,
				Repos:  repoStore,
				Config: config,
			},
		},
	}
```

(The provider already receives `config`, `scmService`, `repoStore` — no signature or `wire_gen.go` change needed.)

- [ ] **Step 2: Build**

Run: `cd /d/raynorpat/covergates && CGO_ENABLED=0 go build ./core/... ./modules/... ./routers/... ./cmd/... ./mock/...`
Expected: clean.

- [ ] **Step 3: Commit**

```bash
cd /d/raynorpat/covergates
git add cmd/server/inject_service.go
git commit -m "notify: register PR comment notifier"
```

---

## Task 7: Frontend "Comment on pull requests" toggle

**Files:**
- Modify: `web/src/types/setting.ts`
- Modify: `web/src/components/SettingsChecks.vue`
- Modify: `web/src/components/__tests__/SettingsChecks.spec.ts`
- Modify: `web/src/components/__tests__/SettingsGeneral.spec.ts`
- Modify: `web/src/stores/__tests__/repository.spec.ts`

- [ ] **Step 1: Add the field to the RepoSetting type**

Edit `web/src/types/setting.ts` — add after `coverageDecreaseThreshold`:

```ts
  coverageMinimum: number
  coverageDecreaseThreshold: number
  disablePRComment: boolean
```

- [ ] **Step 2: Update the SettingsChecks test to drive the toggle**

Replace `web/src/components/__tests__/SettingsChecks.spec.ts` with (keeps the DOM-driven approach; adds the switch assertion):

```ts
import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import * as directives from 'vuetify/directives'
import SettingsChecks from '@/components/SettingsChecks.vue'
import type { RepoSetting } from '@/types/setting'

const vuetify = createVuetify({ components, directives })

const setting: RepoSetting = {
  filters: [], mergePR: false, updateAction: '', protected: false,
  coverageMinimum: 80, coverageDecreaseThreshold: 2, disablePRComment: false
}

describe('SettingsChecks', () => {
  it('emits save with edited thresholds and comment toggle', async () => {
    const w = mount(SettingsChecks, {
      props: { modelValue: setting },
      global: { plugins: [vuetify] }
    })
    const inputs = w.findAll('input[type="number"]')
    await inputs[0].setValue('90')
    await inputs[1].setValue('5')
    // The "Comment on pull requests" switch is on (checked) by default; turn it off.
    const toggle = w.find('input[type="checkbox"]')
    await toggle.setValue(false)
    await w.find('button').trigger('click')
    const events = w.emitted('save') as RepoSetting[][]
    expect(events).toBeTruthy()
    expect(events[0][0].coverageMinimum).toBe(90)
    expect(events[0][0].coverageDecreaseThreshold).toBe(5)
    expect(events[0][0].disablePRComment).toBe(true)
  })
})
```

- [ ] **Step 3: Run to verify it fails**

Run: `cd /d/raynorpat/covergates/web && npx vitest run src/components/__tests__/SettingsChecks.spec.ts`
Expected: FAIL — no checkbox/switch found, or `disablePRComment` is undefined in the payload.

- [ ] **Step 4: Add the switch to the component**

Edit `web/src/components/SettingsChecks.vue`. Add a `commentOn` ref (the inverse of `disablePRComment`), keep it in sync, and emit the inverted value on save. Full updated file:

```vue
<script setup lang="ts">
import { ref, watch } from 'vue'
import type { RepoSetting } from '@/types/setting'

const props = defineProps<{ modelValue: RepoSetting; busy?: boolean }>()
const emit = defineEmits<{ (e: 'save', value: RepoSetting): void }>()

const minimum = ref(props.modelValue.coverageMinimum ?? 0)
const decrease = ref(props.modelValue.coverageDecreaseThreshold ?? 0)
const commentOn = ref(!props.modelValue.disablePRComment)

watch(() => props.modelValue, (v) => {
  minimum.value = v.coverageMinimum ?? 0
  decrease.value = v.coverageDecreaseThreshold ?? 0
  commentOn.value = !v.disablePRComment
})

function save() {
  emit('save', {
    ...props.modelValue,
    coverageMinimum: Number(minimum.value) || 0,
    coverageDecreaseThreshold: Number(decrease.value) || 0,
    disablePRComment: !commentOn.value
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
    <v-switch v-model="commentOn" label="Comment on pull requests" color="primary" hide-details class="mb-2" />
    <v-btn color="primary" :loading="busy" @click="save">Save</v-btn>
  </v-card>
</template>
```

Note: Vuetify 3's `v-switch` renders an `<input type="checkbox">` internally, so the test's `w.find('input[type="checkbox"]')` should locate it. If that selector doesn't match in this Vuetify version, fall back to `w.find('input[role="switch"]')` or `w.find('.v-switch input')` — the test must toggle the switch off and assert `disablePRComment === true`.

- [ ] **Step 5: Run to verify it passes**

Run: `cd /d/raynorpat/covergates/web && npx vitest run src/components/__tests__/SettingsChecks.spec.ts`
Expected: PASS.

- [ ] **Step 6: Update the other RepoSetting fixtures**

The `RepoSetting` interface now requires `disablePRComment`, so the two other test fixtures must include it (otherwise `npm run build`'s type-check fails — `vitest` alone won't catch it).

In `web/src/components/__tests__/SettingsGeneral.spec.ts`, find the `modelValue` literal and add `, disablePRComment: false` before the closing brace (it already has `coverageMinimum: 0, coverageDecreaseThreshold: 0`).

In `web/src/stores/__tests__/repository.spec.ts`, find the `const s = { ... }` `RepoSetting` literal and add `, disablePRComment: false` before the closing brace.

- [ ] **Step 7: Run the full suite, lint, and type-check build**

Run:
```
cd /d/raynorpat/covergates/web && npx vitest run && npm run lint && npm run build
```
Expected: all Vitest suites pass; lint clean; `npm run build` succeeds (only the pre-existing chunk-size warning).

- [ ] **Step 8: Commit**

```bash
cd /d/raynorpat/covergates
git add web/src/types/setting.ts web/src/components/SettingsChecks.vue web/src/components/__tests__/SettingsChecks.spec.ts web/src/components/__tests__/SettingsGeneral.spec.ts web/src/stores/__tests__/repository.spec.ts
git commit -m "web: pull request comment toggle in checks settings"
```

---

## Task 8: Full verification

**Files:** none (verification only)

- [ ] **Step 1: Backend build + vet + targeted tests**

Run:
```
cd /d/raynorpat/covergates && CGO_ENABLED=0 go vet ./core/... ./modules/notify/... ./routers/api/build/... ./cmd/... 2>&1 | tail -20
CGO_ENABLED=0 go build ./core/... ./modules/... ./routers/... ./cmd/... ./mock/...
CGO_ENABLED=0 go test ./modules/notify/ ./modules/scm/ ./routers/api/build/ ./modules/build/
```
Expected: vet clean; build clean; all four packages `ok`. (`models` is excluded — it requires CGO/sqlite and cannot run in this environment.)

- [ ] **Step 2: Frontend lint, test, build**

Run:
```
cd /d/raynorpat/covergates/web && npm run lint && npx vitest run && npm run build
```
Expected: lint clean; all Vitest suites pass; production build succeeds.

- [ ] **Step 3: Final commit (only if a verification fix was needed)**

```bash
cd /d/raynorpat/covergates
git add -A
git commit -m "notify: C2 verification fixes"
```
(Skip if nothing changed.)

---

## Done criteria

- A finalized PR build posts a single coverage comment (summary + changed-files table) to the PR using the creator's token; a re-run deletes the prior comment and posts a fresh one tracked by `(repoID, number)`.
- Non-PR builds and repos with `disablePRComment` set produce no comment.
- Comment failures never affect the upload/webhook HTTP response (best-effort dispatch).
- The Checks settings card exposes a "Comment on pull requests" switch (default on).
- All targeted Go packages and the full frontend suite pass; `go vet` is clean; the frontend type-check build succeeds.
