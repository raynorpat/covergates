package models

import (
	"errors"

	"gorm.io/gorm"
)

// PullRequestComment tracks the latest coverage comment posted to a pull request,
// so a new build can delete the previous comment before posting a fresh one.
type PullRequestComment struct {
	gorm.Model
	RepoID    uint `gorm:"index:idx_pr_comment,unique"`
	Number    int  `gorm:"index:idx_pr_comment,unique"`
	CommentID int
}

// FindPullRequestComment returns the tracked comment ID, or 0 if none recorded.
func (store *RepoStore) FindPullRequestComment(repoID uint, number int) (int, error) {
	session := store.DB.Session()
	c := &PullRequestComment{}
	err := session.Where(&PullRequestComment{RepoID: repoID, Number: number}).First(c).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return c.CommentID, nil
}

// UpdatePullRequestComment records the comment ID for a repo's pull request.
func (store *RepoStore) UpdatePullRequestComment(repoID uint, number, commentID int) error {
	session := store.DB.Session()
	c := &PullRequestComment{RepoID: repoID, Number: number}
	if err := session.Where(c).FirstOrCreate(c).Error; err != nil {
		return err
	}
	c.CommentID = commentID
	return session.Save(c).Error
}
