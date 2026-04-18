package dataset

import (
	"context"

	"github.com/beetio/datacow/internal/core/config"
	"github.com/beetio/datacow/internal/core/db"
)

// Resolver auto-discovers all tables in a database and merges config-defined datasets.
type Resolver struct {
	client               db.Client
	configDatasets       []config.DatasetConfig
	activeDatasourceName string
}

// NewResolver returns a Resolver backed by the given client.
// configDatasets are merged with auto-discovered tables; pass nil for zero-config mode.
// activeDatasourceName controls dataset scoping: scoped datasets are only included when
// their datasource name matches. Pass "" to include only unscoped datasets.
func NewResolver(client db.Client, configDatasets []config.DatasetConfig, activeDatasourceName string) *Resolver {
	return &Resolver{
		client:               client,
		configDatasets:       configDatasets,
		activeDatasourceName: activeDatasourceName,
	}
}

// Resolve returns auto-discovered table datasets followed by config-defined datasets.
func (r *Resolver) Resolve(ctx context.Context) ([]Dataset, error) {
	tables, err := r.client.ListTables(ctx)
	if err != nil {
		return nil, err
	}
	datasets := make([]Dataset, len(tables))
	for i, t := range tables {
		datasets[i] = Dataset{Name: t, Table: t}
	}
	for _, cd := range r.configDatasets {
		// Skip datasets scoped to a different datasource.
		if cd.Datasource != "" && cd.Datasource != r.activeDatasourceName {
			continue
		}
		datasets = append(datasets, Dataset{Name: cd.Name, Table: cd.Table, SQL: cd.SQL})
	}
	return datasets, nil
}
