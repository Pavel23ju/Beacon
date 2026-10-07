// Package migrations embeds the SQL migrations so the binary and the
// integration tests apply exactly the same schema.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
