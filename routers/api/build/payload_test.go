package build

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func ginContext(req *http.Request) *gin.Context {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	return c
}

func TestParsePayloadRawJSON(t *testing.T) {
	body := `{"repo_token":"abc","service_number":"10","parallel":true,
		"git":{"branch":"master","head":{"id":"sha1","message":"msg","author_name":"a","author_email":"e"}},
		"source_files":[{"name":"a.go","coverage":[1,null,0]}]}`
	req := httptest.NewRequest("POST", "/api/v1/jobs", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	payload, err := parsePayload(ginContext(req))
	if err != nil {
		t.Fatal(err)
	}
	if payload.RepoToken != "abc" || payload.ServiceNumber != "10" || !payload.Parallel {
		t.Fatal("scalar fields not parsed")
	}
	if payload.Git == nil || payload.Git.Branch != "master" || payload.Git.Head.ID != "sha1" {
		t.Fatal("git fields not parsed")
	}
	if len(payload.SourceFiles) != 1 || payload.SourceFiles[0].Name != "a.go" {
		t.Fatal("source files not parsed")
	}
	cov := payload.SourceFiles[0].Coverage
	if *cov[0] != 1 || cov[1] != nil || *cov[2] != 0 {
		t.Fatal("coverage array not parsed (null must become nil)")
	}
}

func TestParsePayloadMultipartJSONFile(t *testing.T) {
	buf := &bytes.Buffer{}
	w := multipart.NewWriter(buf)
	part, _ := w.CreateFormFile("json_file", "coverage.json")
	part.Write([]byte(`{"repo_token":"xyz","source_files":[{"name":"b.go","coverage":[1]}]}`))
	w.Close()

	req := httptest.NewRequest("POST", "/api/v1/jobs", buf)
	req.Header.Set("Content-Type", w.FormDataContentType())

	payload, err := parsePayload(ginContext(req))
	if err != nil {
		t.Fatal(err)
	}
	if payload.RepoToken != "xyz" || len(payload.SourceFiles) != 1 {
		t.Fatal("multipart json_file not parsed")
	}
}
