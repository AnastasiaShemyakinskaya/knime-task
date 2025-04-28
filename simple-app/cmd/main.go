package main

import (
	"context"
	"go.uber.org/zap"
	"net/http"
	"os"
	"os/signal"
	"simple-app/api"
	"simple-app/internal/example"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"

	"github.com/AnastasiaShemyakinskaya/knime-task/lib"
)

func main() {
	// Initialize structured logger
	logger, err := zap.NewProduction()
	if err != nil {
		logger.Fatal("failed init log", zap.Error(err))
	}

	// Fetch environment variables for database and NATS urls
	dbUrl := os.Getenv("DB_URL")
	natsUrl := os.Getenv("NATS_URL")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize PostgreSQL connection pool
	pool, err := pgxpool.New(ctx, dbUrl)
	if err != nil {
		logger.Fatal("failed init db", zap.Error(err))
	}
	defer pool.Close()

	// Initialize connection to NATS
	nc, err := nats.Connect(natsUrl, nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(10),
		nats.ReconnectWait(time.Second))
	if err != nil {
		logger.Fatal("failed init nats", zap.Error(err))
	}

	// Create and start the publisher lib for sending message
	publisher := lib.NewPublisher(nc, pool, logger)
	publisher.Run(ctx)

	// Initialize the application service layer (Repo -> Service -> API)
	a := InitApi(ctx, pool, publisher, logger)

	// Register the endpoint for adding a message
	router := gin.Default()
	router.POST("/add", func(c *gin.Context) {
		api.AddMessage(a)(c)
	})

	srv := &http.Server{
		Addr:    ":8080",
		Handler: router,
	}

	// Start the HTTP server asynchronously
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("listen: ", zap.Error(err))
		}
	}()

	// Prepare to gracefully shut down on system interrupt
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	<-quit
	logger.Warn("shutting down gracefully, press Ctrl+C again to force")

	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// Attempt to gracefully shut down the HTTP server
	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatal("Server forced to shutdown: ", zap.Error(err))
	}
	pool.Close()
	nc.Close()
}

func InitApi(ctx context.Context, pool *pgxpool.Pool, publisher lib.Publisher, logger *zap.Logger) *api.Api {
	repo := example.NewRepo(pool)
	service := example.NewService(repo, publisher, logger)
	service.Run(ctx)
	a := api.New(service)
	return a
}
