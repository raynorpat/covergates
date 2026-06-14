package report

import (
	"fmt"

	"github.com/covergates/covergates/core"
	"github.com/gin-gonic/gin"
	"github.com/narqo/go-badge"
)

// HandleGetBadge for the report id
// @Summary get badge for the report id
// @Tags Report
// @Param id path string true "report id"
// @Success 200 {object} string "badge svg"
// @Router /reports/{id}/badge [get]
func HandleGetBadge(
	repoStore core.RepoStore,
	buildStore core.BuildStore,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		reportID := c.Param("id")
		repo, err := repoStore.Find(&core.Repo{ReportID: reportID})
		if err != nil {
			c.String(404, "repository not found")
			return
		}
		coverage := 0
		if build, err := buildStore.LatestOnBranch(repo.ID, repo.Branch, 0); err == nil {
			coverage = int(build.Coverage * 100)
		}
		data, err := badge.RenderBytes(
			"Covergates",
			fmt.Sprintf("%d%%", coverage),
			"#00838F",
		)
		if err != nil {
			c.String(500, err.Error())
			return
		}
		c.Header("Cache-Control", "max-age=600")
		c.Data(200, "image/svg+xml", data)
	}
}
