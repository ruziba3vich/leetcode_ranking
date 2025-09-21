package storage

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/lib/pq"
	"github.com/ruziba3vich/leetcode_ranking/internal/models"
)

const (
	userDataTable        = "user_data"
	stagingUserDataTable = "user_data_staging"
)

type Storage struct {
	db *sql.DB
}

// UpsertUserData copies all records into staging table, then merges into actual table with upsert
func (s *Storage) UpsertUserData(ctx context.Context, records []*models.StageUserDataParams) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Clean staging table
	if _, err := tx.ExecContext(ctx, fmt.Sprintf("TRUNCATE %s;", stagingUserDataTable)); err != nil {
		return fmt.Errorf("truncate staging: %w", err)
	}

	// Prepare COPY INTO staging
	stmt, err := tx.Prepare(pq.CopyIn(
		stagingUserDataTable,
		"username",
		"user_slug",
		"user_avatar",
		"country_code",
		"country_name",
		"real_name",
		"typename",
		"total_problems_solved",
		"total_submissions",
	))
	if err != nil {
		return fmt.Errorf("prepare copyin: %w", err)
	}

	for _, r := range records {
		if _, err := stmt.Exec(
			r.Username,
			r.UserSlug,
			r.UserAvatar,
			r.CountryCode,
			r.CountryName,
			r.RealName,
			r.Typename,
			r.TotalProblemsSolved,
			r.TotalSubmissions,
		); err != nil {
			return fmt.Errorf("copyin exec: %w", err)
		}
	}

	if _, err := stmt.Exec(); err != nil {
		return fmt.Errorf("finalize copyin: %w", err)
	}
	if err := stmt.Close(); err != nil {
		return fmt.Errorf("close stmt: %w", err)
	}

	// Merge into actual table with upsert
	mergeQuery := fmt.Sprintf(`
		INSERT INTO %s (
			username,
			user_slug,
			user_avatar,
			country_code,
			country_name,
			real_name,
			typename,
			total_problems_solved,
			total_submissions
		)
		SELECT
			username,
			user_slug,
			user_avatar,
			country_code,
			country_name,
			real_name,
			typename,
			total_problems_solved,
			total_submissions
		FROM %s
		ON CONFLICT (username) DO UPDATE SET
			user_slug = EXCLUDED.user_slug,
			user_avatar = EXCLUDED.user_avatar,
			country_code = EXCLUDED.country_code,
			country_name = EXCLUDED.country_name,
			real_name = EXCLUDED.real_name,
			typename = EXCLUDED.typename,
			total_problems_solved = EXCLUDED.total_problems_solved,
			total_submissions = EXCLUDED.total_submissions;
	`, userDataTable, stagingUserDataTable)

	if _, err := tx.ExecContext(ctx, mergeQuery); err != nil {
		return fmt.Errorf("merge into actual table: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}
