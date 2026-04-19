package schema

import (
	"context"

	"github.com/polesen/datacow/internal/core/db"
)

// Table holds the complete schema for a single table or view.
type Table struct {
	Name        string
	Kind        db.TableKind
	Columns     []db.Column
	ForeignKeys []db.ForeignKey
}

// Load returns schema information for every table in the database.
func Load(ctx context.Context, client db.Client) ([]Table, error) {
	entries, err := client.ListTables(ctx)
	if err != nil {
		return nil, err
	}

	tables := make([]Table, 0, len(entries))
	for _, e := range entries {
		cols, err := client.Describe(ctx, e.Name)
		if err != nil {
			return nil, err
		}
		fks, err := client.ForeignKeys(ctx, e.Name)
		if err != nil {
			return nil, err
		}
		tables = append(tables, Table{
			Name:        e.Name,
			Kind:        e.Kind,
			Columns:     cols,
			ForeignKeys: fks,
		})
	}
	return tables, nil
}
