package build

import (
	"encoding/json"
	"io/ioutil"

	"github.com/covergates/covergates/core"
	"github.com/gin-gonic/gin"
)

// coverallsPayload is the JSON body sent by coveralls reporters to /api/v1/jobs.
type coverallsPayload struct {
	RepoToken          string             `json:"repo_token"`
	ServiceName        string             `json:"service_name"`
	ServiceNumber      string             `json:"service_number"`
	ServiceJobID       string             `json:"service_job_id"`
	ServiceJobNumber   string             `json:"service_job_number"`
	ServicePullRequest string             `json:"service_pull_request"`
	Parallel           bool               `json:"parallel"`
	FlagName           string             `json:"flag_name"`
	Git                *gitInfo           `json:"git"`
	SourceFiles        []*core.SourceFile `json:"source_files"`
}

type gitInfo struct {
	Head   gitHead `json:"head"`
	Branch string  `json:"branch"`
}

type gitHead struct {
	ID          string `json:"id"`
	Message     string `json:"message"`
	AuthorName  string `json:"author_name"`
	AuthorEmail string `json:"author_email"`
}

// parsePayload reads the coveralls payload from either the multipart "json_file"
// form field or a raw JSON request body.
func parsePayload(c *gin.Context) (*coverallsPayload, error) {
	var data []byte
	if file, err := c.FormFile("json_file"); err == nil {
		f, err := file.Open()
		if err != nil {
			return nil, err
		}
		defer f.Close()
		if data, err = ioutil.ReadAll(f); err != nil {
			return nil, err
		}
	} else {
		body, err := ioutil.ReadAll(c.Request.Body)
		if err != nil {
			return nil, err
		}
		data = body
	}
	payload := &coverallsPayload{}
	if err := json.Unmarshal(data, payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func (p *coverallsPayload) commit() string {
	if p.Git != nil {
		return p.Git.Head.ID
	}
	return ""
}

func (p *coverallsPayload) branch() string {
	if p.Git != nil {
		return p.Git.Branch
	}
	return ""
}
