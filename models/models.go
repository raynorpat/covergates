package models

import (
	"github.com/covergates/covergates/core"
	"github.com/covergates/covergates/modules/util"
	"gorm.io/gorm"
)

var (
	tables []interface{}
)

type databaseService struct {
	db *gorm.DB
}

// NewDatabaseService with GORM
func NewDatabaseService(db *gorm.DB) core.DatabaseService {
	return &databaseService{
		db: db,
	}
}

func (store *databaseService) Session() *gorm.DB {
	return store.db.Session(&gorm.Session{})
}

func (store *databaseService) Migrate() error {
	return migrate(store.db)
}

func init() {
	tables = append(tables,
		&Report{},
		&ReportComment{},
		&Reference{},
		&Coverage{},
		&User{},
		&Repo{},
		&RepoSetting{},
		&RepoHook{},
		&OAuthToken{},
		&Build{},
		&Job{},
	)
}

func migrate(db *gorm.DB) error {
	if err := db.AutoMigrate(tables...); err != nil {
		return err
	}
	return backfillRepoTokens(db)
}

func backfillRepoTokens(db *gorm.DB) error {
	var repos []*Repo
	if err := db.Where("token = '' OR token IS NULL").Find(&repos).Error; err != nil {
		return err
	}
	for _, r := range repos {
		if err := db.Model(r).Update("token", util.GenerateToken()).Error; err != nil {
			return err
		}
	}
	return nil
}
