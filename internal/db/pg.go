package db

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/jaysongiroux/go-scheduler/internal/config"
	"github.com/jaysongiroux/go-scheduler/internal/logger"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

type ConnectionPool struct {
	db *sqlx.DB
}

// PostgresStore implements the Store interface for PostgreSQL
type PostgresStore struct {
	config *config.Config
	db     *sqlx.DB
}

func NewConnectionPool(
	cfg *config.Config,
) (*ConnectionPool, error) {
	db := sqlx.MustConnect("postgres", cfg.DatabaseURL)

	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(1 * time.Minute)

	if err := db.Ping(); err != nil {
		logger.Fatal("Failed to ping database: %v", err)
	}

	logger.Info(
		"Database connection pool established (max_conns=%d, max_idle_conns=%d)",
		cfg.MaxOpenConns,
		cfg.MaxIdleConns,
	)

	return &ConnectionPool{
		db: db,
	}, nil
}

func (p *ConnectionPool) DB() *sqlx.DB {
	return p.db
}

func (p *ConnectionPool) SetDB(db *sqlx.DB) {
	p.db = db
}

func (p *ConnectionPool) Close() error {
	return p.db.Close()
}

// RunMigrations reads and executes all SQL migration files from a directory
func RunMigrations(db *sqlx.DB, migrationsDir string) error {
	files, err := os.ReadDir(migrationsDir)
	if err != nil {
		return err
	}

	// Filter and sort SQL files
	var sqlFiles []string
	for _, f := range files {
		if !f.IsDir() && filepath.Ext(f.Name()) == ".sql" {
			sqlFiles = append(sqlFiles, f.Name())
		}
	}
	sort.Strings(sqlFiles)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	for _, filename := range sqlFiles {
		path := filepath.Join(migrationsDir, filename)
		content, err := os.ReadFile(filepath.Clean(path))
		if err != nil {
			return err
		}

		logger.Info("Running migration: %s", filename)
		if _, err := db.ExecContext(ctx, string(content)); err != nil {
			return err
		}
	}

	logger.Info("Database migrations completed successfully")
	return nil
}

// Migrate executes a SQL schema directly (for inline migrations)
func Migrate(db *sqlx.DB, schema string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := db.ExecContext(ctx, schema)
	if err != nil {
		logger.Fatal("Failed to run database migrations: %v", err)
	}

	logger.Info("Database migrations completed successfully")
	return nil
}

// NewPostgresStore creates a new PostgreSQL store instance
func NewPostgresStore(cfg *config.Config) (*PostgresStore, error) {
	pool, err := NewConnectionPool(cfg)
	if err != nil {
		logger.Fatal("Failed to create connection pool: %v", err)
	}

	store := &PostgresStore{
		config: cfg,
		db:     pool.DB(),
	}

	// Run migrations
	if err := RunMigrations(store.db, "internal/db/migrations"); err != nil {
		logger.Fatal("Failed to run database migrations: %v", err)
	}

	return store, nil
}

// Ping checks the database connection
func (s *PostgresStore) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

// Close closes the database connection
func (s *PostgresStore) Close() error {
	return s.db.Close()
}
