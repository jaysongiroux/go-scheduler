package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jaysongiroux/go-scheduler/internal/api/handlers"
	"github.com/jaysongiroux/go-scheduler/internal/api/middleware"
	"github.com/jaysongiroux/go-scheduler/internal/api/web"
	"github.com/jaysongiroux/go-scheduler/internal/config"
	"github.com/jaysongiroux/go-scheduler/internal/db"
	"github.com/jaysongiroux/go-scheduler/internal/db/services/account"
	"github.com/jaysongiroux/go-scheduler/internal/db/services/calendar"
	"github.com/jaysongiroux/go-scheduler/internal/db/services/event"
	reminder "github.com/jaysongiroux/go-scheduler/internal/db/services/reminders"
	"github.com/jaysongiroux/go-scheduler/internal/db/services/webhook"
	"github.com/jaysongiroux/go-scheduler/internal/logger"
	"github.com/jaysongiroux/go-scheduler/internal/workers"
	"github.com/joho/godotenv"
)

func main() {
	if err := logger.Init(logger.InfoLevel); err != nil {
		logger.Fatal("Failed to initialize logger: %v", err)
	}
	defer func() { _ = logger.Sync() }()

	// Load .env file if it exists
	if err := godotenv.Load(); err != nil {
		// Check if running in Docker (common indicators)
		_, dockerEnvExists := os.Stat("/.dockerenv")
		inDocker := dockerEnvExists == nil || os.Getenv("DOCKER_CONTAINER") != ""

		if inDocker {
			logger.Debug(
				"Running in Docker container, using environment variables from container",
			)
		} else {
			logger.Warn("No .env file found, using environment variables from system")
		}
	}

	logger.Info("Starting Go Scheduler")

	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("Failed to load configuration: %v", err)
	}

	dbPool, err := db.NewConnectionPool(cfg)
	if err != nil {
		logger.Fatal("Failed to create database connection pool: %v", err)
	}
	defer func() {
		if err := dbPool.Close(); err != nil {
			logger.Error("Failed to close database connection pool: %v", err)
		}
	}()

	// Run database migrations
	if err := db.RunMigrations(dbPool.DB(), "internal/db/migrations"); err != nil {
		logger.Fatal("Failed to run database migrations: %v", err)
	}

	restLog := logger.New("REST", cfg.LogLevel)
	authLog := logger.New("AUTH", cfg.LogLevel)

	// Initialize repositories
	calendarRepo := calendar.New(dbPool)
	eventRepo := event.New(dbPool)
	reminderRepo := reminder.New(dbPool)
	webhookRepo := webhook.New(dbPool)
	accountRepo := account.New(dbPool)

	// Create webhook dispatcher
	webhookDispatcher := workers.NewWebhookDispatcher(webhookRepo, cfg)

	// Create worker context (cancelled on shutdown)
	workerCtx, workerCancel := context.WithCancel(context.Background())

	// Start embedded workers
	recurringWorker := workers.NewRecurringWorker(eventRepo, webhookDispatcher, cfg)
	go recurringWorker.Start(workerCtx)

	webhookWorker := workers.NewWebhookWorker(webhookRepo, cfg)
	go webhookWorker.Start(workerCtx)

	reminderTriggerWorker := workers.NewReminderTriggerWorker(
		reminderRepo,
		webhookDispatcher,
		cfg.ReminderPollInterval,
		cfg.ReminderBatchSize,
	)
	go reminderTriggerWorker.Start(workerCtx)

	logger.Info("Background workers started")

	// 1. create a REST Server
	restMux := http.NewServeMux()
	servers := make([]*http.Server, 0)
	muxHandler := handlers.NewRestHandler(
		calendarRepo,
		eventRepo,
		reminderRepo,
		webhookRepo,
		accountRepo,
		webhookDispatcher,
		cfg,
	)
	muxHandler.RegisterRoutes(restMux)
	authMiddleware := middleware.AuthMiddleware(cfg.APIKeyHash, authLog)
	corsMiddleware := middleware.CORSMiddleware()
	producerWithMiddleware := middleware.LoggingMiddleware(restLog)(corsMiddleware(authMiddleware(restMux)))
	restServer := web.NewWebServer(cfg, producerWithMiddleware)
	servers = append(servers, restServer)

	// Wait for interrupt signal to gracefully shutdown all servers
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down all servers and workers...")

	// Cancel worker context to stop background workers
	workerCancel()

	// Stop workers gracefully
	recurringWorker.Stop()
	webhookWorker.Stop()
	reminderTriggerWorker.Stop()
	logger.Info("Background workers stopped")

	// Create shutdown context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Shutdown all servers gracefully
	for _, server := range servers {
		if err := server.Shutdown(ctx); err != nil {
			logger.Warn("Server forced to shutdown: %v", err)
		}
	}

	logger.Info("All servers stopped")
}
