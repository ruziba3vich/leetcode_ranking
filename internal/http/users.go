package http

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ruziba3vich/leetcode_ranking/db/users_storage"
	"github.com/ruziba3vich/leetcode_ranking/internal/dto"
	"github.com/ruziba3vich/leetcode_ranking/internal/errors_"
	"github.com/ruziba3vich/leetcode_ranking/internal/service"
	logger "github.com/ruziba3vich/prodonik_lgger"
)

type Handler struct {
	srv    service.UserService
	logger *logger.Logger
}

// CreateUser godoc
// @Summary     Create a user by fetching data from LeetCode and persisting it
// @Description Takes a username, scrapes public data from LeetCode, and stores it in Postgres.
// @Tags        users
// @Accept      json
// @Produce     json
// @Param       body  body     dto.CreateUserRequest  true  "Create user payload"
// @Success     201   {object} map[string]interface{} "data: created user object"
// @Failure     400   {object} map[string]string      "error: bad request"
// @Failure     404   {object} map[string]string      "error: user not available"
// @Failure     500   {object} map[string]string      "error: internal server error"
// @Router      /users [post]
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

// GetUsersByCountry godoc
// @Summary     List users by country (paginated, ranked)
// @Description Returns users filtered by 2-letter country code, ordered by total_problems_solved DESC, then total_submissions ASC, then username ASC.
// @Tags        users
// @Accept      json
// @Produce     json
// @Param       country  query    string true  "ISO-3166-1 alpha-2 country code (e.g., US, CN, SG)"
// @Param       page     query    int    true  "Page number (1-based)"
// @Param       limit    query    int    true  "Page size (1–100)"
// @Success     200      {object} map[string]interface{} "data: array of users"
// @Failure     400      {object} map[string]string      "error: validation message"
// @Failure     500      {object} map[string]string      "error: internal server error"
// @Router      /users [get]
func (h *Handler) GetUsersByCountry(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	var req dto.GetUsersByCountry
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(req.Country) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "country code is not provided"})
		return
	}

	offset := (req.Page - 1) * req.Limit

	response, err := h.srv.GetUsersByCountry(ctx, &users_storage.GetUsersByCountryParams{
		CountryCode: sql.NullString{String: req.Country, Valid: true},
		Limit:       int32(req.Limit),
		Offset:      int32(offset),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	response.PageLimit = req.PageLimit
	c.JSON(http.StatusOK, gin.H{"data": response})
}
