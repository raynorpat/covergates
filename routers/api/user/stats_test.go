package user_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/covergates/covergates/core"
	"github.com/covergates/covergates/mock"
	"github.com/covergates/covergates/routers/api/request"
	"github.com/covergates/covergates/routers/api/user"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"gorm.io/gorm"
)

func TestHandleRepoStats(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	u := &core.User{Login: "test"}
	repos := []*core.Repo{
		{ID: 1, NameSpace: "o", Name: "a", SCM: core.Github, ReportID: "r1", Branch: "main"}, // cov 0.90
		{ID: 2, NameSpace: "o", Name: "b", SCM: core.Github, ReportID: "r2", Branch: "main"}, // cov 0.50
		{ID: 3, NameSpace: "o", Name: "c", SCM: core.Github, ReportID: "", Branch: "main"},   // not activated, no build
	}
	userStore := mock.NewMockUserStore(ctrl)
	buildStore := mock.NewMockBuildStore(ctrl)
	userStore.EXPECT().ListRepositories(u).Return(repos, nil)
	buildStore.EXPECT().LatestOnBranch(uint(1), "main", uint(0)).Return(&core.Build{Coverage: 0.90}, nil)
	buildStore.EXPECT().LatestOnBranch(uint(2), "main", uint(0)).Return(&core.Build{Coverage: 0.50}, nil)
	buildStore.EXPECT().LatestOnBranch(uint(3), "main", uint(0)).Return(nil, gorm.ErrRecordNotFound)

	r := gin.New()
	r.Use(func(c *gin.Context) { request.WithUser(c, u) })
	r.GET("/user/stats", user.HandleRepoStats(userStore, buildStore))

	req, _ := http.NewRequest("GET", "/user/stats", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	var got struct {
		RepoCount       int     `json:"repoCount"`
		ActivatedCount  int     `json:"activatedCount"`
		AverageCoverage float64 `json:"averageCoverage"`
		TopRepos        []struct {
			Name     string  `json:"name"`
			Coverage float64 `json:"coverage"`
		} `json:"topRepos"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	if got.RepoCount != 3 || got.ActivatedCount != 2 {
		t.Fatalf("counts: repo=%d activated=%d", got.RepoCount, got.ActivatedCount)
	}
	if got.AverageCoverage < 0.69 || got.AverageCoverage > 0.71 { // (0.9+0.5)/2 over repos WITH a build
		t.Fatalf("average = %v, want ~0.70", got.AverageCoverage)
	}
	if len(got.TopRepos) != 2 || got.TopRepos[0].Name != "a" || got.TopRepos[1].Name != "b" {
		t.Fatalf("topRepos wrong: %+v", got.TopRepos) // repo c (no build) excluded; sorted desc by coverage
	}
}

func TestHandleRepoStatsListError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	u := &core.User{Login: "test"}
	userStore := mock.NewMockUserStore(ctrl)
	buildStore := mock.NewMockBuildStore(ctrl)
	userStore.EXPECT().ListRepositories(u).Return(nil, errors.New("db down"))

	r := gin.New()
	r.Use(func(c *gin.Context) { request.WithUser(c, u) })
	r.GET("/user/stats", user.HandleRepoStats(userStore, buildStore))

	req, _ := http.NewRequest("GET", "/user/stats", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 500 {
		t.Fatalf("status = %d, want 500", w.Code)
	}
}
