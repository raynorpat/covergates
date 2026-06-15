package build

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/covergates/covergates/core"
	"github.com/covergates/covergates/mock"
	"github.com/covergates/covergates/routers/api/request"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
)

// fakePRChanges is a minimal core.PullRequestService stub (no generated mock exists).
type fakePRChanges struct{ changes []*core.FileChange }

func (f *fakePRChanges) Find(ctx context.Context, user *core.User, repo string, number int) (*core.PullRequest, error) {
	return nil, nil
}
func (f *fakePRChanges) CreateComment(ctx context.Context, user *core.User, repo string, number int, body string) (int, error) {
	return 0, nil
}
func (f *fakePRChanges) RemoveComment(ctx context.Context, user *core.User, repo string, number int, id int) error {
	return nil
}
func (f *fakePRChanges) ListChanges(ctx context.Context, user *core.User, repo string, number int) ([]*core.FileChange, error) {
	return f.changes, nil
}

func TestHandleChanges(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repos := mock.NewMockRepoStore(ctrl)
	scm := mock.NewMockSCMService(ctrl)
	client := mock.NewMockClient(ctrl)

	repos.EXPECT().Find(gomock.Any()).Return(&core.Repo{NameSpace: "o", Name: "r", SCM: core.Github}, nil)
	scm.EXPECT().Client(core.Github).Return(client, nil)
	client.EXPECT().PullRequests().Return(&fakePRChanges{changes: []*core.FileChange{{Path: "a.go"}}})

	r := gin.New()
	r.Use(func(c *gin.Context) { request.WithUser(c, &core.User{Login: "u"}) })
	r.GET("/api/v1/repos/:scm/:namespace/:name/pulls/:number/changes", HandleChanges(scm, repos))
	req := httptest.NewRequest("GET", "/api/v1/repos/github/o/r/pulls/3/changes", nil)
	if w := serve(r, req); w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestHandleChangesUnauthorized(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repos := mock.NewMockRepoStore(ctrl)
	scm := mock.NewMockSCMService(ctrl)

	r := gin.New()
	r.GET("/api/v1/repos/:scm/:namespace/:name/pulls/:number/changes", HandleChanges(scm, repos))
	req := httptest.NewRequest("GET", "/api/v1/repos/github/o/r/pulls/3/changes", nil)
	if w := serve(r, req); w.Code != 401 {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}
