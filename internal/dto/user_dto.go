package dto

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
)
