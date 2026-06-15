package repo

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/covergates/covergates/core"
	"github.com/covergates/covergates/mock"
	"github.com/covergates/covergates/routers/api/request"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
)

func withUser(login string) gin.HandlerFunc {
	return func(c *gin.Context) { request.WithUser(c, &core.User{Login: login}) }
}

func TestHandleGetSettingForbidden(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	store := mock.NewMockRepoStore(ctrl)
	scm := mock.NewMockSCMService(ctrl)
	client := mock.NewMockClient(ctrl)
	repos := mock.NewMockGitRepoService(ctrl)

	store.EXPECT().Find(gomock.Any()).Return(&core.Repo{Name: "r", NameSpace: "o", SCM: core.Github}, nil)
	scm.EXPECT().Client(core.Github).Return(client, nil)
	client.EXPECT().Repositories().Return(repos)
	repos.EXPECT().Find(gomock.Any(), gomock.Any(), "o/r").Return(nil, errors.New("no access"))

	r := gin.New()
	r.Use(withUser("u"))
	r.GET("/repos/:scm/:namespace/:name/setting", HandleGetSetting(store, scm))
	req := httptest.NewRequest("GET", "/repos/github/o/r/setting", nil)
	testRequest(r, req, func(w *httptest.ResponseRecorder) {
		if w.Code != 403 {
			t.Fatalf("status = %d, want 403", w.Code)
		}
	})
}

func TestHandleGetSettingAllowed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	store := mock.NewMockRepoStore(ctrl)
	scm := mock.NewMockSCMService(ctrl)
	client := mock.NewMockClient(ctrl)
	repos := mock.NewMockGitRepoService(ctrl)

	store.EXPECT().Find(gomock.Any()).Return(&core.Repo{Name: "r", NameSpace: "o", SCM: core.Github}, nil)
	scm.EXPECT().Client(core.Github).Return(client, nil)
	client.EXPECT().Repositories().Return(repos)
	repos.EXPECT().Find(gomock.Any(), gomock.Any(), "o/r").Return(&core.Repo{}, nil)
	store.EXPECT().Setting(gomock.Any()).Return(&core.RepoSetting{Protected: true}, nil)

	r := gin.New()
	r.Use(withUser("u"))
	r.GET("/repos/:scm/:namespace/:name/setting", HandleGetSetting(store, scm))
	req := httptest.NewRequest("GET", "/repos/github/o/r/setting", nil)
	testRequest(r, req, func(w *httptest.ResponseRecorder) {
		if w.Code != 200 {
			t.Fatalf("status = %d, want 200", w.Code)
		}
	})
}

func TestHandleHookCreateForbidden(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	store := mock.NewMockRepoStore(ctrl)
	scm := mock.NewMockSCMService(ctrl)
	client := mock.NewMockClient(ctrl)
	repos := mock.NewMockGitRepoService(ctrl)

	scm.EXPECT().Client(core.Github).Return(client, nil)
	client.EXPECT().Repositories().Return(repos)
	repos.EXPECT().IsAdmin(gomock.Any(), gomock.Any(), "o/r").Return(false)
	store.EXPECT().Creator(gomock.Any()).Return(&core.User{Login: "other"}, nil)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		request.WithUser(c, &core.User{Login: "u"})
		c.Set(keyRepo, &core.Repo{Name: "r", NameSpace: "o", SCM: core.Github})
	})
	// HookService is never reached on the 403 path, so nil is fine.
	r.POST("/hook/create", HandleHookCreate(nil, scm, store))
	req := httptest.NewRequest("POST", "/hook/create", nil)
	testRequest(r, req, func(w *httptest.ResponseRecorder) {
		if w.Code != 403 {
			t.Fatalf("status = %d, want 403", w.Code)
		}
	})
}

func TestHandleUpdateSettingForbidden(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	store := mock.NewMockRepoStore(ctrl)
	scm := mock.NewMockSCMService(ctrl)
	client := mock.NewMockClient(ctrl)
	repos := mock.NewMockGitRepoService(ctrl)

	scm.EXPECT().Client(core.Github).Return(client, nil)
	client.EXPECT().Repositories().Return(repos)
	repos.EXPECT().IsAdmin(gomock.Any(), gomock.Any(), "o/r").Return(false)
	store.EXPECT().Creator(gomock.Any()).Return(&core.User{Login: "other"}, nil)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		request.WithUser(c, &core.User{Login: "u"})
		c.Set(keyRepo, &core.Repo{Name: "r", NameSpace: "o", SCM: core.Github})
	})
	r.POST("/setting", HandleUpdateSetting(store, scm))
	req := httptest.NewRequest("POST", "/setting", nil)
	testRequest(r, req, func(w *httptest.ResponseRecorder) {
		if w.Code != 403 {
			t.Fatalf("status = %d, want 403", w.Code)
		}
	})
}

func TestHandleGetSettingRedactsWebhook(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	store := mock.NewMockRepoStore(ctrl)
	scm := mock.NewMockSCMService(ctrl)
	client := mock.NewMockClient(ctrl)
	repos := mock.NewMockGitRepoService(ctrl)

	store.EXPECT().Find(gomock.Any()).Return(&core.Repo{Name: "r", NameSpace: "o", SCM: core.Github}, nil)
	scm.EXPECT().Client(core.Github).Return(client, nil)
	client.EXPECT().Repositories().Return(repos)
	repos.EXPECT().Find(gomock.Any(), gomock.Any(), "o/r").Return(&core.Repo{}, nil)
	store.EXPECT().Setting(gomock.Any()).Return(&core.RepoSetting{
		SlackWebhook:     "https://hooks.slack.com/secret",
		EmailRecipients:  []string{"a@x.com"},
	}, nil)

	r := gin.New()
	r.Use(withUser("u"))
	r.GET("/repos/:scm/:namespace/:name/setting", HandleGetSetting(store, scm))
	req := httptest.NewRequest("GET", "/repos/github/o/r/setting", nil)
	testRequest(r, req, func(w *httptest.ResponseRecorder) {
		if w.Code != 200 {
			t.Fatalf("status = %d, want 200", w.Code)
		}
		body := w.Body.String()
		if strings.Contains(body, "hooks.slack.com") {
			t.Fatalf("response must not contain webhook secret, got: %s", body)
		}
		if !strings.Contains(body, "a@x.com") {
			t.Fatalf("response must contain emailRecipients, got: %s", body)
		}
	})
}

func TestHandleUpdateSettingPreservesWebhook(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	store := mock.NewMockRepoStore(ctrl)
	scm := mock.NewMockSCMService(ctrl)
	client := mock.NewMockClient(ctrl)
	repos := mock.NewMockGitRepoService(ctrl)

	scm.EXPECT().Client(core.Github).Return(client, nil)
	client.EXPECT().Repositories().Return(repos)
	repos.EXPECT().IsAdmin(gomock.Any(), gomock.Any(), "o/r").Return(true)
	store.EXPECT().Setting(gomock.Any()).Return(&core.RepoSetting{SlackWebhook: "https://hooks.slack.com/kept"}, nil)
	store.EXPECT().UpdateSetting(gomock.Any(), gomock.Any()).DoAndReturn(func(_ *core.Repo, s *core.RepoSetting) error {
		if s.SlackWebhook != "https://hooks.slack.com/kept" {
			t.Fatalf("webhook should be preserved, got: %q", s.SlackWebhook)
		}
		return nil
	})

	r := gin.New()
	r.Use(func(c *gin.Context) {
		request.WithUser(c, &core.User{Login: "u"})
		c.Set(keyRepo, &core.Repo{Name: "r", NameSpace: "o", SCM: core.Github})
	})
	r.POST("/setting", HandleUpdateSetting(store, scm))
	body := `{"notifyTrigger":"always"}`
	req := httptest.NewRequest("POST", "/setting", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	testRequest(r, req, func(w *httptest.ResponseRecorder) {
		if w.Code != 200 {
			t.Fatalf("status = %d, want 200", w.Code)
		}
		if strings.Contains(w.Body.String(), "hooks.slack.com") {
			t.Fatalf("response must not echo the webhook secret, got: %s", w.Body.String())
		}
	})
}

func TestHandleUpdateSettingSetsWebhook(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	store := mock.NewMockRepoStore(ctrl)
	scm := mock.NewMockSCMService(ctrl)
	client := mock.NewMockClient(ctrl)
	repos := mock.NewMockGitRepoService(ctrl)

	scm.EXPECT().Client(core.Github).Return(client, nil)
	client.EXPECT().Repositories().Return(repos)
	repos.EXPECT().IsAdmin(gomock.Any(), gomock.Any(), "o/r").Return(true)
	// store.Setting must NOT be called when incoming webhook is non-empty
	store.EXPECT().UpdateSetting(gomock.Any(), gomock.Any()).DoAndReturn(func(_ *core.Repo, s *core.RepoSetting) error {
		if s.SlackWebhook != "https://hooks.slack.com/new" {
			t.Fatalf("webhook should be set to new value, got: %q", s.SlackWebhook)
		}
		return nil
	})

	r := gin.New()
	r.Use(func(c *gin.Context) {
		request.WithUser(c, &core.User{Login: "u"})
		c.Set(keyRepo, &core.Repo{Name: "r", NameSpace: "o", SCM: core.Github})
	})
	r.POST("/setting", HandleUpdateSetting(store, scm))
	body := `{"slackWebhook":"https://hooks.slack.com/new","notifyTrigger":"always"}`
	req := httptest.NewRequest("POST", "/setting", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	testRequest(r, req, func(w *httptest.ResponseRecorder) {
		if w.Code != 200 {
			t.Fatalf("status = %d, want 200", w.Code)
		}
		if strings.Contains(w.Body.String(), "hooks.slack.com") {
			t.Fatalf("response must not echo the webhook secret, got: %s", w.Body.String())
		}
	})
}
