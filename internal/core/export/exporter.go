package export

import (
	"context"
	"fmt"

	"github.com/polesen/datacow/internal/core/dataset"
)

const exportPageSize = 500

type Format string

const (
	FormatCSV   Format = "csv"
	FormatExcel Format = "xlsx"
)

type Exporter struct {
	executor *dataset.Executor
}

func NewExporter(executor *dataset.Executor) *Exporter {
	return &Exporter{executor: executor}
}

// Export writes all rows matching opts to path. progressFn receives cumulative
// row count after each page; pass nil to skip progress reporting.
func (e *Exporter) Export(ctx context.Context, ds dataset.Dataset, opts dataset.QueryOptions, format Format, path string, progressFn func(int)) error {
	switch format {
	case FormatCSV:
		return e.exportCSV(ctx, ds, opts, path, progressFn)
	case FormatExcel:
		return e.exportExcel(ctx, ds, opts, path, progressFn)
	default:
		return fmt.Errorf("unknown format %q", format)
	}
}

func (e *Exporter) forEachPage(
	ctx context.Context,
	ds dataset.Dataset,
	opts dataset.QueryOptions,
	fn func(columns []string, rows []map[string]any, first bool) error,
	progressFn func(int),
) error {
	var columns []string
	total := 0

	for page := 1; ; page++ {
		opts.Page = page
		result, err := e.executor.Query(ctx, ds, opts)
		if err != nil {
			return fmt.Errorf("query page %d: %w", page, err)
		}

		first := page == 1
		if first {
			columns = make([]string, len(result.Columns))
			for i, c := range result.Columns {
				columns[i] = c.Name
			}
		}

		if err := fn(columns, result.Rows, first); err != nil {
			return err
		}

		total += len(result.Rows)
		if progressFn != nil {
			progressFn(total)
		}

		if page >= result.TotalPages {
			break
		}
	}
	return nil
}
