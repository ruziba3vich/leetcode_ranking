package service

import (
	"context"

	"github.com/ruziba3vich/leetcode_ranking/db/users_storage"
	"github.com/ruziba3vich/leetcode_ranking/internal/dto"
)

type UserService interface {
	CollectUsernames(startPage int, maxPages int) ([]string, int, error)
	CreateUser(ctx context.Context, req *dto.CreateUserRequest) (*users_storage.UserDatum, error)
	DeleteUserByUsername(ctx context.Context, username string) error
	FetchLeetCodeUser(ctx context.Context, username string) (OutputUser, error)
	FetchRankingPage(page int) (*ResponseGlobal, error)
	FetchUser(username string) (*ResponseUser, error)
	GetUserByUsername(ctx context.Context, username string) (*users_storage.UserDatum, error)
	GetUsersByCountry(ctx context.Context, arg *users_storage.GetUsersByCountryParams) (*dto.GetUsersByCountryResponse, error)
	SyncLeaderboard(ctx context.Context, opts SyncOptions) error
	UpdateUserByUsername(ctx context.Context, arg *users_storage.UpdateUserByUsernameParams) (*users_storage.UserDatum, error)
}
