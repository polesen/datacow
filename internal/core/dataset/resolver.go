package dataset

import (
	"context"

	"github.com/beetio/datacow/internal/core/db"
)

// Resolver auto-discovers all tables in a database and exposes them as datasets.
type Resolver struct {
	client db.Client
}

// NewResolver returns a Resolver backed by the given client.
func NewResolver(client db.Client) *Resolver {
	return &Resolver{client: client}
}

// Resolve returns one Dataset per table/view in the database.
func (r *Resolver) Resolve(ctx context.Context) ([]Dataset, error) {
	tables, err := r.client.ListTables(ctx)
	if err != nil {
		return nil, err
	}
	datasets := make([]Dataset, len(tables))
	for i, t := range tables {
		datasets[i] = Dataset{Name: t, Table: t}
	}
	return datasets, nil
}
