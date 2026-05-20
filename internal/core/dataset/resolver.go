package dataset

import (
	"context"

	"github.com/polesen/datacow/internal/core/config"
	"github.com/polesen/datacow/internal/core/db"
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
// KindPerspective entries are inserted immediately after their parent dataset.
func (r *Resolver) Resolve(ctx context.Context) ([]Dataset, error) {
	entries, err := r.client.ListTables(ctx)
	if err != nil {
		return nil, err
	}
	datasets := make([]Dataset, len(entries))
	for i, e := range entries {
		datasets[i] = Dataset{Name: e.Name, Table: e.Name, Kind: kindFromTableKind(e.Kind)}
	}
	for _, cd := range r.configDatasets {
		// Skip datasets scoped to a different datasource.
		if cd.Datasource != "" && cd.Datasource != r.activeDatasourceName {
			continue
		}
		k := KindTable
		if cd.SQL != "" {
			k = KindDataset
		}
		parent := Dataset{Name: cd.Name, Table: cd.Table, SQL: cd.SQL, Kind: k}
		datasets = append(datasets, parent)
		// Append perspectives immediately after their parent.
		for _, p := range cd.Perspectives {
			datasets = append(datasets, perspectiveFromConfig(cd, p))
		}
	}
	return datasets, nil
}

// perspectiveFromConfig converts a PerspectiveConfig into a KindPerspective Dataset.
func perspectiveFromConfig(cd config.DatasetConfig, p config.PerspectiveConfig) Dataset {
	preset := &QueryOptionsPreset{
		Columns: p.Columns,
		Filters: filtersFromConfig(p.Filters),
	}
	if len(p.Sort) > 0 {
		preset.Sort = &Sort{Column: p.Sort[0].Column, Desc: p.Sort[0].Desc}
	}
	return Dataset{
		Name:        p.Name,
		Table:       cd.Table,
		Kind:        KindPerspective,
		ParentTable: cd.Table,
		Preset:      preset,
	}
}

func filtersFromConfig(fcs []config.FilterConfig) []Filter {
	if len(fcs) == 0 {
		return nil
	}
	out := make([]Filter, len(fcs))
	for i, fc := range fcs {
		out[i] = Filter{Column: fc.Column, Operator: fc.Operator, Value: fc.Value}
	}
	return out
}

func kindFromTableKind(k db.TableKind) Kind {
	if k == db.KindView {
		return KindView
	}
	return KindTable
}
