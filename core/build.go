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
	Flag             string        `json:"flag"`
	SourceFiles      []*SourceFile `json:"-"`
	CreatedAt        time.Time     `json:"createdAt"`
}

// Build is a coverage build for a repository, composed of one or more jobs.
type Build struct {
	ID              uint          `json:"id"`
	RepoID          uint          `json:"repoID"`
	Number          int           `json:"number"`
	ServiceName     string        `json:"serviceName"`
	ServiceNumber   string        `json:"serviceNumber"`
	Commit          string        `json:"commit"`
	Branch          string        `json:"branch"`
	PullRequest     int           `json:"pullRequest"` // 0 if not a pull request build
	Status          BuildStatus   `json:"status"`
	Parallel        bool          `json:"parallel"`
	Coverage        float64       `json:"coverage"`
	CoverageChange  float64       `json:"coverageChange"`
	BaseBuildID     uint          `json:"baseBuildID"`
	BaseBuildNumber int           `json:"baseBuildNumber"`
	CommitMessage   string        `json:"commitMessage"`
	AuthorName      string        `json:"authorName"`
	AuthorEmail     string        `json:"authorEmail"`
	SourceFiles     []*SourceFile `json:"sourceFiles,omitempty"`
	Jobs            []*Job        `json:"jobs,omitempty"`
	CreatedAt       time.Time     `json:"createdAt"`
	FinishedAt      time.Time     `json:"finishedAt,omitempty"` // zero if not yet finished
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
