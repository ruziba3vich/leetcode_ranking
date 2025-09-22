package service

import (
	"context"

	"github.com/ruziba3vich/leetcode_ranking/db/users_storage"
	"github.com/ruziba3vich/leetcode_ranking/internal/dto"
	"github.com/ruziba3vich/leetcode_ranking/internal/models"
)

type UserService interface {
	CreateUser(ctx context.Context, req *dto.CreateUserRequest) (*users_storage.UserDatum, error)
	DeleteUserByUsername(ctx context.Context, username string) error
	GetUserByUsername(ctx context.Context, username string) (*users_storage.UserDatum, error)
	GetUserData(ctx context.Context, username string) (*models.StageUserDataParams, error)
	GetUsersByCountry(ctx context.Context, arg *users_storage.GetUsersByCountryParams) (*dto.GetUsersByCountryResponse, error)
	SyncLeaderboard(ctx context.Context, opts SyncOptions) error
	UpdateUserByUsername(ctx context.Context, arg *users_storage.UpdateUserByUsernameParams) (*users_storage.UserDatum, error)
	SyncOff()
	SyncOn()
	GetSyncStatus() *dto.GetSyncStatusResponse
}
