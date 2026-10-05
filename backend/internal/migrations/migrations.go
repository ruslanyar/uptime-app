// Package migrations runs versioned SQL separately from the API process.
package migrations

import (
	"context"
	"fmt"
	"io/fs"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/tern/v2/migrate"
)

type Status struct{ Current, Target int32 }

func Run(ctx context.Context, conn *pgx.Conn, files fs.FS, operation string) (Status, error) {
	if operation != "up" && operation != "down" && operation != "status" {
		return Status{}, fmt.Errorf("expected up, status or down")
	}
	m, e := migrate.NewMigrator(ctx, conn, "public.schema_version")
	if e != nil {
		return Status{}, e
	}
	if e = m.LoadMigrations(files); e != nil {
		return Status{}, e
	}
	current, e := m.GetCurrentVersion(ctx)
	if e != nil {
		return Status{}, e
	}
	switch operation {
	case "up":
		e = m.Migrate(ctx)
	case "down":
		if current > 0 {
			e = m.MigrateTo(ctx, current-1)
		}
	}
	if e != nil {
		return Status{}, e
	}
	current, e = m.GetCurrentVersion(ctx)
	return Status{current, int32(len(m.Migrations))}, e
}
