package dto

import "github.com/ruziba3vich/leetcode_ranking/db/users_storage"

type (
	CreateUserRequest struct {
		Username string `json:"username"`
	}

	GetUsersByCountry struct {
		PageLimit
		Country string `form:"country" binding:"required"`
	}

	PageLimit struct {
		Page  int `form:"page"  binding:"required,min=1"`
		Limit int `form:"limit" binding:"required,min=1,max=100"`
	}

	GetUsersByCountryResponse struct {
		Users      []users_storage.UserDatum `json:"users"`
		TitalCount int64                     `json:"total_count"`
		PageLimit
	}

	StartSyncingReq struct {
		Page int `json:"page"`
	}
)
