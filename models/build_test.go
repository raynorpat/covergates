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
		Flag:         "unit",
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
	if jobs[0].Flag != "unit" {
		t.Fatalf("flag = %q, want unit", jobs[0].Flag)
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
	first.BaseBuildNumber = 7
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
	if got.BaseBuildNumber != 7 {
		t.Fatalf("baseBuildNumber = %d, want 7", got.BaseBuildNumber)
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
