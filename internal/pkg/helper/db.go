package helper

import (
	"database/sql"
	"fmt"

	"github.com/ruziba3vich/leetcode_ranking/internal/pkg/config"
)

func NewDB(cfg *config.Config) *sql.DB {
	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		cfg.Postgres.User,
		cfg.Postgres.Password,
		cfg.Postgres.Host,
		cfg.Postgres.Port,
		cfg.Postgres.DBName,
		cfg.Postgres.SSLMode,
	)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		panic(fmt.Errorf("failed to open database: %s", err.Error()))
	}

	if err := db.Ping(); err != nil {
		panic(fmt.Errorf("failed to ping database: %s", err.Error()))
	}
	return db
}
