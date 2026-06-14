# Coveralls-compatible Build/Job Model Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace covergates' per-commit upsert `Report` ingestion with a coveralls-compatible Build/Job model: a `POST /api/v1/jobs` endpoint and parallel-finalize `POST /webhook` that the official coveralls reporters can target unmodified, with per-repo build numbering, multi-job merge, and coverage deltas against a base build.

**Architecture:** New `Build` and `Job` GORM tables (additive — legacy `Report` tables stay for the existing UI). Pure merge/coverage math lives in `modules/build`; a `BuildService` orchestrates finalization (merge → coverage → base-build delta) using `BuildStore` + `SCMService`. Coveralls JSON is parsed in `routers/api/build`. A secret `Repo.Token` (separate from the public `scm/namespace/name` addressing) authenticates uploads. Once the new path works, the server-side language parsers, the native upload endpoint, and the `cmd/cli` binary are removed.

**Tech Stack:** Go 1.14, Gin, GORM (sqlite/postgres/mysql), google/wire (checked-in `wire_gen.go`, hand-edited), golang/mock (hand-written mocks — `mockgen` is not on PATH).

**Conventions for every task:**
- Run all Go commands from the repo root `D:/raynorpat/covergates` in bash.
- After each task: `go build ./...` must pass before committing.
- Commit messages use the existing imperative style (e.g. `feat:`, `refactor:`, `test:`).

---

## Phase 1 — New model foundation (additive; legacy code untouched)

### Task 1: Core build types and interfaces

**Files:**
- Create: `core/build.go`

- [ ] **Step 1: Write `core/build.go`**

```go
package core

import (
	"context"
	"time"
)

//go:generate mockgen -package mock -destination ../mock/build_mock.go . BuildStore,BuildService

// BuildStatus of a build.
type BuildStatus string

const (
	// BuildProcessing means the build is open and accepting jobs.
	BuildProcessing BuildStatus = "processing"
	// BuildDone means the build is finalized.
	BuildDone BuildStatus = "done"
	// BuildErrored means finalization failed.
	BuildErrored BuildStatus = "errored"
)

// SourceFile holds per-line coverage for a single file. Coverage index i
// corresponds to line i+1. A nil element means the line is not a relevant
// (executable) statement. The JSON tags match the coveralls payload format.
type SourceFile struct {
	Name         string `json:"name"`
	SourceDigest string `json:"source_digest"`
	Coverage     []*int `json:"coverage"`
}

// Job is a single coverage submission within a build (one parallel CI job).
type Job struct {
	ID               uint          `json:"id"`
	BuildID          uint          `json:"buildID"`
	ServiceJobID     string        `json:"serviceJobID"`
	ServiceJobNumber string        `json:"serviceJobNumber"`
	Coverage         float64       `json:"coverage"`
	SourceFiles      []*SourceFile `json:"-"`
	CreatedAt        time.Time     `json:"createdAt"`
}

// Build is a coverage build for a repository, composed of one or more jobs.
type Build struct {
	ID             uint          `json:"id"`
	RepoID         uint          `json:"repoID"`
	Number         int           `json:"number"`
	ServiceName    string        `json:"serviceName"`
	ServiceNumber  string        `json:"serviceNumber"`
	Commit         string        `json:"commit"`
	Branch         string        `json:"branch"`
	PullRequest    int           `json:"pullRequest"`
	Status         BuildStatus   `json:"status"`
	Parallel       bool          `json:"parallel"`
	Coverage       float64       `json:"coverage"`
	CoverageChange float64       `json:"coverageChange"`
	BaseBuildID    uint          `json:"baseBuildID"`
	CommitMessage  string        `json:"commitMessage"`
	AuthorName     string        `json:"authorName"`
	AuthorEmail    string        `json:"authorEmail"`
	SourceFiles    []*SourceFile `json:"sourceFiles,omitempty"`
	Jobs           []*Job        `json:"jobs,omitempty"`
	CreatedAt      time.Time     `json:"createdAt"`
	FinishedAt     time.Time     `json:"finishedAt"`
}

// BuildStore persists builds and jobs.
type BuildStore interface {
	// Create a new build, assigning the next per-repo Number.
	Create(build *Build) error
	// AddJob appends a job to an existing build.
	AddJob(build *Build, job *Job) error
	// FindByServiceNumber finds a build by repo and CI service number.
	FindByServiceNumber(repoID uint, serviceNumber string) (*Build, error)
	// FindByNumber finds a build by repo and per-repo build number, with merged source files.
	FindByNumber(repoID uint, number int) (*Build, error)
	// FindByCommit finds the most recent build for a repo and commit.
	FindByCommit(repoID uint, commit string) (*Build, error)
	// List builds for a repo, optionally filtered by branch, newest first.
	List(repoID uint, branch string, limit, offset int) ([]*Build, error)
	// Jobs of a build, each with its uploaded source files.
	Jobs(buildID uint) ([]*Job, error)
	// LatestOnBranch returns the most recent done build on a branch,
	// excluding excludeBuildID (pass 0 to exclude nothing).
	LatestOnBranch(repoID uint, branch string, excludeBuildID uint) (*Build, error)
	// Update persists changes to a build (status, coverage, merged source files).
	Update(build *Build) error
}

// BuildService orchestrates build finalization.
type BuildService interface {
	// Finalize merges all jobs into the build, computes coverage and the
	// delta against a base build, and marks the build done.
	Finalize(ctx context.Context, repo *Repo, build *Build) error
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./core/...`
Expected: no output (success).

- [ ] **Step 3: Commit**

```bash
git add core/build.go
git commit -m "feat: add core Build/Job types and store interfaces"
```

---

### Task 2: Repo upload token (core + util + repo store)

**Files:**
- Modify: `core/repo.go` (add `Token` field to `Repo`)
- Create: `modules/util/token.go`
- Create: `modules/util/token_test.go`
- Modify: `models/repo.go` (persist `Token`)

- [ ] **Step 1: Add `Token` to `core.Repo`**

In `core/repo.go`, add the `Token` field to the `Repo` struct (after `ReportID`):

```go
// Repo defined a repository structure
type Repo struct {
	ID        uint
	URL       string
	ReportID  string
	Token     string
	NameSpace string
	Name      string
	Branch    string
	Private   bool
	SCM       SCMProvider
}
```

- [ ] **Step 2: Write the failing token-generator test**

Create `modules/util/token_test.go`:

```go
package util

import "testing"

func TestGenerateToken(t *testing.T) {
	a := GenerateToken()
	b := GenerateToken()
	if a == "" {
		t.Fatal("token should not be empty")
	}
	if len(a) != 40 {
		t.Fatalf("token length = %d, want 40", len(a))
	}
	if a == b {
		t.Fatal("two generated tokens should differ")
	}
}
```

- [ ] **Step 3: Run it to verify it fails**

Run: `go test ./modules/util/ -run TestGenerateToken -v`
Expected: FAIL — `undefined: GenerateToken`.

- [ ] **Step 4: Implement `GenerateToken`**

Create `modules/util/token.go`:

```go
package util

import (
	"crypto/rand"
	"encoding/hex"
)

// GenerateToken returns a random 40-character hex secret used as a
// repository upload token.
func GenerateToken() string {
	b := make([]byte, 20)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
```

- [ ] **Step 5: Run it to verify it passes**

Run: `go test ./modules/util/ -run TestGenerateToken -v`
Expected: PASS.

- [ ] **Step 6: Persist `Token` in the repo model**

In `models/repo.go`:

(a) Add the field to the GORM `Repo` struct (after `ReportID`):

```go
	ReportID  string
	Token     string `gorm:"index"`
```

(b) In `ToCoreRepo`, add `Token`:

```go
	return &core.Repo{
		ID:        repo.ID,
		Name:      repo.Name,
		NameSpace: repo.NameSpace,
		ReportID:  repo.ReportID,
		Token:     repo.Token,
		SCM:       core.SCMProvider(repo.SCM),
		Branch:    repo.Branch,
		URL:       repo.URL,
		Private:   repo.Private,
	}
```

(c) In `copyRepo`, add `Token` so it survives `Update`:

```go
func copyRepo(dst *Repo, src *core.Repo) {
	dst.ReportID = src.ReportID
	dst.Token = src.Token
	dst.Branch = src.Branch
	dst.Private = src.Private
}
```

(d) In `Create`, generate a token when creating a repo. Add the import
`"github.com/covergates/covergates/modules/util"` and set the field:

```go
	r := &Repo{
		URL:       repo.URL,
		NameSpace: repo.NameSpace,
		Name:      repo.Name,
		SCM:       string(repo.SCM),
		Branch:    repo.Branch,
		Private:   repo.Private,
		Token:     util.GenerateToken(),
	}
	if err := session.Create(r).Error; err != nil {
		return err
	}
	repo.Token = r.Token
	repo.ID = r.ID
	return nil
```

(Replace the existing `return session.Create(r).Error` tail of `Create` with the block above.)

Note: `updateOrCreate` (used by repo sync) keeps its existing `Select(...)` list which does **not** include `Token`, so syncing never overwrites an existing token. That is intended.

- [ ] **Step 7: Verify build**

Run: `go build ./...`
Expected: success.

- [ ] **Step 8: Commit**

```bash
git add core/repo.go modules/util/token.go modules/util/token_test.go models/repo.go
git commit -m "feat: add secret repo upload token"
```

---

### Task 3: Build/Job GORM models, migration, and store

**Files:**
- Create: `models/build.go`
- Modify: `models/models.go` (register tables + token backfill)

- [ ] **Step 1: Write `models/build.go`**

```go
package models

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/covergates/covergates/core"
	"gorm.io/gorm"
)

var errBuildFields = errors.New("build must have a repository id")

// Build is a coverage build belonging to a repository.
type Build struct {
	gorm.Model
	RepoID         uint   `gorm:"index"`
	Number         int    `gorm:"index"`
	ServiceName    string
	ServiceNumber  string `gorm:"index"`
	Commit         string `gorm:"index"`
	Branch         string `gorm:"index"`
	PullRequest    int
	Status         string
	Parallel       bool
	Coverage       float64
	CoverageChange float64
	BaseBuildID    uint
	CommitMessage  string
	AuthorName     string
	AuthorEmail    string
	CoverageData   []byte
	Jobs           []*Job
	FinishedAt     time.Time
}

// Job is a single coverage submission within a build.
type Job struct {
	gorm.Model
	BuildID          uint `gorm:"index"`
	ServiceJobID     string
	ServiceJobNumber string
	Coverage         float64
	SourceFilesData  []byte
}

// BuildStore persists builds and jobs.
type BuildStore struct {
	DB core.DatabaseService
}

// Create a new build, assigning the next per-repo Number.
func (store *BuildStore) Create(b *core.Build) error {
	if b.RepoID == 0 {
		return errBuildFields
	}
	return store.DB.Session().Transaction(func(tx *gorm.DB) error {
		var result struct{ Max int }
		if err := tx.Model(&Build{}).
			Where("repo_id = ?", b.RepoID).
			Select("COALESCE(MAX(number), 0) as max").
			Scan(&result).Error; err != nil {
			return err
		}
		m := &Build{}
		copyBuildToModel(m, b)
		m.Number = result.Max + 1
		if err := tx.Create(m).Error; err != nil {
			return err
		}
		b.ID = m.ID
		b.Number = m.Number
		b.CreatedAt = m.CreatedAt
		return nil
	})
}

// AddJob appends a job to an existing build.
func (store *BuildStore) AddJob(b *core.Build, j *core.Job) error {
	data, err := json.Marshal(j.SourceFiles)
	if err != nil {
		return err
	}
	m := &Job{
		BuildID:          b.ID,
		ServiceJobID:     j.ServiceJobID,
		ServiceJobNumber: j.ServiceJobNumber,
		Coverage:         j.Coverage,
		SourceFilesData:  data,
	}
	if err := store.DB.Session().Create(m).Error; err != nil {
		return err
	}
	j.ID = m.ID
	j.BuildID = b.ID
	return nil
}

// FindByServiceNumber finds a build by repo and CI service number.
func (store *BuildStore) FindByServiceNumber(repoID uint, serviceNumber string) (*core.Build, error) {
	m := &Build{}
	err := store.DB.Session().
		Where(&Build{RepoID: repoID, ServiceNumber: serviceNumber}).
		First(m).Error
	if err != nil {
		return nil, err
	}
	return m.ToCoreBuild(false), nil
}

// FindByNumber finds a build by repo and per-repo number, with merged source files.
func (store *BuildStore) FindByNumber(repoID uint, number int) (*core.Build, error) {
	m := &Build{}
	err := store.DB.Session().
		Where(&Build{RepoID: repoID, Number: number}).
		First(m).Error
	if err != nil {
		return nil, err
	}
	return m.ToCoreBuild(true), nil
}

// FindByCommit finds the most recent build for a repo and commit.
func (store *BuildStore) FindByCommit(repoID uint, commit string) (*core.Build, error) {
	m := &Build{}
	err := store.DB.Session().
		Where(&Build{RepoID: repoID, Commit: commit}).
		Order("number desc").
		First(m).Error
	if err != nil {
		return nil, err
	}
	return m.ToCoreBuild(false), nil
}

// List builds for a repo, optionally filtered by branch, newest first.
func (store *BuildStore) List(repoID uint, branch string, limit, offset int) ([]*core.Build, error) {
	if limit <= 0 {
		limit = 30
	}
	var builds []*Build
	condition := &Build{RepoID: repoID}
	if branch != "" {
		condition.Branch = branch
	}
	err := store.DB.Session().
		Where(condition).
		Order("number desc").
		Limit(limit).Offset(offset).
		Find(&builds).Error
	if err != nil {
		return nil, err
	}
	result := make([]*core.Build, len(builds))
	for i, m := range builds {
		result[i] = m.ToCoreBuild(false)
	}
	return result, nil
}

// Jobs of a build, each with its uploaded source files.
func (store *BuildStore) Jobs(buildID uint) ([]*core.Job, error) {
	var jobs []*Job
	err := store.DB.Session().
		Where(&Job{BuildID: buildID}).
		Order("id asc").
		Find(&jobs).Error
	if err != nil {
		return nil, err
	}
	result := make([]*core.Job, len(jobs))
	for i, m := range jobs {
		result[i] = m.toCoreJob()
	}
	return result, nil
}

// LatestOnBranch returns the most recent done build on a branch,
// excluding excludeBuildID (pass 0 to exclude nothing).
func (store *BuildStore) LatestOnBranch(repoID uint, branch string, excludeBuildID uint) (*core.Build, error) {
	m := &Build{}
	session := store.DB.Session().
		Where(&Build{RepoID: repoID, Branch: branch, Status: string(core.BuildDone)})
	if excludeBuildID > 0 {
		session = session.Where("id <> ?", excludeBuildID)
	}
	if err := session.Order("number desc").First(m).Error; err != nil {
		return nil, err
	}
	return m.ToCoreBuild(false), nil
}

// Update persists changes to a build.
func (store *BuildStore) Update(b *core.Build) error {
	m := &Build{}
	if err := store.DB.Session().First(m, b.ID).Error; err != nil {
		return err
	}
	copyBuildToModel(m, b)
	if b.SourceFiles != nil {
		data, err := json.Marshal(b.SourceFiles)
		if err != nil {
			return err
		}
		m.CoverageData = data
	}
	return store.DB.Session().Save(m).Error
}

// ToCoreBuild converts the model to a core.Build. When withFiles is true the
// merged source files are unmarshaled from CoverageData.
func (m *Build) ToCoreBuild(withFiles bool) *core.Build {
	b := &core.Build{
		ID:             m.ID,
		RepoID:         m.RepoID,
		Number:         m.Number,
		ServiceName:    m.ServiceName,
		ServiceNumber:  m.ServiceNumber,
		Commit:         m.Commit,
		Branch:         m.Branch,
		PullRequest:    m.PullRequest,
		Status:         core.BuildStatus(m.Status),
		Parallel:       m.Parallel,
		Coverage:       m.Coverage,
		CoverageChange: m.CoverageChange,
		BaseBuildID:    m.BaseBuildID,
		CommitMessage:  m.CommitMessage,
		AuthorName:     m.AuthorName,
		AuthorEmail:    m.AuthorEmail,
		CreatedAt:      m.CreatedAt,
		FinishedAt:     m.FinishedAt,
	}
	if withFiles && len(m.CoverageData) > 0 {
		var files []*core.SourceFile
		if err := json.Unmarshal(m.CoverageData, &files); err == nil {
			b.SourceFiles = files
		}
	}
	return b
}

func (m *Job) toCoreJob() *core.Job {
	j := &core.Job{
		ID:               m.ID,
		BuildID:          m.BuildID,
		ServiceJobID:     m.ServiceJobID,
		ServiceJobNumber: m.ServiceJobNumber,
		Coverage:         m.Coverage,
		CreatedAt:        m.CreatedAt,
	}
	if len(m.SourceFilesData) > 0 {
		var files []*core.SourceFile
		if err := json.Unmarshal(m.SourceFilesData, &files); err == nil {
			j.SourceFiles = files
		}
	}
	return j
}

// copyBuildToModel copies scalar fields from core.Build onto the model,
// leaving gorm.Model (ID/timestamps) and CoverageData untouched.
func copyBuildToModel(dst *Build, src *core.Build) {
	dst.RepoID = src.RepoID
	dst.Number = src.Number
	dst.ServiceName = src.ServiceName
	dst.ServiceNumber = src.ServiceNumber
	dst.Commit = src.Commit
	dst.Branch = src.Branch
	dst.PullRequest = src.PullRequest
	dst.Status = string(src.Status)
	dst.Parallel = src.Parallel
	dst.Coverage = src.Coverage
	dst.CoverageChange = src.CoverageChange
	dst.BaseBuildID = src.BaseBuildID
	dst.CommitMessage = src.CommitMessage
	dst.AuthorName = src.AuthorName
	dst.AuthorEmail = src.AuthorEmail
	dst.FinishedAt = src.FinishedAt
}
```

- [ ] **Step 2: Register tables and add token backfill in `models/models.go`**

Replace the `init()` and `migrate()` portions of `models/models.go` with:

```go
func init() {
	tables = append(tables,
		&Report{},
		&ReportComment{},
		&Reference{},
		&Coverage{},
		&User{},
		&Repo{},
		&RepoSetting{},
		&RepoHook{},
		&OAuthToken{},
		&Build{},
		&Job{},
	)
}

func migrate(db *gorm.DB) error {
	if err := db.AutoMigrate(tables...); err != nil {
		return err
	}
	return backfillRepoTokens(db)
}

func backfillRepoTokens(db *gorm.DB) error {
	var repos []*Repo
	if err := db.Where("token = '' OR token IS NULL").Find(&repos).Error; err != nil {
		return err
	}
	for _, r := range repos {
		if err := db.Model(r).Update("token", util.GenerateToken()).Error; err != nil {
			return err
		}
	}
	return nil
}
```

Add the import `"github.com/covergates/covergates/modules/util"` to `models/models.go`.

- [ ] **Step 3: Verify build**

Run: `go build ./...`
Expected: success.

- [ ] **Step 4: Commit**

```bash
git add models/build.go models/models.go
git commit -m "feat: add Build/Job models, store, and token backfill"
```

---

### Task 4: Build store tests

**Files:**
- Create: `models/build_test.go`

- [ ] **Step 1: Write the store tests**

`getDatabaseService(t)` is defined in `models/models_test.go` and returns
`(*gomock.Controller, core.DatabaseService)` backed by sqlite with all tables migrated.

```go
package models

import (
	"testing"

	"github.com/covergates/covergates/core"
	"gorm.io/gorm"
)

func intPtr(v int) *int { return &v }

func TestBuildStoreCreateAssignsNumber(t *testing.T) {
	ctrl, db := getDatabaseService(t)
	defer ctrl.Finish()
	store := &BuildStore{DB: db}

	for i := 1; i <= 3; i++ {
		b := &core.Build{RepoID: 1, Commit: "c", Branch: "master", Status: core.BuildProcessing}
		if err := store.Create(b); err != nil {
			t.Fatal(err)
		}
		if b.Number != i {
			t.Fatalf("build %d got Number %d, want %d", i, b.Number, i)
		}
	}

	// A different repo restarts numbering at 1.
	other := &core.Build{RepoID: 2, Commit: "c", Branch: "master", Status: core.BuildProcessing}
	if err := store.Create(other); err != nil {
		t.Fatal(err)
	}
	if other.Number != 1 {
		t.Fatalf("other repo Number = %d, want 1", other.Number)
	}
}

func TestBuildStoreAddJobAndJobs(t *testing.T) {
	ctrl, db := getDatabaseService(t)
	defer ctrl.Finish()
	store := &BuildStore{DB: db}

	build := &core.Build{RepoID: 1, ServiceNumber: "10", Commit: "c", Status: core.BuildProcessing}
	if err := store.Create(build); err != nil {
		t.Fatal(err)
	}
	job := &core.Job{
		ServiceJobID: "job-1",
		Coverage:     0.5,
		SourceFiles: []*core.SourceFile{
			{Name: "a.go", Coverage: []*int{intPtr(1), nil, intPtr(0)}},
		},
	}
	if err := store.AddJob(build, job); err != nil {
		t.Fatal(err)
	}
	if job.ID == 0 {
		t.Fatal("job ID should be set")
	}

	jobs, err := store.Jobs(build.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 1 {
		t.Fatalf("got %d jobs, want 1", len(jobs))
	}
	if len(jobs[0].SourceFiles) != 1 || jobs[0].SourceFiles[0].Name != "a.go" {
		t.Fatal("source files did not round-trip")
	}
	if jobs[0].SourceFiles[0].Coverage[1] != nil {
		t.Fatal("nil coverage element should round-trip as nil")
	}
}

func TestBuildStoreFinders(t *testing.T) {
	ctrl, db := getDatabaseService(t)
	defer ctrl.Finish()
	store := &BuildStore{DB: db}

	build := &core.Build{RepoID: 1, ServiceNumber: "42", Commit: "abc", Branch: "master", Status: core.BuildProcessing}
	if err := store.Create(build); err != nil {
		t.Fatal(err)
	}

	if got, err := store.FindByServiceNumber(1, "42"); err != nil || got.ID != build.ID {
		t.Fatalf("FindByServiceNumber failed: %v", err)
	}
	if got, err := store.FindByNumber(1, build.Number); err != nil || got.ID != build.ID {
		t.Fatalf("FindByNumber failed: %v", err)
	}
	if got, err := store.FindByCommit(1, "abc"); err != nil || got.ID != build.ID {
		t.Fatalf("FindByCommit failed: %v", err)
	}
	if _, err := store.FindByServiceNumber(1, "nope"); err == nil {
		t.Fatal("expected error for missing service number")
	}
}

func TestBuildStoreUpdateAndLatestOnBranch(t *testing.T) {
	ctrl, db := getDatabaseService(t)
	defer ctrl.Finish()
	store := &BuildStore{DB: db}

	first := &core.Build{RepoID: 1, Commit: "c1", Branch: "master", Status: core.BuildProcessing}
	if err := store.Create(first); err != nil {
		t.Fatal(err)
	}
	first.Status = core.BuildDone
	first.Coverage = 0.8
	first.SourceFiles = []*core.SourceFile{{Name: "a.go", Coverage: []*int{intPtr(1)}}}
	if err := store.Update(first); err != nil {
		t.Fatal(err)
	}

	// Merged source files round-trip via FindByNumber(withFiles=true).
	got, err := store.FindByNumber(1, first.Number)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != core.BuildDone || got.Coverage != 0.8 {
		t.Fatal("status/coverage not persisted")
	}
	if len(got.SourceFiles) != 1 || got.SourceFiles[0].Name != "a.go" {
		t.Fatal("merged source files not persisted")
	}

	second := &core.Build{RepoID: 1, Commit: "c2", Branch: "master", Status: core.BuildProcessing}
	if err := store.Create(second); err != nil {
		t.Fatal(err)
	}

	// Latest done build on master, excluding the in-progress second build.
	latest, err := store.LatestOnBranch(1, "master", second.ID)
	if err != nil {
		t.Fatal(err)
	}
	if latest.ID != first.ID {
		t.Fatalf("LatestOnBranch returned %d, want %d", latest.ID, first.ID)
	}

	// No done build on an unknown branch.
	if _, err := store.LatestOnBranch(1, "feature", 0); err == nil || err != gorm.ErrRecordNotFound {
		t.Fatalf("expected ErrRecordNotFound, got %v", err)
	}
}
```

- [ ] **Step 2: Run the tests**

Run: `go test ./models/ -run TestBuildStore -v`
Expected: all PASS.

- [ ] **Step 3: Commit**

```bash
git add models/build_test.go
git commit -m "test: build store"
```

---

### Task 5: Merge and coverage math

**Files:**
- Create: `modules/build/merge.go`
- Create: `modules/build/merge_test.go`

- [ ] **Step 1: Write the failing tests**

Create `modules/build/merge_test.go`:

```go
package build

import (
	"testing"

	"github.com/covergates/covergates/core"
)

func p(v int) *int { return &v }

func TestMergeSumsHitsElementwise(t *testing.T) {
	jobA := []*core.SourceFile{
		{Name: "a.go", Coverage: []*int{p(1), nil, p(0)}},
	}
	jobB := []*core.SourceFile{
		{Name: "a.go", Coverage: []*int{p(2), nil, p(3)}},
		{Name: "b.go", Coverage: []*int{p(0)}},
	}

	merged := Merge([][]*core.SourceFile{jobA, jobB})

	if len(merged) != 2 {
		t.Fatalf("got %d files, want 2", len(merged))
	}
	a := merged[0]
	if a.Name != "a.go" {
		t.Fatalf("first file = %s, want a.go (insertion order)", a.Name)
	}
	if *a.Coverage[0] != 3 {
		t.Fatalf("line 1 = %d, want 3", *a.Coverage[0])
	}
	if a.Coverage[1] != nil {
		t.Fatal("line 2 should stay nil")
	}
	if *a.Coverage[2] != 3 {
		t.Fatalf("line 3 = %d, want 3", *a.Coverage[2])
	}
}

func TestMergeDoesNotMutateInput(t *testing.T) {
	jobA := []*core.SourceFile{{Name: "a.go", Coverage: []*int{p(1)}}}
	jobB := []*core.SourceFile{{Name: "a.go", Coverage: []*int{p(2)}}}
	Merge([][]*core.SourceFile{jobA, jobB})
	if *jobA[0].Coverage[0] != 1 {
		t.Fatal("Merge mutated its input")
	}
}

func TestCoverageRatio(t *testing.T) {
	files := []*core.SourceFile{
		{Name: "a.go", Coverage: []*int{p(1), nil, p(0), p(2)}}, // 3 relevant, 2 covered
	}
	got := Coverage(files)
	want := 2.0 / 3.0
	if got != want {
		t.Fatalf("coverage = %v, want %v", got, want)
	}
}

func TestCoverageEmpty(t *testing.T) {
	if got := Coverage(nil); got != 0 {
		t.Fatalf("coverage of nil = %v, want 0", got)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./modules/build/ -v`
Expected: FAIL — `undefined: Merge` / `undefined: Coverage`.

- [ ] **Step 3: Implement `modules/build/merge.go`**

```go
package build

import "github.com/covergates/covergates/core"

// Merge combines source files from multiple jobs into one set. Files are unioned
// by name (insertion order preserved); per line, the result is nil when every
// job is nil there, otherwise the sum of non-nil hit counts. Inputs are not mutated.
func Merge(jobs [][]*core.SourceFile) []*core.SourceFile {
	merged := make(map[string]*core.SourceFile)
	order := make([]string, 0)
	for _, files := range jobs {
		for _, f := range files {
			existing, ok := merged[f.Name]
			if !ok {
				merged[f.Name] = &core.SourceFile{
					Name:         f.Name,
					SourceDigest: f.SourceDigest,
					Coverage:     cloneCoverage(f.Coverage),
				}
				order = append(order, f.Name)
				continue
			}
			existing.Coverage = mergeCoverage(existing.Coverage, f.Coverage)
			if existing.SourceDigest == "" {
				existing.SourceDigest = f.SourceDigest
			}
		}
	}
	result := make([]*core.SourceFile, len(order))
	for i, name := range order {
		result[i] = merged[name]
	}
	return result
}

// Coverage computes the statement coverage ratio (0..1) over source files.
func Coverage(files []*core.SourceFile) float64 {
	var relevant, covered int
	for _, f := range files {
		for _, hit := range f.Coverage {
			if hit == nil {
				continue
			}
			relevant++
			if *hit > 0 {
				covered++
			}
		}
	}
	if relevant == 0 {
		return 0
	}
	return float64(covered) / float64(relevant)
}

func cloneCoverage(c []*int) []*int {
	out := make([]*int, len(c))
	for i, hit := range c {
		if hit != nil {
			v := *hit
			out[i] = &v
		}
	}
	return out
}

func mergeCoverage(a, b []*int) []*int {
	n := len(a)
	if len(b) > n {
		n = len(b)
	}
	out := make([]*int, n)
	for i := 0; i < n; i++ {
		var pa, pb *int
		if i < len(a) {
			pa = a[i]
		}
		if i < len(b) {
			pb = b[i]
		}
		if pa == nil && pb == nil {
			continue
		}
		sum := 0
		if pa != nil {
			sum += *pa
		}
		if pb != nil {
			sum += *pb
		}
		out[i] = &sum
	}
	return out
}
```

- [ ] **Step 4: Run to verify pass**

Run: `go test ./modules/build/ -v`
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add modules/build/merge.go modules/build/merge_test.go
git commit -m "feat: source-file merge and coverage math"
```

---

### Task 6: Hand-written mocks for BuildStore and BuildService

**Files:**
- Create: `mock/build_mock.go`

- [ ] **Step 1: Write `mock/build_mock.go`**

```go
// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/covergates/covergates/core (interfaces: BuildStore,BuildService)

package mock

import (
	context "context"
	reflect "reflect"

	core "github.com/covergates/covergates/core"
	gomock "github.com/golang/mock/gomock"
)

// MockBuildStore is a mock of BuildStore interface.
type MockBuildStore struct {
	ctrl     *gomock.Controller
	recorder *MockBuildStoreMockRecorder
}

// MockBuildStoreMockRecorder is the mock recorder for MockBuildStore.
type MockBuildStoreMockRecorder struct {
	mock *MockBuildStore
}

// NewMockBuildStore creates a new mock instance.
func NewMockBuildStore(ctrl *gomock.Controller) *MockBuildStore {
	mock := &MockBuildStore{ctrl: ctrl}
	mock.recorder = &MockBuildStoreMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockBuildStore) EXPECT() *MockBuildStoreMockRecorder {
	return m.recorder
}

// AddJob mocks base method.
func (m *MockBuildStore) AddJob(build *core.Build, job *core.Job) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AddJob", build, job)
	ret0, _ := ret[0].(error)
	return ret0
}

// AddJob indicates an expected call of AddJob.
func (mr *MockBuildStoreMockRecorder) AddJob(build, job interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddJob", reflect.TypeOf((*MockBuildStore)(nil).AddJob), build, job)
}

// Create mocks base method.
func (m *MockBuildStore) Create(build *core.Build) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Create", build)
	ret0, _ := ret[0].(error)
	return ret0
}

// Create indicates an expected call of Create.
func (mr *MockBuildStoreMockRecorder) Create(build interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Create", reflect.TypeOf((*MockBuildStore)(nil).Create), build)
}

// FindByCommit mocks base method.
func (m *MockBuildStore) FindByCommit(repoID uint, commit string) (*core.Build, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "FindByCommit", repoID, commit)
	ret0, _ := ret[0].(*core.Build)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// FindByCommit indicates an expected call of FindByCommit.
func (mr *MockBuildStoreMockRecorder) FindByCommit(repoID, commit interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "FindByCommit", reflect.TypeOf((*MockBuildStore)(nil).FindByCommit), repoID, commit)
}

// FindByNumber mocks base method.
func (m *MockBuildStore) FindByNumber(repoID uint, number int) (*core.Build, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "FindByNumber", repoID, number)
	ret0, _ := ret[0].(*core.Build)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// FindByNumber indicates an expected call of FindByNumber.
func (mr *MockBuildStoreMockRecorder) FindByNumber(repoID, number interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "FindByNumber", reflect.TypeOf((*MockBuildStore)(nil).FindByNumber), repoID, number)
}

// FindByServiceNumber mocks base method.
func (m *MockBuildStore) FindByServiceNumber(repoID uint, serviceNumber string) (*core.Build, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "FindByServiceNumber", repoID, serviceNumber)
	ret0, _ := ret[0].(*core.Build)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// FindByServiceNumber indicates an expected call of FindByServiceNumber.
func (mr *MockBuildStoreMockRecorder) FindByServiceNumber(repoID, serviceNumber interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "FindByServiceNumber", reflect.TypeOf((*MockBuildStore)(nil).FindByServiceNumber), repoID, serviceNumber)
}

// Jobs mocks base method.
func (m *MockBuildStore) Jobs(buildID uint) ([]*core.Job, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Jobs", buildID)
	ret0, _ := ret[0].([]*core.Job)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Jobs indicates an expected call of Jobs.
func (mr *MockBuildStoreMockRecorder) Jobs(buildID interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Jobs", reflect.TypeOf((*MockBuildStore)(nil).Jobs), buildID)
}

// LatestOnBranch mocks base method.
func (m *MockBuildStore) LatestOnBranch(repoID uint, branch string, excludeBuildID uint) (*core.Build, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "LatestOnBranch", repoID, branch, excludeBuildID)
	ret0, _ := ret[0].(*core.Build)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// LatestOnBranch indicates an expected call of LatestOnBranch.
func (mr *MockBuildStoreMockRecorder) LatestOnBranch(repoID, branch, excludeBuildID interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "LatestOnBranch", reflect.TypeOf((*MockBuildStore)(nil).LatestOnBranch), repoID, branch, excludeBuildID)
}

// List mocks base method.
func (m *MockBuildStore) List(repoID uint, branch string, limit, offset int) ([]*core.Build, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "List", repoID, branch, limit, offset)
	ret0, _ := ret[0].([]*core.Build)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// List indicates an expected call of List.
func (mr *MockBuildStoreMockRecorder) List(repoID, branch, limit, offset interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "List", reflect.TypeOf((*MockBuildStore)(nil).List), repoID, branch, limit, offset)
}

// Update mocks base method.
func (m *MockBuildStore) Update(build *core.Build) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Update", build)
	ret0, _ := ret[0].(error)
	return ret0
}

// Update indicates an expected call of Update.
func (mr *MockBuildStoreMockRecorder) Update(build interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Update", reflect.TypeOf((*MockBuildStore)(nil).Update), build)
}

// MockBuildService is a mock of BuildService interface.
type MockBuildService struct {
	ctrl     *gomock.Controller
	recorder *MockBuildServiceMockRecorder
}

// MockBuildServiceMockRecorder is the mock recorder for MockBuildService.
type MockBuildServiceMockRecorder struct {
	mock *MockBuildService
}

// NewMockBuildService creates a new mock instance.
func NewMockBuildService(ctrl *gomock.Controller) *MockBuildService {
	mock := &MockBuildService{ctrl: ctrl}
	mock.recorder = &MockBuildServiceMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockBuildService) EXPECT() *MockBuildServiceMockRecorder {
	return m.recorder
}

// Finalize mocks base method.
func (m *MockBuildService) Finalize(ctx context.Context, repo *core.Repo, build *core.Build) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Finalize", ctx, repo, build)
	ret0, _ := ret[0].(error)
	return ret0
}

// Finalize indicates an expected call of Finalize.
func (mr *MockBuildServiceMockRecorder) Finalize(ctx, repo, build interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Finalize", reflect.TypeOf((*MockBuildService)(nil).Finalize), ctx, repo, build)
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./mock/...`
Expected: success.

- [ ] **Step 3: Commit**

```bash
git add mock/build_mock.go
git commit -m "test: hand-written mocks for BuildStore and BuildService"
```

---

### Task 7: Build finalization service

**Files:**
- Create: `modules/build/service.go`
- Create: `modules/build/service_test.go`

- [ ] **Step 1: Write the failing service tests**

Create `modules/build/service_test.go`. It uses the existing `mock` package
(`MockBuildStore`, `MockRepoStore`, `MockSCMService`, `MockClient`). There is **no**
generated mock for `core.PullRequestService`, so the PR test defines a tiny
hand-written `fakePRService` and returns it from `MockClient.PullRequests()`.

```go
package build

import (
	"context"
	"testing"

	"github.com/covergates/covergates/core"
	"github.com/covergates/covergates/mock"
	"github.com/golang/mock/gomock"
	"gorm.io/gorm"
)

// fakePRService is a minimal core.PullRequestService stub; only Find is exercised.
type fakePRService struct{ target string }

func (f *fakePRService) Find(ctx context.Context, user *core.User, repo string, number int) (*core.PullRequest, error) {
	return &core.PullRequest{Number: number, Target: f.target}, nil
}
func (f *fakePRService) CreateComment(ctx context.Context, user *core.User, repo string, number int, body string) (int, error) {
	return 0, nil
}
func (f *fakePRService) RemoveComment(ctx context.Context, user *core.User, repo string, number int, id int) error {
	return nil
}
func (f *fakePRService) ListChanges(ctx context.Context, user *core.User, repo string, number int) ([]*core.FileChange, error) {
	return nil, nil
}

func TestFinalizePushBuildComputesDelta(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	builds := mock.NewMockBuildStore(ctrl)
	repos := mock.NewMockRepoStore(ctrl)
	scm := mock.NewMockSCMService(ctrl)

	repo := &core.Repo{ID: 1, Branch: "master"}
	build := &core.Build{ID: 5, RepoID: 1, Branch: "master", Number: 2, Status: core.BuildProcessing}

	builds.EXPECT().Jobs(uint(5)).Return([]*core.Job{
		{SourceFiles: []*core.SourceFile{{Name: "a.go", Coverage: []*int{p(1), p(0)}}}},
	}, nil)
	builds.EXPECT().LatestOnBranch(uint(1), "master", uint(5)).Return(
		&core.Build{ID: 1, Coverage: 0.25}, nil,
	)
	builds.EXPECT().Update(gomock.Any()).DoAndReturn(func(b *core.Build) error {
		if b.Status != core.BuildDone {
			t.Fatal("status should be done")
		}
		if b.Coverage != 0.5 {
			t.Fatalf("coverage = %v, want 0.5", b.Coverage)
		}
		if b.CoverageChange != 0.25 {
			t.Fatalf("coverageChange = %v, want 0.25", b.CoverageChange)
		}
		if b.BaseBuildID != 1 {
			t.Fatalf("baseBuildID = %d, want 1", b.BaseBuildID)
		}
		return nil
	})

	svc := &Service{Builds: builds, Repos: repos, SCM: scm}
	if err := svc.Finalize(context.Background(), repo, build); err != nil {
		t.Fatal(err)
	}
}

func TestFinalizeNoBaseBuild(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	builds := mock.NewMockBuildStore(ctrl)
	repos := mock.NewMockRepoStore(ctrl)
	scm := mock.NewMockSCMService(ctrl)

	repo := &core.Repo{ID: 1, Branch: "master"}
	build := &core.Build{ID: 5, RepoID: 1, Branch: "master", Status: core.BuildProcessing}

	builds.EXPECT().Jobs(uint(5)).Return([]*core.Job{
		{SourceFiles: []*core.SourceFile{{Name: "a.go", Coverage: []*int{p(1)}}}},
	}, nil)
	builds.EXPECT().LatestOnBranch(uint(1), "master", uint(5)).Return(nil, gorm.ErrRecordNotFound)
	builds.EXPECT().Update(gomock.Any()).DoAndReturn(func(b *core.Build) error {
		if b.CoverageChange != 0 || b.BaseBuildID != 0 {
			t.Fatal("no base build means zero delta")
		}
		return nil
	})

	svc := &Service{Builds: builds, Repos: repos, SCM: scm}
	if err := svc.Finalize(context.Background(), repo, build); err != nil {
		t.Fatal(err)
	}
}

func TestFinalizePullRequestUsesBaseBranch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	builds := mock.NewMockBuildStore(ctrl)
	repos := mock.NewMockRepoStore(ctrl)
	scm := mock.NewMockSCMService(ctrl)
	client := mock.NewMockClient(ctrl)

	repo := &core.Repo{ID: 1, NameSpace: "o", Name: "r", Branch: "master", SCM: core.Github}
	build := &core.Build{ID: 7, RepoID: 1, Branch: "feature", PullRequest: 3, Status: core.BuildProcessing}

	builds.EXPECT().Jobs(uint(7)).Return([]*core.Job{
		{SourceFiles: []*core.SourceFile{{Name: "a.go", Coverage: []*int{p(1)}}}},
	}, nil)
	repos.EXPECT().Creator(repo).Return(&core.User{Login: "u"}, nil)
	scm.EXPECT().Client(core.Github).Return(client, nil)
	client.EXPECT().PullRequests().Return(&fakePRService{target: "develop"})
	builds.EXPECT().LatestOnBranch(uint(1), "develop", uint(7)).Return(
		&core.Build{ID: 2, Coverage: 1.0}, nil,
	)
	builds.EXPECT().Update(gomock.Any()).Return(nil)

	svc := &Service{Builds: builds, Repos: repos, SCM: scm}
	if err := svc.Finalize(context.Background(), repo, build); err != nil {
		t.Fatal(err)
	}
}
```

The SCM provider constant `core.Github` is defined in `core/const.go`.

- [ ] **Step 2: Run to verify failure**

Run: `go test ./modules/build/ -run TestFinalize -v`
Expected: FAIL — `undefined: Service`.

- [ ] **Step 3: Implement `modules/build/service.go`**

```go
package build

import (
	"context"

	"github.com/covergates/covergates/core"
)

// Service implements core.BuildService.
type Service struct {
	Builds core.BuildStore
	Repos  core.RepoStore
	SCM    core.SCMService
}

// Finalize merges all jobs into the build, computes coverage and the delta
// against a base build, and marks the build done.
func (s *Service) Finalize(ctx context.Context, repo *core.Repo, build *core.Build) error {
	jobs, err := s.Builds.Jobs(build.ID)
	if err != nil {
		return err
	}
	sets := make([][]*core.SourceFile, len(jobs))
	for i, j := range jobs {
		sets[i] = j.SourceFiles
	}
	merged := Merge(sets)
	build.SourceFiles = merged
	build.Coverage = Coverage(merged)

	if base, err := s.baseBuild(ctx, repo, build); err == nil && base != nil {
		build.CoverageChange = build.Coverage - base.Coverage
		build.BaseBuildID = base.ID
	}

	build.Status = core.BuildDone
	return s.Builds.Update(build)
}

func (s *Service) baseBuild(ctx context.Context, repo *core.Repo, build *core.Build) (*core.Build, error) {
	branch := build.Branch
	if build.PullRequest > 0 {
		if target := s.pullRequestBase(ctx, repo, build.PullRequest); target != "" {
			branch = target
		} else if repo.Branch != "" {
			branch = repo.Branch
		}
	}
	return s.Builds.LatestOnBranch(repo.ID, branch, build.ID)
}

func (s *Service) pullRequestBase(ctx context.Context, repo *core.Repo, number int) string {
	user, err := s.Repos.Creator(repo)
	if err != nil {
		return ""
	}
	client, err := s.SCM.Client(repo.SCM)
	if err != nil {
		return ""
	}
	pr, err := client.PullRequests().Find(ctx, user, repo.FullName(), number)
	if err != nil {
		return ""
	}
	return pr.Target
}
```

- [ ] **Step 4: Run to verify pass**

Run: `go test ./modules/build/ -v`
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add modules/build/service.go modules/build/service_test.go
git commit -m "feat: build finalization service with delta against base build"
```

---

## Phase 2 — Coveralls upload endpoints (additive)

### Task 8: Coveralls payload parsing

**Files:**
- Create: `routers/api/build/payload.go`
- Create: `routers/api/build/payload_test.go`

- [ ] **Step 1: Write the failing parser tests**

Create `routers/api/build/payload_test.go`:

```go
package build

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func ginContext(req *http.Request) *gin.Context {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	return c
}

func TestParsePayloadRawJSON(t *testing.T) {
	body := `{"repo_token":"abc","service_number":"10","parallel":true,
		"git":{"branch":"master","head":{"id":"sha1","message":"msg","author_name":"a","author_email":"e"}},
		"source_files":[{"name":"a.go","coverage":[1,null,0]}]}`
	req := httptest.NewRequest("POST", "/api/v1/jobs", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	payload, err := parsePayload(ginContext(req))
	if err != nil {
		t.Fatal(err)
	}
	if payload.RepoToken != "abc" || payload.ServiceNumber != "10" || !payload.Parallel {
		t.Fatal("scalar fields not parsed")
	}
	if payload.Git == nil || payload.Git.Branch != "master" || payload.Git.Head.ID != "sha1" {
		t.Fatal("git fields not parsed")
	}
	if len(payload.SourceFiles) != 1 || payload.SourceFiles[0].Name != "a.go" {
		t.Fatal("source files not parsed")
	}
	cov := payload.SourceFiles[0].Coverage
	if *cov[0] != 1 || cov[1] != nil || *cov[2] != 0 {
		t.Fatal("coverage array not parsed (null must become nil)")
	}
}

func TestParsePayloadMultipartJSONFile(t *testing.T) {
	buf := &bytes.Buffer{}
	w := multipart.NewWriter(buf)
	part, _ := w.CreateFormFile("json_file", "coverage.json")
	part.Write([]byte(`{"repo_token":"xyz","source_files":[{"name":"b.go","coverage":[1]}]}`))
	w.Close()

	req := httptest.NewRequest("POST", "/api/v1/jobs", buf)
	req.Header.Set("Content-Type", w.FormDataContentType())

	payload, err := parsePayload(ginContext(req))
	if err != nil {
		t.Fatal(err)
	}
	if payload.RepoToken != "xyz" || len(payload.SourceFiles) != 1 {
		t.Fatal("multipart json_file not parsed")
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./routers/api/build/ -v`
Expected: FAIL — `undefined: parsePayload`.

- [ ] **Step 3: Implement `routers/api/build/payload.go`**

```go
package build

import (
	"encoding/json"
	"io/ioutil"

	"github.com/covergates/covergates/core"
	"github.com/gin-gonic/gin"
)

// coverallsPayload is the JSON body sent by coveralls reporters to /api/v1/jobs.
type coverallsPayload struct {
	RepoToken          string             `json:"repo_token"`
	ServiceName        string             `json:"service_name"`
	ServiceNumber      string             `json:"service_number"`
	ServiceJobID       string             `json:"service_job_id"`
	ServiceJobNumber   string             `json:"service_job_number"`
	ServicePullRequest string             `json:"service_pull_request"`
	Parallel           bool               `json:"parallel"`
	FlagName           string             `json:"flag_name"`
	Git                *gitInfo           `json:"git"`
	SourceFiles        []*core.SourceFile `json:"source_files"`
}

type gitInfo struct {
	Head   gitHead `json:"head"`
	Branch string  `json:"branch"`
}

type gitHead struct {
	ID          string `json:"id"`
	Message     string `json:"message"`
	AuthorName  string `json:"author_name"`
	AuthorEmail string `json:"author_email"`
}

// parsePayload reads the coveralls payload from either the multipart "json_file"
// form field or a raw JSON request body.
func parsePayload(c *gin.Context) (*coverallsPayload, error) {
	var data []byte
	if file, err := c.FormFile("json_file"); err == nil {
		f, err := file.Open()
		if err != nil {
			return nil, err
		}
		defer f.Close()
		if data, err = ioutil.ReadAll(f); err != nil {
			return nil, err
		}
	} else {
		body, err := ioutil.ReadAll(c.Request.Body)
		if err != nil {
			return nil, err
		}
		data = body
	}
	payload := &coverallsPayload{}
	if err := json.Unmarshal(data, payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func (p *coverallsPayload) commit() string {
	if p.Git != nil {
		return p.Git.Head.ID
	}
	return ""
}

func (p *coverallsPayload) branch() string {
	if p.Git != nil {
		return p.Git.Branch
	}
	return ""
}
```

- [ ] **Step 4: Run to verify pass**

Run: `go test ./routers/api/build/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add routers/api/build/payload.go routers/api/build/payload_test.go
git commit -m "feat: parse coveralls upload payload"
```

---

### Task 9: Upload and webhook handlers

**Files:**
- Create: `routers/api/build/build.go`
- Create: `routers/api/build/build_test.go`

- [ ] **Step 1: Implement `routers/api/build/build.go`**

```go
package build

import (
	"fmt"
	"strconv"

	"github.com/covergates/covergates/config"
	"github.com/covergates/covergates/core"
	buildmod "github.com/covergates/covergates/modules/build"
	"github.com/covergates/covergates/routers/api/request"
	"github.com/gin-gonic/gin"
)

// HandleJobs ingests a coveralls job submission.
// @Summary Upload a coverage job (coveralls-compatible)
// @Tags Build
// @Success 200 {object} string "job created"
// @Router /jobs [post]
func HandleJobs(
	cfg *config.Config,
	repoStore core.RepoStore,
	buildStore core.BuildStore,
	buildService core.BuildService,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		payload, err := parsePayload(c)
		if err != nil {
			c.JSON(422, gin.H{"message": "invalid payload"})
			return
		}
		if payload.RepoToken == "" || len(payload.SourceFiles) == 0 {
			c.JSON(422, gin.H{"message": "missing repo_token or source_files"})
			return
		}
		repo, err := repoStore.Find(&core.Repo{Token: payload.RepoToken})
		if err != nil {
			c.JSON(422, gin.H{"message": "repository not found for token"})
			return
		}
		build, err := resolveBuild(buildStore, repo, payload)
		if err != nil {
			c.JSON(500, gin.H{"message": err.Error()})
			return
		}
		job := &core.Job{
			ServiceJobID:     payload.ServiceJobID,
			ServiceJobNumber: payload.ServiceJobNumber,
			SourceFiles:      payload.SourceFiles,
			Coverage:         buildmod.Coverage(payload.SourceFiles),
		}
		if err := buildStore.AddJob(build, job); err != nil {
			c.JSON(500, gin.H{"message": err.Error()})
			return
		}
		if !payload.Parallel {
			if err := buildService.Finalize(c.Request.Context(), repo, build); err != nil {
				c.JSON(500, gin.H{"message": err.Error()})
				return
			}
		}
		url := fmt.Sprintf("%s/report/%s/%s", cfg.Server.URL(), repo.SCM, repo.FullName())
		c.JSON(200, gin.H{"id": job.ID, "url": url, "message": "Job created"})
	}
}

type webhookBody struct {
	Payload struct {
		BuildNum string `json:"build_num"`
		Status   string `json:"status"`
	} `json:"payload"`
}

// HandleWebhook finalizes a parallel build.
// @Summary Finalize a parallel coverage build (coveralls-compatible)
// @Tags Build
// @Success 200 {object} string "done"
// @Router /webhook [post]
func HandleWebhook(
	repoStore core.RepoStore,
	buildStore core.BuildStore,
	buildService core.BuildService,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.Query("repo_token")
		if token == "" {
			c.JSON(422, gin.H{"message": "missing repo_token"})
			return
		}
		repo, err := repoStore.Find(&core.Repo{Token: token})
		if err != nil {
			c.JSON(422, gin.H{"message": "repository not found for token"})
			return
		}
		body := &webhookBody{}
		if err := c.BindJSON(body); err != nil {
			c.JSON(422, gin.H{"message": "invalid payload"})
			return
		}
		build, err := buildStore.FindByServiceNumber(repo.ID, body.Payload.BuildNum)
		if err != nil {
			c.JSON(422, gin.H{"message": "build not found"})
			return
		}
		if err := buildService.Finalize(c.Request.Context(), repo, build); err != nil {
			c.JSON(500, gin.H{"message": err.Error()})
			return
		}
		c.JSON(200, gin.H{"done": true})
	}
}

// HandleList lists builds for a repository.
// @Summary List builds for a repository
// @Tags Build
// @Router /repos/{scm}/{namespace}/{name}/builds [get]
func HandleList(
	scmService core.SCMService,
	repoStore core.RepoStore,
	buildStore core.BuildStore,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		repo, ok := permittedRepo(c, scmService, repoStore)
		if !ok {
			return
		}
		builds, err := buildStore.List(repo.ID, c.Query("branch"), queryInt(c, "limit", 30), queryInt(c, "offset", 0))
		if err != nil {
			c.JSON(404, []*core.Build{})
			return
		}
		c.JSON(200, builds)
	}
}

// HandleGet returns a single build with merged source files.
// @Summary Get a build by number
// @Tags Build
// @Router /repos/{scm}/{namespace}/{name}/builds/{number} [get]
func HandleGet(
	scmService core.SCMService,
	repoStore core.RepoStore,
	buildStore core.BuildStore,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		repo, ok := permittedRepo(c, scmService, repoStore)
		if !ok {
			return
		}
		number, err := strconv.Atoi(c.Param("number"))
		if err != nil {
			c.JSON(400, gin.H{"message": "invalid build number"})
			return
		}
		build, err := buildStore.FindByNumber(repo.ID, number)
		if err != nil {
			c.JSON(404, gin.H{"message": "build not found"})
			return
		}
		if jobs, err := buildStore.Jobs(build.ID); err == nil {
			for _, j := range jobs {
				j.SourceFiles = nil
			}
			build.Jobs = jobs
		}
		c.JSON(200, build)
	}
}

// resolveBuild finds the build for this payload or creates a new one.
func resolveBuild(store core.BuildStore, repo *core.Repo, payload *coverallsPayload) (*core.Build, error) {
	if payload.ServiceNumber != "" {
		if build, err := store.FindByServiceNumber(repo.ID, payload.ServiceNumber); err == nil {
			return build, nil
		}
	} else if commit := payload.commit(); commit != "" {
		if build, err := store.FindByCommit(repo.ID, commit); err == nil {
			return build, nil
		}
	}
	pr := 0
	if payload.ServicePullRequest != "" {
		pr, _ = strconv.Atoi(payload.ServicePullRequest)
	}
	build := &core.Build{
		RepoID:        repo.ID,
		ServiceName:   payload.ServiceName,
		ServiceNumber: payload.ServiceNumber,
		Commit:        payload.commit(),
		Branch:        payload.branch(),
		PullRequest:   pr,
		Parallel:      payload.Parallel,
		Status:        core.BuildProcessing,
	}
	if payload.Git != nil {
		build.CommitMessage = payload.Git.Head.Message
		build.AuthorName = payload.Git.Head.AuthorName
		build.AuthorEmail = payload.Git.Head.AuthorEmail
	}
	if err := store.Create(build); err != nil {
		return nil, err
	}
	return build, nil
}

// permittedRepo resolves the repo from the path and enforces read permission:
// public repos are readable by anyone; private repos require a context user.
// This mirrors the existing report read-route behavior.
func permittedRepo(c *gin.Context, scmService core.SCMService, repoStore core.RepoStore) (*core.Repo, bool) {
	repo, err := repoStore.Find(&core.Repo{
		NameSpace: c.Param("namespace"),
		Name:      c.Param("name"),
		SCM:       core.SCMProvider(c.Param("scm")),
	})
	if err != nil {
		c.JSON(404, gin.H{"message": "repository not found"})
		return nil, false
	}
	if !repo.Private {
		return repo, true
	}
	user, ok := request.UserFrom(c)
	if !ok {
		c.JSON(401, gin.H{"message": "unauthorized"})
		return nil, false
	}
	client, err := scmService.Client(repo.SCM)
	if err != nil {
		c.JSON(500, gin.H{"message": err.Error()})
		return nil, false
	}
	if _, err := client.Repositories().Find(c.Request.Context(), user, repo.FullName()); err != nil {
		c.JSON(401, gin.H{"message": "unauthorized"})
		return nil, false
	}
	return repo, true
}

func queryInt(c *gin.Context, key string, def int) int {
	if v, err := strconv.Atoi(c.Query(key)); err == nil {
		return v
	}
	return def
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./routers/api/build/...`
Expected: success.

- [ ] **Step 3: Write handler tests**

Create `routers/api/build/build_test.go`. The config's `Server.URL()` is used in
the success response; construct a minimal `*config.Config`.

```go
package build

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/covergates/covergates/config"
	"github.com/covergates/covergates/core"
	"github.com/covergates/covergates/mock"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"gorm.io/gorm"
)

func newConfig() *config.Config {
	cfg := &config.Config{}
	cfg.Server.Addr = "http://localhost:8080"
	return cfg
}

func serve(r *gin.Engine, req *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestHandleJobsSingleFinalizes(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repos := mock.NewMockRepoStore(ctrl)
	builds := mock.NewMockBuildStore(ctrl)
	svc := mock.NewMockBuildService(ctrl)

	repo := &core.Repo{ID: 1, NameSpace: "o", Name: "r", SCM: core.Github}
	repos.EXPECT().Find(&core.Repo{Token: "tok"}).Return(repo, nil)
	builds.EXPECT().FindByServiceNumber(uint(1), "5").Return(nil, gorm.ErrRecordNotFound)
	builds.EXPECT().Create(gomock.Any()).DoAndReturn(func(b *core.Build) error { b.ID = 9; return nil })
	builds.EXPECT().AddJob(gomock.Any(), gomock.Any()).DoAndReturn(func(b *core.Build, j *core.Job) error { j.ID = 3; return nil })
	svc.EXPECT().Finalize(gomock.Any(), repo, gomock.Any()).Return(nil)

	r := gin.New()
	r.POST("/api/v1/jobs", HandleJobs(newConfig(), repos, builds, svc))

	body := `{"repo_token":"tok","service_number":"5",
		"git":{"branch":"master","head":{"id":"sha"}},
		"source_files":[{"name":"a.go","coverage":[1,0]}]}`
	req := httptest.NewRequest("POST", "/api/v1/jobs", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := serve(r, req)
	if w.Code != 200 {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
}

func TestHandleJobsParallelDoesNotFinalize(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repos := mock.NewMockRepoStore(ctrl)
	builds := mock.NewMockBuildStore(ctrl)
	svc := mock.NewMockBuildService(ctrl)

	repo := &core.Repo{ID: 1, NameSpace: "o", Name: "r", SCM: core.Github}
	repos.EXPECT().Find(&core.Repo{Token: "tok"}).Return(repo, nil)
	builds.EXPECT().FindByServiceNumber(uint(1), "5").Return(&core.Build{ID: 9, RepoID: 1}, nil)
	builds.EXPECT().AddJob(gomock.Any(), gomock.Any()).Return(nil)
	// No Finalize expected.

	r := gin.New()
	r.POST("/api/v1/jobs", HandleJobs(newConfig(), repos, builds, svc))
	body := `{"repo_token":"tok","service_number":"5","parallel":true,
		"source_files":[{"name":"a.go","coverage":[1]}]}`
	req := httptest.NewRequest("POST", "/api/v1/jobs", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	if w := serve(r, req); w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestHandleJobsBadToken(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repos := mock.NewMockRepoStore(ctrl)
	builds := mock.NewMockBuildStore(ctrl)
	svc := mock.NewMockBuildService(ctrl)

	repos.EXPECT().Find(&core.Repo{Token: "bad"}).Return(nil, gorm.ErrRecordNotFound)

	r := gin.New()
	r.POST("/api/v1/jobs", HandleJobs(newConfig(), repos, builds, svc))
	body := `{"repo_token":"bad","source_files":[{"name":"a.go","coverage":[1]}]}`
	req := httptest.NewRequest("POST", "/api/v1/jobs", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	if w := serve(r, req); w.Code != 422 {
		t.Fatalf("status = %d, want 422", w.Code)
	}
}

func TestHandleWebhookDone(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repos := mock.NewMockRepoStore(ctrl)
	builds := mock.NewMockBuildStore(ctrl)
	svc := mock.NewMockBuildService(ctrl)

	repo := &core.Repo{ID: 1}
	repos.EXPECT().Find(&core.Repo{Token: "tok"}).Return(repo, nil)
	builds.EXPECT().FindByServiceNumber(uint(1), "5").Return(&core.Build{ID: 9, RepoID: 1}, nil)
	svc.EXPECT().Finalize(gomock.Any(), repo, gomock.Any()).Return(nil)

	r := gin.New()
	r.POST("/webhook", HandleWebhook(repos, builds, svc))
	body := `{"payload":{"build_num":"5","status":"done"}}`
	req := httptest.NewRequest("POST", "/webhook?repo_token=tok", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	if w := serve(r, req); w.Code != 200 {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
}

func TestHandleWebhookUnknownBuild(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repos := mock.NewMockRepoStore(ctrl)
	builds := mock.NewMockBuildStore(ctrl)
	svc := mock.NewMockBuildService(ctrl)

	repos.EXPECT().Find(&core.Repo{Token: "tok"}).Return(&core.Repo{ID: 1}, nil)
	builds.EXPECT().FindByServiceNumber(uint(1), "9").Return(nil, gorm.ErrRecordNotFound)

	r := gin.New()
	r.POST("/webhook", HandleWebhook(repos, builds, svc))
	body := `{"payload":{"build_num":"9","status":"done"}}`
	req := httptest.NewRequest("POST", "/webhook?repo_token=tok", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	if w := serve(r, req); w.Code != 422 {
		t.Fatalf("status = %d, want 422", w.Code)
	}
}

func TestHandleListPublicRepo(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	scm := mock.NewMockSCMService(ctrl)
	repos := mock.NewMockRepoStore(ctrl)
	builds := mock.NewMockBuildStore(ctrl)

	repos.EXPECT().Find(gomock.Any()).Return(&core.Repo{ID: 1, Private: false}, nil)
	builds.EXPECT().List(uint(1), "", 30, 0).Return([]*core.Build{{Number: 1}}, nil)

	r := gin.New()
	r.GET("/api/v1/repos/:scm/:namespace/:name/builds", HandleList(scm, repos, builds))
	req := httptest.NewRequest("GET", "/api/v1/repos/github/o/r/builds", nil)
	if w := serve(r, req); w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
}
```

- [ ] **Step 4: Run the tests**

Run: `go test ./routers/api/build/ -v`
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add routers/api/build/build.go routers/api/build/build_test.go
git commit -m "feat: coveralls jobs upload, webhook, and build read handlers"
```

---

### Task 10: Token rotation endpoint

**Files:**
- Modify: `routers/api/repo/repo.go` (add `HandleTokenRenew`)

- [ ] **Step 1: Add `HandleTokenRenew`**

Add to `routers/api/repo/repo.go` (import `"github.com/covergates/covergates/modules/util"` if not present), modeled on `HandleReportIDRenew`:

```go
// HandleTokenRenew generates a new upload token for the repository
// @Summary renew repository upload token
// @Tags Repository
// @Param scm path string true "SCM"
// @Param namespace path string true "Namespace"
// @Param name path string true "name"
// @Success 200 {object} core.Repo "updated repository"
// @Router /repos/{scm}/{namespace}/{name}/token [patch]
func HandleTokenRenew(store core.RepoStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		user := request.MustGetUserFrom(c)
		repo, err := store.Find(&core.Repo{
			Name:      c.Param("name"),
			NameSpace: c.Param("namespace"),
			SCM:       core.SCMProvider(c.Param("scm")),
		})
		if err != nil {
			c.String(500, err.Error())
			return
		}
		repo.Token = util.GenerateToken()
		if err := store.Update(repo); err != nil {
			c.Error(err)
			c.String(500, err.Error())
			return
		}
		if err := store.UpdateCreator(repo, user); err != nil {
			c.Error(err)
			c.String(500, err.Error())
			return
		}
		c.JSON(200, repo)
	}
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./routers/api/repo/...`
Expected: success.

- [ ] **Step 3: Commit**

```bash
git add routers/api/repo/repo.go
git commit -m "feat: upload token rotation endpoint"
```

---

### Task 11: Wire BuildStore/BuildService and register routes

**Files:**
- Modify: `cmd/server/inject_store.go`
- Modify: `cmd/server/inject_service.go`
- Modify: `cmd/server/inject_router.go`
- Modify: `cmd/server/wire_gen.go`
- Modify: `routers/init.go`
- Modify: `routers/api/api.go`

This task only **adds** the new providers/fields/routes. `CoverageService` removal happens in Phase 3.

- [ ] **Step 1: Add the build store provider**

In `cmd/server/inject_store.go`, add to `storeSet` and define the provider:

```go
var storeSet = wire.NewSet(
	provideDatabaseService,
	provideUserStore,
	provideReportStore,
	provideRepoStore,
	provideOAuthStore,
	provideBuildStore,
)

func provideBuildStore(db core.DatabaseService) core.BuildStore {
	return &models.BuildStore{
		DB: db,
	}
}
```

- [ ] **Step 2: Add the build service provider**

In `cmd/server/inject_service.go`, add the import
`buildmod "github.com/covergates/covergates/modules/build"`, add `provideBuildService`
to `serviceSet`, and define:

```go
func provideBuildService(
	buildStore core.BuildStore,
	repoStore core.RepoStore,
	scmService core.SCMService,
) core.BuildService {
	return &buildmod.Service{
		Builds: buildStore,
		Repos:  repoStore,
		SCM:    scmService,
	}
}
```

- [ ] **Step 3: Extend `routers.Routers` and `api.Router`**

In `routers/init.go`, add fields to `Routers` (under `// store`):

```go
	BuildStore   core.BuildStore
	BuildService core.BuildService
```

and pass them to `apiRoute`:

```go
		BuildStore:   r.BuildStore,
		BuildService: r.BuildService,
```

In `routers/api/api.go`, add to the `Router` struct:

```go
	BuildService core.BuildService
```
(under `// service`) and

```go
	BuildStore  core.BuildStore
```
(under `// store`).

- [ ] **Step 4: Register routes in `routers/api/api.go`**

Add the import `"github.com/covergates/covergates/routers/api/build"`.

Inside `RegisterRoutes`, register the coveralls upload route in the `/api/v1`
group (`g`), the webhook at the engine root (`e`), and the build read + token
routes. Concretely:

(a) Right after `g := e.Group("/api/v1")` opens (same level as the `user`/`reports`
blocks), add the jobs route:

```go
		g.POST("/jobs", build.HandleJobs(r.Config, r.RepoStore, r.BuildStore, r.BuildService))
```

(b) In the authenticated repo group (`/repos/:scm/:namespace/:name` with `checkLogin`),
add the token rotation route alongside `g.PATCH("/report", ...)`:

```go
			g.PATCH("/token", repo.HandleTokenRenew(r.RepoStore))
```

(c) In the unauthenticated repo group (`/repos/:scm/:namespace/:name` near
`repo.HandleGet`), add the build read routes:

```go
		g.GET("/builds", build.HandleList(r.SCMService, r.RepoStore, r.BuildStore))
		g.GET("/builds/:number", build.HandleGet(r.SCMService, r.RepoStore, r.BuildStore))
```

(d) At the end of `RegisterRoutes` (outside the `/api/v1` group, on the engine `e`),
register the webhook:

```go
	e.POST("/webhook", build.HandleWebhook(r.RepoStore, r.BuildStore, r.BuildService))
```

- [ ] **Step 5: Hand-edit `cmd/server/wire_gen.go`**

`wire` is not installed, so edit the generated injector directly. In
`InitializeApplication`, add the two new providers and pass them to `provideRouter`:

```go
	reportStore := provideReportStore(databaseService)
	buildStore := provideBuildStore(databaseService)
	buildService := provideBuildService(buildStore, repoStore, scmService)
	hookService := provideHookService(scmService, repoStore, reportStore, reportService)
	oAuthStore := provideOAuthStore(databaseService)
	oAuthService := provideOAuthService(config2, oAuthStore, userStore)
	routers := provideRouter(session, config2, loginMiddleware, scmService, coverageService, chartService, reportService, repoService, hookService, oAuthService, userStore, reportStore, repoStore, oAuthStore, buildStore, buildService)
```

(Add `buildStore`/`buildService` as the final two args; `provideRouter`'s signature
is updated in the next step.)

- [ ] **Step 6: Update `provideRouter` in `cmd/server/inject_router.go`**

Add two parameters at the end of `provideRouter` and set the new fields:

```go
	// store
	userStore core.UserStore,
	reportStore core.ReportStore,
	repoStore core.RepoStore,
	oauthStore core.OAuthStore,
	buildStore core.BuildStore,
	buildService core.BuildService,
) *routers.Routers {
	return &routers.Routers{
		Config:          config,
		Session:         session,
		LoginMiddleware: login,
		SCMService:      scmService,
		CoverageService: coverageService,
		ChartService:    chartService,
		RepoService:     repoService,
		ReportService:   reportService,
		HookService:     hookService,
		OAuthService:    oauthSerice,
		UserStore:       userStore,
		ReportStore:     reportStore,
		RepoStore:       repoStore,
		OAuthStore:      oauthStore,
		BuildStore:      buildStore,
		BuildService:    buildService,
	}
}
```

- [ ] **Step 7: Verify the whole tree builds and tests pass**

Run: `go build ./... && go test ./routers/... ./modules/... ./models/... -count=1`
Expected: success; all tests PASS.

- [ ] **Step 8: Commit**

```bash
git add cmd/server/ routers/init.go routers/api/api.go
git commit -m "feat: wire build store/service and register coveralls routes"
```

---

## Phase 3 — Repoint reads, remove legacy ingestion

### Task 12: Repoint the badge to the build model

**Files:**
- Modify: `routers/api/report/badge.go`
- Modify: `routers/api/api.go` (badge route args)

- [ ] **Step 1: Rewrite `HandleGetBadge` to use builds**

Replace `routers/api/report/badge.go` with:

```go
package report

import (
	"fmt"

	"github.com/covergates/covergates/core"
	"github.com/gin-gonic/gin"
	"github.com/narqo/go-badge"
)

// HandleGetBadge for the report id
// @Summary get badge for the report id
// @Tags Report
// @Param id path string true "report id"
// @Success 200 {object} string "badge svg"
// @Router /reports/{id}/badge [get]
func HandleGetBadge(
	repoStore core.RepoStore,
	buildStore core.BuildStore,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		reportID := c.Param("id")
		repo, err := repoStore.Find(&core.Repo{ReportID: reportID})
		if err != nil {
			c.String(404, "repository not found")
			return
		}
		coverage := 0
		if build, err := buildStore.LatestOnBranch(repo.ID, repo.Branch, 0); err == nil {
			coverage = int(build.Coverage * 100)
		}
		data, err := badge.RenderBytes(
			"Covergates",
			fmt.Sprintf("%d%%", coverage),
			"#00838F",
		)
		if err != nil {
			c.String(500, err.Error())
			return
		}
		c.Header("Cache-Control", "max-age=600")
		c.Data(200, "image/svg+xml", data)
	}
}
```

- [ ] **Step 2: Update the badge route registration**

In `routers/api/api.go`, change the badge route to pass the build store:

```go
		g.GET("/:id/badge", report.HandleGetBadge(r.RepoStore, r.BuildStore))
```

- [ ] **Step 3: Verify build**

Run: `go build ./...`
Expected: success.

- [ ] **Step 4: Commit**

```bash
git add routers/api/report/badge.go routers/api/api.go
git commit -m "refactor: serve coverage badge from build model"
```

---

### Task 13: Remove the native upload handler and route

**Files:**
- Modify: `routers/api/report/report.go` (delete `HandleUpload`, `loadCoverageReport`)
- Modify: `routers/api/report/report_test.go` (delete `TestUpload` + now-unused helpers)
- Modify: `routers/api/api.go` (remove the upload route)

- [ ] **Step 1: Remove the upload route**

In `routers/api/api.go`, delete the entire `g.POST("/:id", ...)` block inside the
`reports` group (the one wrapping `report.HandleUpload(...)` with
`InjectReportContext`/`ProtectReport`). Leave the other report routes intact.

- [ ] **Step 2: Delete `HandleUpload` and `loadCoverageReport`**

In `routers/api/report/report.go`, delete the `HandleUpload` function (and its
swagger comment block) and the `loadCoverageReport` helper. Then remove imports
that are now unused. After deletion the file's imports should be:

```go
import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/covergates/covergates/config"
	"github.com/covergates/covergates/core"
	"github.com/covergates/covergates/routers/api/request"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)
```

(Drop `encoding/json` — it was only used by `HandleUpload`. Keep `context` — it is
used by `getGitRepository`.)

- [ ] **Step 3: Remove the upload test and unused helpers**

In `routers/api/report/report_test.go`, delete the `TestUpload` function. Then
delete the helpers that become unused: `addFormFile`, `createForm`, and
`encodeJSON` (search the file to confirm no other test references them; if one
does, keep it). Remove now-unused imports (`bytes`, `io`, `mime/multipart`, and
`log "github.com/sirupsen/logrus"` if only `TestUpload` used them).

- [ ] **Step 4: Verify report package builds and tests pass**

Run: `go test ./routers/api/report/ -count=1`
Expected: PASS (remaining report tests).

- [ ] **Step 5: Commit**

```bash
git add routers/api/report/report.go routers/api/report/report_test.go routers/api/api.go
git commit -m "refactor: remove native coverage upload endpoint"
```

---

### Task 14: Remove CoverageService and the parser packages

**Files:**
- Modify: `core/report.go` (remove `CoverageService` interface; fix `go:generate`)
- Modify: `mock/report_mock.go` (remove `MockCoverageService`)
- Modify: `cmd/server/inject_service.go` (remove `provideCoverageService`)
- Modify: `cmd/server/inject_router.go` (remove `coverageService` param + field)
- Modify: `cmd/server/wire_gen.go` (remove `coverageService`)
- Modify: `routers/init.go` (remove `CoverageService` field)
- Modify: `routers/api/api.go` (remove `CoverageService` field)
- Delete: `service/golang`, `service/python`, `service/ruby`, `service/lcov`, `service/clover`, `service/perl`, `service/coverage`, `service/common`

- [ ] **Step 1: Remove the `CoverageService` interface from `core/report.go`**

Delete the `CoverageService` interface block. Change the `go:generate` directive at
the top of the file from:

```go
//go:generate mockgen -package mock -destination ../mock/report_mock.go . ReportStore,CoverageService
```

to:

```go
//go:generate mockgen -package mock -destination ../mock/report_mock.go . ReportStore
```

The `io` import in `core/report.go` is still used by `ReportService` (`MarkdownReport`
returns `io.Reader`) — keep it.

- [ ] **Step 2: Remove `MockCoverageService` from `mock/report_mock.go`**

Delete everything from `type MockCoverageService struct {` to the end of the file
(the `MockCoverageService`, `MockCoverageServiceMockRecorder`, `NewMockCoverageService`,
and all their methods). Keep the `MockReportStore` portion. Remove any imports at the
top of `mock/report_mock.go` that were only used by the removed methods (e.g. `io`,
`context`) — run `go build ./mock/...` to confirm which remain.

- [ ] **Step 3: Remove `provideCoverageService`**

In `cmd/server/inject_service.go`: delete `provideCoverageService` from `serviceSet`,
delete the function, and remove the `"github.com/covergates/covergates/service/coverage"`
import.

- [ ] **Step 4: Remove `coverageService` from the router wiring**

(a) `cmd/server/inject_router.go`: remove the `coverageService core.CoverageService`
parameter and the `CoverageService: coverageService,` field assignment.

(b) `cmd/server/wire_gen.go`: remove the `coverageService := provideCoverageService()`
line and remove `coverageService` from the `provideRouter(...)` call.

(c) `routers/init.go`: remove the `CoverageService core.CoverageService` field from
`Routers` and the `CoverageService: r.CoverageService,` assignment in `apiRoute`.

(d) `routers/api/api.go`: remove the `CoverageService core.CoverageService` field from
`Router`.

- [ ] **Step 5: Delete the parser packages**

```bash
git rm -r service/golang service/python service/ruby service/lcov service/clover service/perl service/coverage service/common
```

- [ ] **Step 6: Verify the whole tree builds and tests pass**

Run: `go build ./... && go test ./... -count=1`
Expected: success. (Note: tests tagged `gitea` require a Gitea service and are not
run here; the default `go test ./...` excludes them.)

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "refactor: remove server-side coverage parsers and CoverageService"
```

---

### Task 15: Remove the CLI binary and update build/CI

**Files:**
- Delete: `cmd/cli/`
- Modify: `Dockerfile`
- Modify: `.github/workflows/cd.yml`

- [ ] **Step 1: Delete the CLI**

```bash
git rm -r cmd/cli
```

- [ ] **Step 2: Remove CLI stages from `Dockerfile`**

Delete the `cli-build` stage and the `cli` stage. The resulting `Dockerfile` is:

```dockerfile
FROM golang:alpine AS build
RUN apk --update add musl-dev
RUN apk --update add util-linux-dev
RUN apk --update add gcc g++

WORKDIR /go/src/github.com/covergates/covergates

RUN go env -w GOPROXY=https://goproxy.cn,direct
COPY go.mod ./
COPY go.sum ./
RUN go mod download

FROM build as server-build
RUN apk --update add nodejs npm
COPY web/package.json ./web/package.json
COPY web/package-lock.json ./web/package-lock.json
RUN cd web && npm install
RUN node --version && npm --version
RUN go env -w GOBIN=/bin
RUN go install github.com/bradrydzewski/togo@latest
COPY web ./web
RUN go generate ./web
COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -v -o covergates ./cmd/server

FROM alpine as server
COPY --from=server-build /go/src/github.com/covergates/covergates/covergates /covergates
ENTRYPOINT [ "/covergates" ]
```

- [ ] **Step 3: Remove CLI build/release from `.github/workflows/cd.yml`**

In the `Build Binary` step's script, delete the two CLI-specific lines:

```
          cli_flag="-X main.Version=$GITHUB_TAG -X main.CoverGatesAPI=$SERVER_API_URL"
```
and
```
          gox -ldflags="$cli_flag" -osarch="$targets" -output "covergates-{{.OS}}-{{.Arch}}" ./cmd/cli
```

Change the outputs array from:

```
          outputs=(covergates covergates-server)
```
to:

```
          outputs=(covergates-server)
```

In the `Upload Binary` github-script step, the asset loop uploads
`covergates-${tag}-${arch}.tar.gz`, which now contains only the server binary
(renamed to `covergates-server`). Leave the archive names unchanged so release
asset URLs stay stable. The `SERVER_API_URL` env var is now unused but harmless;
leave it.

- [ ] **Step 4: Verify build (CLI gone, server still builds)**

Run: `go build ./...`
Expected: success.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "refactor: drop covergates CLI binary and CLI build/release steps"
```

---

## Phase 4 — Final verification

### Task 16: Full verification

- [ ] **Step 1: Tidy modules (in case CLI-only deps were removed)**

Run: `go mod tidy`
Expected: `go.mod`/`go.sum` updated or unchanged; no errors.

- [ ] **Step 2: Full build**

Run: `go build ./...`
Expected: success.

- [ ] **Step 3: Full test suite**

Run: `go test ./... -count=1`
Expected: all PASS (gitea-tagged tests excluded by default).

- [ ] **Step 4: Lint (matches CI)**

Run: `gofmt -l . | grep -v vendor`
Expected: no files listed (all formatted). If `golint` is installed, also run
`golint ./...` and address exported-symbol comment warnings on new files.

- [ ] **Step 5: Manual smoke test (optional but recommended)**

Start the server against sqlite and exercise the coveralls endpoint:

```bash
go run ./cmd/server &
# After activating a repo via the UI/API to obtain its token, POST a job:
curl -s -X POST http://localhost:8080/api/v1/jobs \
  -H 'Content-Type: application/json' \
  -d '{"repo_token":"<TOKEN>","git":{"branch":"master","head":{"id":"<SHA>"}},
       "source_files":[{"name":"main.go","coverage":[1,null,0]}]}'
# Expect: {"id":...,"message":"Job created","url":"..."}
curl -s http://localhost:8080/api/v1/repos/<scm>/<owner>/<name>/builds
# Expect: a JSON array with one build, status "done", coverage 0.5
```

- [ ] **Step 6: Commit any tidy changes**

```bash
git add go.mod go.sum
git commit -m "chore: go mod tidy after parser/CLI removal" || echo "nothing to commit"
```

---

## Self-Review Notes (spec coverage)

- **A1 data model** → Tasks 1, 3, 4.
- **A2 repo token** → Tasks 2, 10 (rotate), 3 (backfill).
- **A3 POST /api/v1/jobs** → Tasks 8, 9, 11.
- **A4 POST /webhook** → Tasks 9, 11 (root registration).
- **A5 merge algorithm** → Task 5.
- **A6 delta/base selection** → Task 7.
- **A7 store + read endpoints** → Tasks 1, 3, 9, 11; badge repoint Task 12.
- **A8 removals** → Tasks 13 (native upload), 14 (parsers/CoverageService), 15 (CLI + Dockerfile/CD).
- **A9 testing & errors** → Tasks 4, 5, 7, 8, 9 (422 error shapes covered in 9).

Out of scope (deferred per spec): flags, carryforward, legacy `Report` table removal,
UI (Sub-project B), notifications/checks (Sub-project C).
