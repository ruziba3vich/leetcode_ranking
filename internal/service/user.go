package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/ruziba3vich/leetcode_ranking/db/users_storage"
	logger "github.com/ruziba3vich/prodonik_lgger"
)

type UserService struct {
	storage users_storage.Querier
	logger  logger.Logger
}

func NewUserService(storage users_storage.Querier, log logger.Logger) *UserService {
	return &UserService{storage: storage, logger: log}
}

func (s *UserService) CreateUser(ctx context.Context, arg *users_storage.CreateUserParams) (*users_storage.UserDatum, error) {
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

func (s *UserService) DeleteUserByUsername(ctx context.Context, username string) error {
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

func (s *UserService) GetUserByUsername(ctx context.Context, username string) (*users_storage.UserDatum, error) {
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

func (s *UserService) GetUsersByCountry(ctx context.Context, arg *users_storage.GetUsersByCountryParams) ([]users_storage.UserDatum, error) {
	users, err := s.storage.GetUsersByCountry(ctx, *arg)
	if err != nil {
		s.logger.Errorf("GetUsersByCountry: params=%+v err=%v", arg, err)
		return nil, err
	}
	s.logger.Infof("GetUsersByCountry: params=%+v count=%d", arg, len(users))
	return users, nil
}

func (s *UserService) UpdateUserByUsername(ctx context.Context, arg *users_storage.UpdateUserByUsernameParams) (*users_storage.UserDatum, error) {
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
