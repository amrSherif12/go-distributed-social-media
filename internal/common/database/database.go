package database

import (
	"database/sql"
	"embed"
	"go_distributed_social_media/internal/common/config"
	"log"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/pressly/goose/v3"
)

func NewDB(dbName string, dbHost string, dbPort string) (*sql.DB, error) {

	sqlCfg := mysql.Config{
		User:                 config.Required(config.SQLDbUser),
		Passwd:               config.Required(config.SQLDbPass),
		Net:                  "tcp",
		Addr:                 dbHost + ":" + dbPort,
		DBName:               dbName,
		ParseTime:            true,
		AllowNativePasswords: true,
	}

	var lastErr error
	for attempt := 0; attempt < 30; attempt++ {
		db, err := sql.Open("mysql", sqlCfg.FormatDSN())
		if err != nil {
			lastErr = err
			time.Sleep(time.Second)
			continue
		}
		if err := db.Ping(); err != nil {
			lastErr = err
			_ = db.Close()
			time.Sleep(time.Second)
			continue
		}
		return db, nil
	}
	return nil, lastErr
}

func RunMigrations(db *sql.DB, migrationFS embed.FS) {
	goose.SetBaseFS(migrationFS)

	if err := goose.SetDialect("mysql"); err != nil {
		log.Fatalf("goose set dialect failed: %v", err)
	}

	log.Println("Running database migrations from embedded files...")

	if err := goose.Up(db, "."); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	log.Println("Database migrations applied successfully!")
}
