package models

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/covergates/covergates/core"
	"gorm.io/gorm"
)

var errBuildFields = errors.New("build must have a repository id")

// Build is a coverage build belonging to a repository.
type Build struct {
	gorm.Model
	RepoID          uint `gorm:"index"`
	Number          int  `gorm:"index"`
	ServiceName     string
	ServiceNumber   string `gorm:"index"`
	Commit          string `gorm:"index"`
	Branch          string `gorm:"index"`
	PullRequest     int
	Status          string
	Parallel        bool
	Coverage        float64
	CoverageChange  float64
	BaseBuildID     uint
	BaseBuildNumber int
	CommitMessage   string
	AuthorName      string
	AuthorEmail     string
	CoverageData    []byte
	Jobs            []*Job
	FinishedAt      time.Time
}

// Job is a single coverage submission within a build.
type Job struct {
	gorm.Model
	BuildID          uint `gorm:"index"`
	ServiceJobID     string
	ServiceJobNumber string
	Coverage         float64
	SourceFilesData  []byte
}

// BuildStore persists builds and jobs.
type BuildStore struct {
	DB core.DatabaseService
}

// Create a new build, assigning the next per-repo Number.
func (store *BuildStore) Create(b *core.Build) error {
	if b.RepoID == 0 {
		return errBuildFields
	}
	return store.DB.Session().Transaction(func(tx *gorm.DB) error {
		var result struct{ Max int }
		if err := tx.Model(&Build{}).
			Where("repo_id = ?", b.RepoID).
			Select("COALESCE(MAX(number), 0) as max").
			Scan(&result).Error; err != nil {
			return err
		}
		m := &Build{}
		copyBuildToModel(m, b)
		m.Number = result.Max + 1
		if err := tx.Create(m).Error; err != nil {
			return err
		}
		b.ID = m.ID
		b.Number = m.Number
		b.CreatedAt = m.CreatedAt
		return nil
	})
}

// AddJob appends a job to an existing build.
func (store *BuildStore) AddJob(b *core.Build, j *core.Job) error {
	data, err := json.Marshal(j.SourceFiles)
	if err != nil {
		return err
	}
	m := &Job{
		BuildID:          b.ID,
		ServiceJobID:     j.ServiceJobID,
		ServiceJobNumber: j.ServiceJobNumber,
		Coverage:         j.Coverage,
		SourceFilesData:  data,
	}
	if err := store.DB.Session().Create(m).Error; err != nil {
		return err
	}
	j.ID = m.ID
	j.BuildID = b.ID
	return nil
}

// FindByServiceNumber finds a build by repo and CI service number.
func (store *BuildStore) FindByServiceNumber(repoID uint, serviceNumber string) (*core.Build, error) {
	m := &Build{}
	err := store.DB.Session().
		Where(&Build{RepoID: repoID, ServiceNumber: serviceNumber}).
		First(m).Error
	if err != nil {
		return nil, err
	}
	return m.ToCoreBuild(false), nil
}

// FindByNumber finds a build by repo and per-repo number, with merged source files.
func (store *BuildStore) FindByNumber(repoID uint, number int) (*core.Build, error) {
	if number <= 0 {
		return nil, gorm.ErrRecordNotFound
	}
	m := &Build{}
	err := store.DB.Session().
		Where(&Build{RepoID: repoID, Number: number}).
		First(m).Error
	if err != nil {
		return nil, err
	}
	return m.ToCoreBuild(true), nil
}

// FindByCommit finds the most recent build for a repo and commit.
func (store *BuildStore) FindByCommit(repoID uint, commit string) (*core.Build, error) {
	m := &Build{}
	err := store.DB.Session().
		Where(&Build{RepoID: repoID, Commit: commit}).
		Order("number desc").
		First(m).Error
	if err != nil {
		return nil, err
	}
	return m.ToCoreBuild(false), nil
}

// List builds for a repo, optionally filtered by branch, newest first.
func (store *BuildStore) List(repoID uint, branch string, limit, offset int) ([]*core.Build, error) {
	if limit <= 0 {
		limit = 30
	}
	var builds []*Build
	condition := &Build{RepoID: repoID}
	if branch != "" {
		condition.Branch = branch
	}
	err := store.DB.Session().
		Where(condition).
		Order("number desc").
		Limit(limit).Offset(offset).
		Find(&builds).Error
	if err != nil {
		return nil, err
	}
	result := make([]*core.Build, len(builds))
	for i, m := range builds {
		result[i] = m.ToCoreBuild(false)
	}
	return result, nil
}

// Jobs of a build, each with its uploaded source files.
func (store *BuildStore) Jobs(buildID uint) ([]*core.Job, error) {
	var jobs []*Job
	err := store.DB.Session().
		Where(&Job{BuildID: buildID}).
		Order("id asc").
		Find(&jobs).Error
	if err != nil {
		return nil, err
	}
	result := make([]*core.Job, len(jobs))
	for i, m := range jobs {
		result[i] = m.toCoreJob()
	}
	return result, nil
}

// LatestOnBranch returns the most recent done build on a branch,
// excluding excludeBuildID (pass 0 to exclude nothing).
func (store *BuildStore) LatestOnBranch(repoID uint, branch string, excludeBuildID uint) (*core.Build, error) {
	m := &Build{}
	session := store.DB.Session().
		Where(&Build{RepoID: repoID, Branch: branch, Status: string(core.BuildDone)})
	if excludeBuildID > 0 {
		session = session.Where("id <> ?", excludeBuildID)
	}
	if err := session.Order("number desc").First(m).Error; err != nil {
		return nil, err
	}
	return m.ToCoreBuild(false), nil
}

// Update persists changes to a build.
func (store *BuildStore) Update(b *core.Build) error {
	m := &Build{}
	if err := store.DB.Session().First(m, b.ID).Error; err != nil {
		return err
	}
	copyBuildToModel(m, b)
	if b.SourceFiles != nil {
		data, err := json.Marshal(b.SourceFiles)
		if err != nil {
			return err
		}
		m.CoverageData = data
	}
	return store.DB.Session().Save(m).Error
}

// ToCoreBuild converts the model to a core.Build. When withFiles is true the
// merged source files are unmarshaled from CoverageData.
func (m *Build) ToCoreBuild(withFiles bool) *core.Build {
	b := &core.Build{
		ID:              m.ID,
		RepoID:          m.RepoID,
		Number:          m.Number,
		ServiceName:     m.ServiceName,
		ServiceNumber:   m.ServiceNumber,
		Commit:          m.Commit,
		Branch:          m.Branch,
		PullRequest:     m.PullRequest,
		Status:          core.BuildStatus(m.Status),
		Parallel:        m.Parallel,
		Coverage:        m.Coverage,
		CoverageChange:  m.CoverageChange,
		BaseBuildID:     m.BaseBuildID,
		BaseBuildNumber: m.BaseBuildNumber,
		CommitMessage:   m.CommitMessage,
		AuthorName:      m.AuthorName,
		AuthorEmail:     m.AuthorEmail,
		CreatedAt:       m.CreatedAt,
		FinishedAt:      m.FinishedAt,
	}
	if withFiles && len(m.CoverageData) > 0 {
		var files []*core.SourceFile
		if err := json.Unmarshal(m.CoverageData, &files); err == nil {
			b.SourceFiles = files
		}
	}
	return b
}

func (m *Job) toCoreJob() *core.Job {
	j := &core.Job{
		ID:               m.ID,
		BuildID:          m.BuildID,
		ServiceJobID:     m.ServiceJobID,
		ServiceJobNumber: m.ServiceJobNumber,
		Coverage:         m.Coverage,
		CreatedAt:        m.CreatedAt,
	}
	if len(m.SourceFilesData) > 0 {
		var files []*core.SourceFile
		if err := json.Unmarshal(m.SourceFilesData, &files); err == nil {
			j.SourceFiles = files
		}
	}
	return j
}

// copyBuildToModel copies scalar fields from core.Build onto the model,
// leaving gorm.Model (ID/timestamps) and CoverageData untouched.
func copyBuildToModel(dst *Build, src *core.Build) {
	dst.RepoID = src.RepoID
	dst.Number = src.Number
	dst.ServiceName = src.ServiceName
	dst.ServiceNumber = src.ServiceNumber
	dst.Commit = src.Commit
	dst.Branch = src.Branch
	dst.PullRequest = src.PullRequest
	dst.Status = string(src.Status)
	dst.Parallel = src.Parallel
	dst.Coverage = src.Coverage
	dst.CoverageChange = src.CoverageChange
	dst.BaseBuildID = src.BaseBuildID
	dst.BaseBuildNumber = src.BaseBuildNumber
	dst.CommitMessage = src.CommitMessage
	dst.AuthorName = src.AuthorName
	dst.AuthorEmail = src.AuthorEmail
	dst.FinishedAt = src.FinishedAt
}
