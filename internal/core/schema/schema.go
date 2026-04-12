package schema

import (
	"context"

	"github.com/beetio/datacow/internal/core/db"
)

// Table holds the complete schema for a single table.
type Table struct {
	Name        string
	Columns     []db.Column
	ForeignKeys []db.ForeignKey
}

// Load returns schema information for every table in the database.
func Load(ctx context.Context, client db.Client) ([]Table, error) {
	names, err := client.ListTables(ctx)
	if err != nil {
		return nil, err
	}

	tables := make([]Table, 0, len(names))
	for _, name := range names {
		cols, err := client.Describe(ctx, name)
		if err != nil {
			return nil, err
		}
		fks, err := client.ForeignKeys(ctx, name)
		if err != nil {
			return nil, err
		}
		tables = append(tables, Table{
			Name:        name,
			Columns:     cols,
			ForeignKeys: fks,
		})
	}
	return tables, nil
}
