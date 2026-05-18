package db

import "embed"

// migrationFS embeds the SQL migration files into the binary.
// golang-migrate's iofs source reads from this via fs.Sub.
//
//go:embed migrations/*.sql
var migrationFS embed.FS
