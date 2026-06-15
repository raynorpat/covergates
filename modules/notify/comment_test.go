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
