package config

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strconv"
	"time"

	"github.com/jaysongiroux/go-scheduler/internal/logger"
)

type Config struct {
	DatabaseURL      string
	APIKeyHash       string
	MaxOpenConns     int
	MaxIdleConns     int
	WebServerAddress string
	LogLevel         logger.LogLevel

	// Recurring event generation worker config
	RecurringGenerationInterval time.Duration // How often to run the generation worker
	GenerationWindow            time.Duration // How far ahead to generate instances (e.g., 12 months)
	GenerationBuffer            time.Duration // Extra buffer to prevent gaps

	// Webhook delivery worker config
	WebhookWorkerConcurrency int           // Number of concurrent webhook delivery goroutines
	WebhookBaseRetryDelay    time.Duration // Base delay for exponential backoff
	WebhookMaxRetries        int           // Maximum number of retry attempts
	WebhookTimeoutSeconds    int           // Timeout seconds for webhook delivery
	WebhookMaxBatchSize      int           // Maximum number of items to batch in a single webhook delivery

	// Default page sizes
	DefaultPageSize int // Default page size for API responses

	// Reminder trigger worker config
	ReminderPollInterval time.Duration // How often to poll for reminders
	ReminderBatchSize    int           // How many reminders to process at once

	// ICS sync worker config
	IcsSyncInterval   time.Duration // How often to check for calendars needing sync
	IcsSyncBatchSize  int           // How many calendars to process per batch
	IcsRequestTimeout time.Duration // Timeout for fetching ICS feeds
	IcsEncryptionKey  []byte        // 32-byte key for encrypting credentials
}

const (
	defaultMaxOpenConns     = 10
	defaultMaxIdleConns     = 5
	defaultWebServerAddress = ":8080"

	// Recurring worker defaults
	defaultRecurringGenerationInterval = 1 * time.Hour
	defaultGenerationWindow            = 365 * 96 * time.Hour // 96 months
	defaultGenerationBuffer            = 7 * 24 * time.Hour   // 7 days

	// Webhook worker defaults
	defaultWebhookWorkerConcurrency = 10
	defaultWebhookBaseRetryDelay    = 1 * time.Minute
	defaultWebhookMaxRetries        = 3
	defaultWebhookTimeoutSeconds    = 10
	defaultWebhookMaxBatchSize      = 100

	// Default page size defaults
	defaultDefaultPageSize = 10

	// Reminder trigger worker defaults
	defaultReminderPollInterval = 1 * time.Second
	defaultReminderBatchSize    = 100

	// ICS sync worker defaults
	defaultIcsSyncInterval   = 1 * time.Hour
	defaultIcsSyncBatchSize  = 100
	defaultIcsRequestTimeout = 30 * time.Second
)

func getNumberFromEnv(envVar string, defaultValue int) int {
	num := os.Getenv(envVar)
	if num == "" {
		return defaultValue
	}
	numInt, err := strconv.Atoi(num)
	if err != nil {
		logger.Fatal("Failed to convert %s to integer: %v", envVar, err)
	}
	return numInt
}

func getDurationFromEnv(envVar string, defaultValue time.Duration) time.Duration {
	val := os.Getenv(envVar)
	if val == "" {
		return defaultValue
	}
	duration, err := time.ParseDuration(val)
	if err != nil {
		logger.Fatal("Failed to parse %s as duration: %v", envVar, err)
	}
	return duration
}

func Load() (*Config, error) {
	apiKey := os.Getenv("API_KEY")
	if apiKey == "" {
		logger.Fatal("API_KEY is not set")
	}
	apiKeyHash := sha256.Sum256([]byte(apiKey))
	apiKeyHashString := hex.EncodeToString(apiKeyHash[:])

	maxOpenConns := getNumberFromEnv("DB_MAX_OPEN_CONNS", defaultMaxOpenConns)
	maxIdleConns := getNumberFromEnv("DB_MAX_IDLE_CONNS", defaultMaxIdleConns)
	webServerAddress := os.Getenv("WEB_SERVER_ADDRESS")
	if webServerAddress == "" {
		webServerAddress = defaultWebServerAddress
	}
	LogLevel := logger.ParseLogLevel(os.Getenv("LOG_LEVEL"))
	if LogLevel == 0 {
		LogLevel = logger.InfoLevel
	}
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		logger.Fatal("DATABASE_URL is not set")
	}

	// Recurring worker config
	recurringGenerationInterval := getDurationFromEnv(
		"RECURRING_GENERATION_INTERVAL",
		defaultRecurringGenerationInterval,
	)
	generationWindow := getDurationFromEnv("GENERATION_WINDOW", defaultGenerationWindow)
	generationBuffer := getDurationFromEnv("GENERATION_BUFFER", defaultGenerationBuffer)

	// Webhook worker config
	webhookWorkerConcurrency := getNumberFromEnv(
		"WEBHOOK_WORKER_CONCURRENCY",
		defaultWebhookWorkerConcurrency,
	)
	webhookBaseRetryDelay := getDurationFromEnv(
		"WEBHOOK_BASE_RETRY_DELAY",
		defaultWebhookBaseRetryDelay,
	)
	webhookMaxRetries := getNumberFromEnv("WEBHOOK_MAX_RETRIES", defaultWebhookMaxRetries)

	webhookTimeoutSeconds := getNumberFromEnv(
		"WEBHOOK_TIMEOUT_SECONDS",
		defaultWebhookTimeoutSeconds,
	)
	webhookMaxBatchSize := getNumberFromEnv("WEBHOOK_MAX_BATCH_SIZE", defaultWebhookMaxBatchSize)
	defaultPageSize := getNumberFromEnv("DEFAULT_PAGE_SIZE", defaultDefaultPageSize)

	// Reminder trigger worker config
	reminderPollInterval := getDurationFromEnv(
		"REMINDER_POLL_INTERVAL",
		defaultReminderPollInterval,
	)
	reminderBatchSize := getNumberFromEnv("REMINDER_BATCH_SIZE", defaultReminderBatchSize)

	// ICS sync worker config
	icsSyncInterval := getDurationFromEnv("ICS_SYNC_INTERVAL", defaultIcsSyncInterval)
	icsSyncBatchSize := getNumberFromEnv("ICS_SYNC_BATCH_SIZE", defaultIcsSyncBatchSize)
	icsRequestTimeout := getDurationFromEnv("ICS_REQUEST_TIMEOUT", defaultIcsRequestTimeout)

	// Load ICS encryption key (must be 32 bytes for AES-256)
	icsEncryptionKeyStr := os.Getenv("ICS_ENCRYPTION_KEY")
	var icsEncryptionKey []byte
	if icsEncryptionKeyStr != "" {
		icsEncryptionKey = []byte(icsEncryptionKeyStr)
		if len(icsEncryptionKey) != 32 {
			logger.Fatal(
				"ICS_ENCRYPTION_KEY must be exactly 32 bytes (got %d bytes)",
				len(icsEncryptionKey),
			)
		}
	} else {
		logger.Fatal("ICS_ENCRYPTION_KEY is not set")
	}

	cfg := &Config{
		DatabaseURL:      databaseURL,
		APIKeyHash:       apiKeyHashString,
		MaxOpenConns:     maxOpenConns,
		MaxIdleConns:     maxIdleConns,
		WebServerAddress: webServerAddress,
		LogLevel:         LogLevel,

		// Recurring worker
		RecurringGenerationInterval: recurringGenerationInterval,
		GenerationWindow:            generationWindow,
		GenerationBuffer:            generationBuffer,

		// Webhook worker
		WebhookWorkerConcurrency: webhookWorkerConcurrency,
		WebhookBaseRetryDelay:    webhookBaseRetryDelay,
		WebhookMaxRetries:        webhookMaxRetries,
		WebhookTimeoutSeconds:    webhookTimeoutSeconds,
		WebhookMaxBatchSize:      webhookMaxBatchSize,

		// Default page size
		DefaultPageSize: defaultPageSize,

		// Reminder trigger worker
		ReminderPollInterval: reminderPollInterval,
		ReminderBatchSize:    reminderBatchSize,

		// ICS sync worker
		IcsSyncInterval:   icsSyncInterval,
		IcsSyncBatchSize:  icsSyncBatchSize,
		IcsRequestTimeout: icsRequestTimeout,
		IcsEncryptionKey:  icsEncryptionKey,
	}
	return cfg, nil
}
