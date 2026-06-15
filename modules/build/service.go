package build

import (
	"context"
	"time"

	"github.com/covergates/covergates/core"
)

// Service implements core.BuildService.
type Service struct {
	Builds core.BuildStore
	Repos  core.RepoStore
	SCM    core.SCMService
}

// Finalize merges all jobs into the build, computes coverage and the delta
// against a base build, and marks the build done. On any failure the build is
// marked errored and the error is returned.
func (s *Service) Finalize(ctx context.Context, repo *core.Repo, build *core.Build) error {
	if err := s.finalize(ctx, repo, build); err != nil {
		build.Status = core.BuildErrored
		build.FinishedAt = time.Now()
		_ = s.Builds.Update(build)
		return err
	}
	return nil
}

func (s *Service) finalize(ctx context.Context, repo *core.Repo, build *core.Build) error {
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
		build.BaseBuildNumber = base.Number
	}

	build.Status = core.BuildDone
	build.FinishedAt = time.Now()
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
