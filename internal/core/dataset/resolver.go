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

// Resolve returns auto-discovered table datasets with perspectives interleaved, followed
// by config-defined datasets that are not already represented by an auto-discovered table.
//
// Deduplication rule: a config dataset that is a plain table reference (no SQL, Name==Table)
// for a table that was already auto-discovered is not added as a second parent entry.
// Its perspectives are instead inserted immediately after the matching auto-discovered entry.
// This prevents the same table from appearing twice in the schema explorer after a perspective
// is saved.
func (r *Resolver) Resolve(ctx context.Context) ([]Dataset, error) {
	entries, err := r.client.ListTables(ctx)
	if err != nil {
		return nil, err
	}

	// Build auto-discovered list and an index from table name → position.
	auto := make([]Dataset, len(entries))
	autoIdx := make(map[string]int, len(entries))
	for i, e := range entries {
		auto[i] = Dataset{Name: e.Name, Table: e.Name, Kind: kindFromTableKind(e.Kind)}
		autoIdx[e.Name] = i
	}

	// Collect extra perspectives to interleave after their auto-discovered parent.
	extraPerspectives := make(map[int][]Dataset)

	// Config datasets that are NOT deduplicated go here (appended after auto entries).
	var configEntries []Dataset

	for _, cd := range r.configDatasets {
		if cd.Datasource != "" && cd.Datasource != r.activeDatasourceName {
			continue
		}
		k := KindTable
		if cd.SQL != "" {
			k = KindDataset
		}
		// Deduplicate: plain table reference whose table was already auto-discovered.
		// Merge its perspectives into the auto-discovered slot; skip the redundant parent.
		if cd.SQL == "" && cd.Table != "" && cd.Name == cd.Table {
			if idx, ok := autoIdx[cd.Table]; ok {
				for _, p := range cd.Perspectives {
					extraPerspectives[idx] = append(extraPerspectives[idx], perspectiveFromConfig(cd, p))
				}
				continue
			}
		}
		configEntries = append(configEntries, Dataset{Name: cd.Name, Table: cd.Table, SQL: cd.SQL, Kind: k})
		for _, p := range cd.Perspectives {
			configEntries = append(configEntries, perspectiveFromConfig(cd, p))
		}
	}

	// Assemble: each auto entry followed by its extra perspectives, then config entries.
	total := len(auto) + len(configEntries)
	for _, extras := range extraPerspectives {
		total += len(extras)
	}
	datasets := make([]Dataset, 0, total)
	for i, ds := range auto {
		datasets = append(datasets, ds)
		datasets = append(datasets, extraPerspectives[i]...)
	}
	datasets = append(datasets, configEntries...)
	return datasets, nil
}

// perspectiveFromConfig converts a PerspectiveConfig into a KindPerspective Dataset.
func perspectiveFromConfig(cd config.DatasetConfig, p config.PerspectiveConfig) Dataset {
	preset := &QueryOptionsPreset{
		Columns: p.Columns,
		Filters: filtersFromConfig(p.Filters),
	}
	for _, sc := range p.Sort {
		preset.Sort = append(preset.Sort, Sort{Column: sc.Column, Desc: sc.Desc})
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
