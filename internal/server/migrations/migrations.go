package migrations

import (
	"embed"
	"github.com/pochtalexa/ya-practicum-metrics/internal/server/storage"
	"github.com/pressly/goose/v3"
)

//go:embed *.sql
var SQLFiles embed.FS

func ApplyMigrations() error {
	db := storage.DBstorage.DBconn
	fsys := SQLFiles

	goose.SetBaseFS(fsys)
	goose.SetSequential(true)

	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}

	if err := goose.Up(db, "."); err != nil {
		return err
	}

	return nil
}
