package account

import (
	"context"

	"github.com/jaysongiroux/go-scheduler/internal/db"
)

type Queries struct {
	pool *db.ConnectionPool
}

func New(pool *db.ConnectionPool) *Queries {
	return &Queries{pool: pool}
}

func (q *Queries) DeleteAccount(ctx context.Context, accountID string) error {
	// Delete must be done in multiple separate Exec calls,
	// because you can't run multiple DELETEs in a single query string.
	queries := []string{
		"DELETE FROM calendar_event_reminders WHERE account_id = $1",
		"DELETE FROM calendar_events WHERE account_id = $1",
		"DELETE FROM calendar_members WHERE account_id = $1",
		"DELETE FROM calendars WHERE account_id = $1",
	}
	for _, query := range queries {
		if _, err := q.pool.DB().ExecContext(ctx, query, accountID); err != nil {
			return err
		}
	}
	return nil
}
