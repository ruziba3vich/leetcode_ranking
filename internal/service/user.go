package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/ruziba3vich/leetcode_ranking/db/users_storage"
	"github.com/ruziba3vich/leetcode_ranking/internal/dto"
	"github.com/ruziba3vich/leetcode_ranking/internal/storage"
	logger "github.com/ruziba3vich/prodonik_lgger"
)

type userService struct {
	leetCodeClient *LeetCodeClient
	storage        users_storage.Querier
	logger         *logger.Logger
	dbStorage      *storage.Storage
	sync           bool
	syncingPage    int
}

func NewUserService(storage users_storage.Querier, dbStorage *storage.Storage, leetCodeClient *LeetCodeClient, log *logger.Logger) UserService {
	return &userService{
		storage:        storage,
		dbStorage:      dbStorage,
		leetCodeClient: leetCodeClient,
		logger:         log,
	}
}

func (s *userService) SyncOn() {
	s.sync = true
}

func (s *userService) SyncOff() {
	s.sync = false
}

func (s *userService) CreateUser(ctx context.Context, req *dto.CreateUserRequest) (*users_storage.UserDatum, error) {
	data, err := s.fetchAndConvertUser(req.Username)
	if err != nil {
		s.logger.Error("could not fetch user", map[string]any{"error": err.Error(), "username": req.Username})
		return nil, err
	}
	arg := &users_storage.CreateUserParams{
		Username: data.Username,
		UserSlug: data.UserSlug,
		UserAvatar: sql.NullString{
			String: data.UserAvatar,
			Valid:  true,
		},
		CountryCode: sql.NullString{
			String: data.CountryCode,
			Valid:  true,
		},
		CountryName: sql.NullString{
			String: data.CountryName,
			Valid:  true,
		},
		RealName: sql.NullString{
			String: data.RealName,
			Valid:  true,
		},
		Typename: sql.NullString{
			String: data.Typename,
			Valid:  true,
		},
		TotalProblemsSolved: int32(data.TotalProblemsSolved),
		TotalSubmissions:    int32(data.TotalSubmissions),
	}
	if strings.TrimSpace(arg.Username) == "" {
		return nil, fmt.Errorf("username is required")
	}

	u, err := s.storage.CreateUser(ctx, *arg)
	if err != nil {
		s.logger.Errorf("CreateUser: username=%s err=%v", arg.Username, err)
		return nil, err
	}
	s.logger.Infof("CreateUser: username=%s id=%d", u.Username, u.ID)
	return &u, nil
}

func (s *userService) DeleteUserByUsername(ctx context.Context, username string) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return fmt.Errorf("username is required")
	}

	if err := s.storage.DeleteUserByUsername(ctx, username); err != nil {
		s.logger.Errorf("DeleteUserByUsername: username=%s err=%v", username, err)
		return err
	}
	s.logger.Infof("DeleteUserByUsername: username=%s ok", username)
	return nil
}

func (s *userService) GetUserByUsername(ctx context.Context, username string) (*users_storage.UserDatum, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return nil, fmt.Errorf("username is required")
	}

	u, err := s.storage.GetUserByUsername(ctx, username)
	if err != nil {
		s.logger.Errorf("GetUserByUsername: username=%s err=%v", username, err)
		return nil, err
	}
	s.logger.Infof("GetUserByUsername: username=%s id=%d", username, u.ID)
	return &u, nil
}

func (s *userService) GetUsersByCountry(ctx context.Context, arg *users_storage.GetUsersByCountryParams) (*dto.GetUsersByCountryResponse, error) {
	users, err := s.storage.GetUsersByCountry(ctx, *arg)
	if err != nil {
		s.logger.Errorf("GetUsersByCountry: params=%+v err=%v", arg, err)
		return nil, err
	}

	totalCount, err := s.storage.GetAllUsersCountByCountry(ctx, arg.CountryCode)
	if err != nil {
		return nil, errors.New("failed to fetch users count")
	}
	s.logger.Infof("GetUsersByCountry: params=%+v count=%d", arg, len(users))
	return &dto.GetUsersByCountryResponse{
		Users:      users,
		TitalCount: totalCount,
	}, nil
}

func (s *userService) UpdateUserByUsername(ctx context.Context, arg *users_storage.UpdateUserByUsernameParams) (*users_storage.UserDatum, error) {
	if strings.TrimSpace(arg.Username) == "" {
		return nil, fmt.Errorf("username is required")
	}

	u, err := s.storage.UpdateUserByUsername(ctx, *arg)
	if err != nil {
		s.logger.Errorf("UpdateUserByUsername: username=%s err=%v", arg.Username, err)
		return nil, err
	}
	s.logger.Infof("UpdateUserByUsername: username=%s id=%d", arg.Username, u.ID)
	return &u, nil
}
