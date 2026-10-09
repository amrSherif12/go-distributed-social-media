package user_db_migrations

import (
	"database/sql"
	"embed"
	"go_distributed_social_media/internal/common/database"
)

//go:embed *.sql
var migrationFiles embed.FS

func MigrateUserDB(db *sql.DB) {
	database.RunMigrations(db, migrationFiles)
}
