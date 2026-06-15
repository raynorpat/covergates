package user

import (
	"sort"

	"github.com/covergates/covergates/core"
	"github.com/covergates/covergates/routers/api/request"
	"github.com/gin-gonic/gin"
)

type repoCoverage struct {
	SCM       core.SCMProvider `json:"scm"`
	NameSpace string           `json:"namespace"`
	Name      string           `json:"name"`
	ReportID  string           `json:"reportID"`
	Coverage  float64          `json:"coverage"`
}

type repoStats struct {
	RepoCount       int            `json:"repoCount"`
	ActivatedCount  int            `json:"activatedCount"`
	AverageCoverage float64        `json:"averageCoverage"`
	TopRepos        []repoCoverage `json:"topRepos"`
}

// HandleRepoStats summarizes the user's repositories and their coverage.
// @Summary Repository coverage stats for the user
// @Tags User
// @Success 200 {object} repoStats
// @Router /user/stats [get]
func HandleRepoStats(userStore core.UserStore, buildStore core.BuildStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		user := request.MustGetUserFrom(c)
		repos, err := userStore.ListRepositories(user)
		if err != nil {
			c.JSON(500, &repoStats{})
			return
		}
		stats := &repoStats{RepoCount: len(repos)}
		withCoverage := make([]repoCoverage, 0, len(repos))
		var sum float64
		for _, repo := range repos {
			if repo.ReportID != "" {
				stats.ActivatedCount++
			}
			build, err := buildStore.LatestOnBranch(repo.ID, repo.Branch, 0)
			if err != nil || build == nil {
				continue
			}
			withCoverage = append(withCoverage, repoCoverage{
				SCM:       repo.SCM,
				NameSpace: repo.NameSpace,
				Name:      repo.Name,
				ReportID:  repo.ReportID,
				Coverage:  build.Coverage,
			})
			sum += build.Coverage
		}
		if len(withCoverage) > 0 {
			stats.AverageCoverage = sum / float64(len(withCoverage))
		}
		sort.SliceStable(withCoverage, func(i, j int) bool {
			return withCoverage[i].Coverage > withCoverage[j].Coverage
		})
		if len(withCoverage) > 5 {
			withCoverage = withCoverage[:5]
		}
		stats.TopRepos = withCoverage
		c.JSON(200, stats)
	}
}
