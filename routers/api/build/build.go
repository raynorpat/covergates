package build

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/covergates/covergates/config"
	"github.com/covergates/covergates/core"
	buildmod "github.com/covergates/covergates/modules/build"
	"github.com/covergates/covergates/routers/api/request"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// HandleJobs ingests a coveralls job submission.
// @Summary Upload a coverage job (coveralls-compatible)
// @Tags Build
// @Success 200 {object} string "job created"
// @Router /jobs [post]
func HandleJobs(
	cfg *config.Config,
	repoStore core.RepoStore,
	buildStore core.BuildStore,
	buildService core.BuildService,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		payload, err := parsePayload(c)
		if err != nil {
			c.JSON(422, gin.H{"message": "invalid payload"})
			return
		}
		if payload.RepoToken == "" || len(payload.SourceFiles) == 0 {
			c.JSON(422, gin.H{"message": "missing repo_token or source_files"})
			return
		}
		repo, err := repoStore.Find(&core.Repo{Token: payload.RepoToken})
		if err != nil {
			c.JSON(422, gin.H{"message": "repository not found for token"})
			return
		}
		build, err := resolveBuild(buildStore, repo, payload)
		if err != nil {
			c.JSON(500, gin.H{"message": err.Error()})
			return
		}
		job := &core.Job{
			ServiceJobID:     payload.ServiceJobID,
			ServiceJobNumber: payload.ServiceJobNumber,
			SourceFiles:      payload.SourceFiles,
			Coverage:         buildmod.Coverage(payload.SourceFiles),
		}
		if err := buildStore.AddJob(build, job); err != nil {
			c.JSON(500, gin.H{"message": err.Error()})
			return
		}
		if !payload.Parallel {
			if err := buildService.Finalize(c.Request.Context(), repo, build); err != nil {
				c.JSON(500, gin.H{"message": err.Error()})
				return
			}
		}
		url := fmt.Sprintf("%s/report/%s/%s", cfg.Server.URL(), repo.SCM, repo.FullName())
		c.JSON(200, gin.H{"id": job.ID, "url": url, "message": "Job created"})
	}
}

type webhookBody struct {
	Payload struct {
		BuildNum string `json:"build_num"`
		Status   string `json:"status"`
	} `json:"payload"`
}

// HandleWebhook finalizes a parallel build.
// @Summary Finalize a parallel coverage build (coveralls-compatible)
// @Tags Build
// @Success 200 {object} string "done"
// @Router /webhook [post]
func HandleWebhook(
	repoStore core.RepoStore,
	buildStore core.BuildStore,
	buildService core.BuildService,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.Query("repo_token")
		if token == "" {
			c.JSON(422, gin.H{"message": "missing repo_token"})
			return
		}
		repo, err := repoStore.Find(&core.Repo{Token: token})
		if err != nil {
			c.JSON(422, gin.H{"message": "repository not found for token"})
			return
		}
		body := &webhookBody{}
		if err := c.ShouldBindJSON(body); err != nil {
			c.JSON(422, gin.H{"message": "invalid payload"})
			return
		}
		build, err := buildStore.FindByServiceNumber(repo.ID, body.Payload.BuildNum)
		if err != nil {
			c.JSON(422, gin.H{"message": "build not found"})
			return
		}
		if err := buildService.Finalize(c.Request.Context(), repo, build); err != nil {
			c.JSON(500, gin.H{"message": err.Error()})
			return
		}
		c.JSON(200, gin.H{"done": true})
	}
}

// HandleList lists builds for a repository.
// @Summary List builds for a repository
// @Tags Build
// @Router /repos/{scm}/{namespace}/{name}/builds [get]
func HandleList(
	scmService core.SCMService,
	repoStore core.RepoStore,
	buildStore core.BuildStore,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		repo, ok := permittedRepo(c, scmService, repoStore)
		if !ok {
			return
		}
		builds, err := buildStore.List(repo.ID, c.Query("branch"), queryInt(c, "limit", 30), queryInt(c, "offset", 0))
		if err != nil {
			c.JSON(404, []*core.Build{})
			return
		}
		c.JSON(200, builds)
	}
}

// HandleGet returns a single build with merged source files.
// @Summary Get a build by number
// @Tags Build
// @Router /repos/{scm}/{namespace}/{name}/builds/{number} [get]
func HandleGet(
	scmService core.SCMService,
	repoStore core.RepoStore,
	buildStore core.BuildStore,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		repo, ok := permittedRepo(c, scmService, repoStore)
		if !ok {
			return
		}
		number, err := strconv.Atoi(c.Param("number"))
		if err != nil {
			c.JSON(400, gin.H{"message": "invalid build number"})
			return
		}
		build, err := buildStore.FindByNumber(repo.ID, number)
		if err != nil {
			c.JSON(404, gin.H{"message": "build not found"})
			return
		}
		if jobs, err := buildStore.Jobs(build.ID); err == nil {
			for _, j := range jobs {
				j.SourceFiles = nil
			}
			build.Jobs = jobs
		}
		c.JSON(200, build)
	}
}

// resolveBuild finds the build for this payload or creates a new one.
func resolveBuild(store core.BuildStore, repo *core.Repo, payload *coverallsPayload) (*core.Build, error) {
	if payload.ServiceNumber != "" {
		build, err := store.FindByServiceNumber(repo.ID, payload.ServiceNumber)
		if err == nil {
			return build, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	} else if commit := payload.commit(); commit != "" {
		build, err := store.FindByCommit(repo.ID, commit)
		if err == nil {
			return build, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}
	pr := 0
	if payload.ServicePullRequest != "" {
		pr, _ = strconv.Atoi(payload.ServicePullRequest)
	}
	build := &core.Build{
		RepoID:        repo.ID,
		ServiceName:   payload.ServiceName,
		ServiceNumber: payload.ServiceNumber,
		Commit:        payload.commit(),
		Branch:        payload.branch(),
		PullRequest:   pr,
		Parallel:      payload.Parallel,
		Status:        core.BuildProcessing,
	}
	if payload.Git != nil {
		build.CommitMessage = payload.Git.Head.Message
		build.AuthorName = payload.Git.Head.AuthorName
		build.AuthorEmail = payload.Git.Head.AuthorEmail
	}
	if err := store.Create(build); err != nil {
		return nil, err
	}
	return build, nil
}

// permittedRepo resolves the repo from the path and enforces read permission:
// public repos are readable by anyone; private repos require a context user.
// This mirrors the existing report read-route behavior.
func permittedRepo(c *gin.Context, scmService core.SCMService, repoStore core.RepoStore) (*core.Repo, bool) {
	repo, err := repoStore.Find(&core.Repo{
		NameSpace: c.Param("namespace"),
		Name:      c.Param("name"),
		SCM:       core.SCMProvider(c.Param("scm")),
	})
	if err != nil {
		c.JSON(404, gin.H{"message": "repository not found"})
		return nil, false
	}
	if !repo.Private {
		return repo, true
	}
	user, ok := request.UserFrom(c)
	if !ok {
		c.JSON(401, gin.H{"message": "unauthorized"})
		return nil, false
	}
	client, err := scmService.Client(repo.SCM)
	if err != nil {
		c.JSON(500, gin.H{"message": err.Error()})
		return nil, false
	}
	if _, err := client.Repositories().Find(c.Request.Context(), user, repo.FullName()); err != nil {
		c.JSON(401, gin.H{"message": "unauthorized"})
		return nil, false
	}
	return repo, true
}

func queryInt(c *gin.Context, key string, def int) int {
	if v, err := strconv.Atoi(c.Query(key)); err == nil {
		return v
	}
	return def
}
