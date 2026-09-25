// Package migrations embeds the SQL migration files so the migrate tool and the test helper can
// run them without needing a filesystem path at runtime.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
