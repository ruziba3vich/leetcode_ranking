// @title Leetcoders API
// @version 1.0
// @description Leetcoders API Documentation
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.url http://www.swagger.io/support
// @contact.email support@swagger.io

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
	"github.com/robfig/cron"
	"github.com/ruziba3vich/leetcode_ranking/db/users_storage"
	_ "github.com/ruziba3vich/leetcode_ranking/docs"
	custom_http "github.com/ruziba3vich/leetcode_ranking/internal/http"
	"github.com/ruziba3vich/leetcode_ranking/internal/pkg/config"
	"github.com/ruziba3vich/leetcode_ranking/internal/pkg/helper"
	"github.com/ruziba3vich/leetcode_ranking/internal/service"
	"github.com/ruziba3vich/leetcode_ranking/internal/storage"
	logger "github.com/ruziba3vich/prodonik_lgger"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.uber.org/fx"
)

func main() {
	fx.New(
		fx.Provide(
			config.Load,
			newLogger,
			helper.NewDB,
			storage.NewStorage,
			newUsersStorage,
			service.NewLeetCodeClient,
			service.NewUserService,
			custom_http.NewHandler,
			newEngine,
		),
		fx.Invoke(
			// startCron,
			registerHandlerRoutes,
			runHTTPServer,
		),
	).Run()
}

func newLogger(cfg *config.Config) *logger.Logger {
	l, err := logger.NewLogger(cfg.LogFilePath)
	if err != nil {
		log.Fatalf("failed to init logger: %v", err)
	}
	return l
}

func newUsersStorage(db *sql.DB) users_storage.Querier {
	return users_storage.New(db)
}

func registerHandlerRoutes(h *custom_http.Handler, router *gin.Engine) {
	api := router.Group("/api/v1/")
	{
		api.POST("/add-user", h.CreateUser)
		api.GET("/get-users", h.GetUsersByCountry)
		api.POST("/sync-leaderboard", h.SyncLeaderboard)
		api.POST("/stop-syncing", h.StopSyncing)
		api.GET("/sync-status", h.GetSyncingStatus)
	}
}

func newEngine() *gin.Engine {
	engine := gin.Default()

	// Allow all origins
	engine.Use(cors.New(cors.Config{
		AllowAllOrigins:  true,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	engine.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	return engine
}

func runHTTPServer(
	lc fx.Lifecycle,
	cfg *config.Config,
	log *logger.Logger,
	router *gin.Engine,
) {

	srv := &http.Server{
		Addr:    ":" + cfg.AppPort,
		Handler: router,
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			log.Infof("Starting HTTP server on %s", srv.Addr)
			go func() {
				if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					log.Error("HTTP server stopped with error", map[string]any{"error": err})
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			log.Info("Stopping HTTP server...")
			shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			return srv.Shutdown(shutdownCtx)
		},
	})
}

func startCron(srv service.UserService) {
	log.Println("cron started")
	c := cron.New()

	c.AddFunc("0 0 * * *", func() {
		createCron(srv)
	})
}

func createCron(srv service.UserService) error {
	ctx, cancel := context.WithTimeout(context.TODO(), time.Minute*10)
	defer cancel()
	opts := service.SyncOptions{
		StartPage: 1,
		Pages:     30164,
	}
	return srv.SyncLeaderboard(ctx, opts)
}
