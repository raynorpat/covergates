package build

import (
	"context"
	"errors"
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

func TestFinalizeErroredOnFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	builds := mock.NewMockBuildStore(ctrl)
	repos := mock.NewMockRepoStore(ctrl)
	scm := mock.NewMockSCMService(ctrl)

	repo := &core.Repo{ID: 1, Branch: "master"}
	build := &core.Build{ID: 5, RepoID: 1, Branch: "master", Status: core.BuildProcessing}

	builds.EXPECT().Jobs(uint(5)).Return(nil, errors.New("db down"))
	builds.EXPECT().Update(gomock.Any()).DoAndReturn(func(b *core.Build) error {
		if b.Status != core.BuildErrored {
			t.Fatalf("status = %v, want errored", b.Status)
		}
		return nil
	})

	svc := &Service{Builds: builds, Repos: repos, SCM: scm}
	if err := svc.Finalize(context.Background(), repo, build); err == nil {
		t.Fatal("expected error")
	}
}
