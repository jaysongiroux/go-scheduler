package db

import (
	"database/sql"

	"github.com/google/uuid"
)

// ToNullString converts a byte slice to sql.NullString for nullable JSON columns
func ToNullString(b []byte) sql.NullString {
	if len(b) == 0 {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: string(b), Valid: true}
}

// ToNullInt64 converts a *int64 to sql.NullInt64 for nullable BIGINT columns
func ToNullInt64(i *int64) sql.NullInt64 {
	if i == nil {
		return sql.NullInt64{Valid: false}
	}
	return sql.NullInt64{Int64: *i, Valid: true}
}

// ToNullUUID converts a *uuid.UUID to a nullable value for UUID columns
// Returns nil if the UUID pointer is nil, otherwise returns the UUID
func ToNullUUID(u *uuid.UUID) interface{} {
	if u == nil {
		return nil
	}
	return *u
}

// ToNullableString converts any pointer to a string type to sql.NullString
// Used for nullable enum/string columns in the database
func ToNullableString[T ~string](s *T) sql.NullString {
	if s == nil {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: string(*s), Valid: true}
}

// ChunkData chunks a slice of interface{} into smaller slices of size batchSize
func ChunkData(data []interface{}, batchSize int) [][]interface{} {
	if batchSize <= 0 {
		return nil
	}

	chunks := make([][]interface{}, 0, (len(data)+batchSize-1)/batchSize)

	for i := 0; i < len(data); i += batchSize {
		end := i + batchSize
		if end > len(data) {
			end = len(data)
		}
		chunks = append(chunks, data[i:end])
	}

	return chunks
}
