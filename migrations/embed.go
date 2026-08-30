package migrations

import "embed"

// FS holds every .sql migration, embedded at build time.
//
//go:embed *.sql
var FS embed.FS
