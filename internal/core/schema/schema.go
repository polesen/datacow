package schema

import (
	"context"

	"github.com/polesen/datacow/internal/core/db"
)

// InboundFK describes an FK that points at this table from elsewhere.
type InboundFK struct {
	FromTable  string // referencing table
	FromColumn string // column in the referencing table that holds the FK
	ToColumn   string // the column in *this* table that is being referenced
}

// Table holds the complete schema for a single table or view.
type Table struct {
	Name         string
	Kind         db.TableKind
	Columns      []db.Column
	ForeignKeys  []db.ForeignKey
	ReferencedBy []InboundFK
	Indexes      []db.Index
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
		var indexes []db.Index
		if e.Kind != db.KindView {
			idxs, err := client.Indexes(ctx, e.Name)
			if err != nil {
				return nil, err
			}
			indexes = idxs
		}
		tables = append(tables, Table{
			Name:        e.Name,
			Kind:        e.Kind,
			Columns:     cols,
			ForeignKeys: fks,
			Indexes:     indexes,
		})
	}

	// Second pass: build ReferencedBy from every table's outgoing FKs.
	refByMap := make(map[string][]InboundFK, len(tables))
	for _, t := range tables {
		for _, fk := range t.ForeignKeys {
			refByMap[fk.ReferencedTable] = append(refByMap[fk.ReferencedTable], InboundFK{
				FromTable:  t.Name,
				FromColumn: fk.Column,
				ToColumn:   fk.ReferencedColumn,
			})
		}
	}
	for i := range tables {
		tables[i].ReferencedBy = refByMap[tables[i].Name]
	}

	return tables, nil
}
