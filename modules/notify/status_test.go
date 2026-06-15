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

	user := &core.User{Login: "octocat"}
	store.EXPECT().Creator(repo).Return(user, nil)
	scmService.EXPECT().Client(repo.SCM).Return(client, nil)
	client.EXPECT().Repositories().Return(repos)
	repos.EXPECT().CreateStatus(
		gomock.Any(),
		user,
		repo.FullName(),
		"abc123",
		gomock.AssignableToTypeOf(&core.Status{}),
	).DoAndReturn(func(_ context.Context, _ *core.User, _, _ string, s *core.Status) error {
		if s.State != core.StatusSuccess {
			t.Errorf("state = %q, want success", s.State)
		}
		if s.Label != "coverage/covergates" {
			t.Errorf("label = %q", s.Label)
		}
		wantTarget := "http://localhost:8080/report/github/octocat/hello/builds/12"
		if s.Target != wantTarget {
			t.Errorf("target = %q, want %q", s.Target, wantTarget)
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
