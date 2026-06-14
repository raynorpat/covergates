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

func TestHandleWebhookBadJSON(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repos := mock.NewMockRepoStore(ctrl)
	builds := mock.NewMockBuildStore(ctrl)
	svc := mock.NewMockBuildService(ctrl)

	repos.EXPECT().Find(&core.Repo{Token: "tok"}).Return(&core.Repo{ID: 1}, nil)

	r := gin.New()
	r.POST("/webhook", HandleWebhook(repos, builds, svc))
	req := httptest.NewRequest("POST", "/webhook?repo_token=tok", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")

	if w := serve(r, req); w.Code != 422 {
		t.Fatalf("status = %d, want 422", w.Code)
	}
}

func TestHandleGetReturnsBuild(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	scm := mock.NewMockSCMService(ctrl)
	repos := mock.NewMockRepoStore(ctrl)
	builds := mock.NewMockBuildStore(ctrl)

	repos.EXPECT().Find(gomock.Any()).Return(&core.Repo{ID: 1, Private: false}, nil)
	builds.EXPECT().FindByNumber(uint(1), 2).Return(&core.Build{ID: 7, Number: 2, Coverage: 0.5}, nil)
	builds.EXPECT().Jobs(uint(7)).Return([]*core.Job{}, nil)

	r := gin.New()
	r.GET("/api/v1/repos/:scm/:namespace/:name/builds/:number", HandleGet(scm, repos, builds))
	req := httptest.NewRequest("GET", "/api/v1/repos/github/o/r/builds/2", nil)
	if w := serve(r, req); w.Code != 200 {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
}
