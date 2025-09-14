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

	"github.com/gin-gonic/gin"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
	"github.com/ruziba3vich/leetcode_ranking/db/users_storage"
	_ "github.com/ruziba3vich/leetcode_ranking/docs"
	custom_http "github.com/ruziba3vich/leetcode_ranking/internal/http"
	"github.com/ruziba3vich/leetcode_ranking/internal/pkg/config"
	"github.com/ruziba3vich/leetcode_ranking/internal/pkg/helper"
	"github.com/ruziba3vich/leetcode_ranking/internal/service"
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
			newUsersStorage,
			service.NewLeetCodeClient,
			service.NewUserService,
			custom_http.NewHandler,
			newEngine,
		),
		fx.Invoke(
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
	}
}

func newEngine() *gin.Engine {
	engine := gin.Default()
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
