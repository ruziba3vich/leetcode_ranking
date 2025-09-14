package http

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ruziba3vich/leetcode_ranking/internal/dto"
	"github.com/ruziba3vich/leetcode_ranking/internal/errors_"
	"github.com/ruziba3vich/leetcode_ranking/internal/service"
	logger "github.com/ruziba3vich/prodonik_lgger"
)

type Handler struct {
	srv    service.UserService
	logger *logger.Logger
}

func (h *Handler) CreateUser(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	var req dto.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil || len(req.Username) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad request"})
		return
	}

	response, err := h.srv.CreateUser(ctx, &req)
	if err != nil {
		if errors.Is(err, errors_.ErrUserNotAvailable) {
			c.JSON(http.StatusNotFound, gin.H{"error": errors_.ErrUserNotAvailable.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": response})
}
