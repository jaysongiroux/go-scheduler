package account

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jaysongiroux/go-scheduler/internal/db"
	"github.com/jaysongiroux/go-scheduler/internal/logger"
)

// Account represents an account
type Account struct {
	AccountID string          `json:"account_id"`
	CreatedTs int64           `json:"created_ts"`
	UpdatedTs int64           `json:"updated_ts"`
	Settings  json.RawMessage `json:"settings"`
	Metadata  json.RawMessage `json:"metadata"`
}

// Queries holds the database connection pool
type Queries struct {
	pool *db.ConnectionPool
}

// New creates a new Queries instance
func New(pool *db.ConnectionPool) *Queries {
	return &Queries{pool: pool}
}

// check if account exists
func (q *Queries) CheckAccountExists(ctx context.Context, accountID string) (bool, error) {
	query := `
		SELECT EXISTS(SELECT 1 FROM accounts WHERE account_id = $1)
	`

	var exists bool
	err := q.pool.DB().QueryRowContext(ctx, query, accountID).Scan(&exists)
	return exists, err
}

func (q *Queries) CreateAccount(ctx context.Context, account *Account) error {
	query := `
		INSERT INTO accounts (account_id, created_ts, updated_ts, settings, metadata)
		VALUES ($1, $2, $3, $4, $5)
	`

	_, err := q.pool.DB().
		ExecContext(ctx, query, account.AccountID, account.CreatedTs, account.UpdatedTs, account.Settings, account.Metadata)
	return err
}

func (q *Queries) GetAccountByID(ctx context.Context, accountID string) (*Account, error) {
	query := `
		SELECT account_id, created_ts, updated_ts, settings, metadata
		FROM accounts
		WHERE account_id = $1
		LIMIT 1
	`

	var account Account
	err := q.pool.DB().
		QueryRowContext(ctx, query, accountID).
		Scan(&account.AccountID, &account.CreatedTs, &account.UpdatedTs, &account.Settings, &account.Metadata)
	return &account, err
}

func (q *Queries) DeleteAccountByID(ctx context.Context, accountID string) error {
	query := `
		DELETE FROM accounts WHERE account_id = $1
	`

	_, err := q.pool.DB().ExecContext(ctx, query, accountID)
	return err
}

func (q *Queries) UpdateAccountByID(ctx context.Context, accountID string, account *Account) error {
	query := `
		UPDATE accounts SET settings = $2, metadata = $3, updated_ts = $4 WHERE account_id = $1
	`

	updatedTs := time.Now().UTC().Unix()
	account.UpdatedTs = updatedTs

	var settings any = account.Settings
	var metadata any = account.Metadata

	// If Settings is empty (nil or empty []byte), set to nil (NULL in DB)
	if b, ok := settings.([]byte); ok {
		if len(b) == 0 || string(b) == "null" {
			settings = db.ToNullString([]byte("null"))
		}
	}

	// If Metadata is empty (nil or empty []byte), set to nil (NULL in DB)
	if b, ok := metadata.([]byte); ok {
		if len(b) == 0 || string(b) == "null" {
			// pg null string
			metadata = db.ToNullString([]byte("null"))
		}
	}

	_, err := q.pool.DB().
		ExecContext(ctx, query, accountID, settings, metadata, updatedTs)
	return err
}

type AccountFilters struct {
	MetadataFilters map[string]interface{}
	SettingsFilters map[string]interface{}
}

func (q *Queries) GetAccounts(
	ctx context.Context,
	limit int,
	offset int,
	filters *AccountFilters,
) ([]*Account, error) {
	query := `
		SELECT account_id, created_ts, updated_ts, settings, metadata
		FROM accounts
	`

	args := make([]interface{}, 0)
	whereClauses := make([]string, 0)
	argPosition := 1

	if filters != nil {
		for key, value := range filters.MetadataFilters {
			filterJSON, err := json.Marshal(map[string]interface{}{key: value})
			if err != nil {
				return nil, fmt.Errorf("failed to marshal metadata filter: %w", err)
			}
			whereClauses = append(whereClauses, fmt.Sprintf("metadata @> $%d::jsonb", argPosition))
			args = append(args, string(filterJSON))
			argPosition++
		}

		for key, value := range filters.SettingsFilters {
			filterJSON, err := json.Marshal(map[string]interface{}{key: value})
			if err != nil {
				return nil, fmt.Errorf("failed to marshal settings filter: %w", err)
			}
			whereClauses = append(
				whereClauses,
				fmt.Sprintf("settings::jsonb @> $%d::jsonb", argPosition),
			)
			args = append(args, string(filterJSON))
			argPosition++
		}
	}

	if len(whereClauses) > 0 {
		query += " WHERE " + strings.Join(whereClauses, " AND ")
	}

	query += fmt.Sprintf(`
		ORDER BY created_ts DESC
		LIMIT $%d OFFSET $%d
	`, argPosition, argPosition+1)

	args = append(args, limit, offset)

	rows, err := q.pool.DB().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			logger.Error("Failed to close rows: %v", err)
		}
	}()

	accounts := make([]*Account, 0)
	for rows.Next() {
		var account Account
		err := rows.Scan(
			&account.AccountID,
			&account.CreatedTs,
			&account.UpdatedTs,
			&account.Settings,
			&account.Metadata,
		)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, &account)
	}
	return accounts, nil
}
