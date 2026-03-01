package db

import "embed"

// MigrationsFS holds the SQL migration files embedded in the binary.
// This allows the app to run migrations without needing the migrations
// directory on the filesystem (e.g. in Docker).
//go:embed migrations/*.sql
var MigrationsFS embed.FS
