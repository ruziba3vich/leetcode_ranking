package service

import (
	"context"

	"github.com/ruziba3vich/leetcode_ranking/db/users_storage"
	"github.com/ruziba3vich/leetcode_ranking/internal/dto"
	"github.com/ruziba3vich/leetcode_ranking/internal/models"
)

type UserService interface {
	CollectUsernames(startPage int, maxPages int) ([]string, int, error)
	DeleteUserByUsername(ctx context.Context, username string) error
	// FetchLeetCodeUser(ctx context.Context, username string) (OutputUser, error) // DEPRICATED
	FetchRankingPage(page int) (*ResponseGlobal, error)
	FetchUser(username string) (*models.StageUserDataParams, error)
	GetUserByUsername(ctx context.Context, username string) (*users_storage.UserDatum, error)
	GetUsersByCountry(ctx context.Context, arg *users_storage.GetUsersByCountryParams) (*dto.GetUsersByCountryResponse, error)
	SyncLeaderboard(ctx context.Context, opts SyncOptions) error
	UpdateUserByUsername(ctx context.Context, arg *users_storage.UpdateUserByUsernameParams) (*users_storage.UserDatum, error)
}
