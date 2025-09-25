package http

import (
	"context"
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

func NewHandler(srv service.UserService, logger *logger.Logger) *Handler {
	return &Handler{
		srv:    srv,
		logger: logger,
	}
}

// CreateUser godoc
// @Summary     Create a user by fetching data from LeetCode and persisting it
// @Description Takes a username, scrapes public data from LeetCode, and stores it in Postgres.
// @Tags        users
// @Accept      json
// @Produce     json
// @Param       body  body     dto.CreateUserRequest  true  "Create user payload"
// @Success     201   {object} users_storage.UserDatum       "Created user object"
// @Failure     400   {object} map[string]string      "Bad request"
// @Failure     404   {object} map[string]string      "User not available"
// @Failure     500   {object} map[string]string      "Internal server error"
// @Router      /api/v1/add-user [post]
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

	c.JSON(http.StatusCreated, response)
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
// @Success     200      {object} dto.GetUsersByCountryResponse "List of users by country"
// @Failure     400      {object} map[string]string     "Validation message"
// @Failure     500      {object} map[string]string     "Internal server error"
// @Router      /api/v1/get-users [get]
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
		Country:   req.Country,
		LimitArg:  int32(req.Limit),
		OffsetArg: int32(offset),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	response.PageLimit = req.PageLimit
	c.JSON(http.StatusOK, response)
}

// SyncLeaderboard godoc
// @Summary     Start leaderboard syncing
// @Description Starts the background process to sync the leaderboard from LeetCode.
// @Tags        leaderboard
// @Accept      json
// @Produce     json
// @Param       body  body     dto.StartSyncingReq  true  "Sync start request (page number to begin from)"
// @Success     200   {object} map[string]string    "Syncing started"
// @Failure     400   {object} map[string]string    "Invalid request"
// @Router      /api/v1/sync-leaderboard [post]
func (h *Handler) SyncLeaderboard(c *gin.Context) {
	var req dto.StartSyncingReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	stat := h.srv.GetSyncStatus()
	if stat.IsOn {
		c.JSON(http.StatusBadRequest, gin.H{"error": "syncing is already on"})
		return
	}

	h.srv.SyncOn()
	go h.srv.SyncLeaderboard(c.Request.Context(), service.SyncOptions{StartPage: req.Page, Workers: 4})
	c.JSON(http.StatusOK, gin.H{"response": "syncing started"})
}

// StopSyncing godoc
// @Summary     Stop leaderboard syncing
// @Description Stops the ongoing background leaderboard sync job.
// @Tags        leaderboard
// @Accept      json
// @Produce     json
// @Success     200   {object} map[string]string    "Syncing stopped"
// @Failure     400   {object} map[string]string    "Invalid request"
// @Router      /api/v1/stop-syncing [post]
func (h *Handler) StopSyncing(c *gin.Context) {
	h.srv.SyncOff()
	c.JSON(http.StatusOK, gin.H{"response": "syncing stopped"})
}

// GetSyncingStatus godoc
// @Summary     Get syncing status
// @Description Returns whether the leaderboard syncing process is active and current progress info.
// @Tags        leaderboard
// @Accept      json
// @Produce     json
// @Success     200   {object} dto.GetSyncStatusResponse "Current sync status"
// @Router      /api/v1/sync-status [get]
func (h *Handler) GetSyncingStatus(c *gin.Context) {
	c.JSON(http.StatusOK, h.srv.GetSyncStatus())
}
