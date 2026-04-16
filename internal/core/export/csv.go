package export

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"

	"github.com/beetio/datacow/internal/core/dataset"
)

func (e *Exporter) exportCSV(ctx context.Context, ds dataset.Dataset, opts dataset.QueryOptions, path string, progressFn func(int)) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	w := csv.NewWriter(f)
	opts.PageSize = exportPageSize

	err = e.forEachPage(ctx, ds, opts, func(columns []string, rows []map[string]any, first bool) error {
		if first {
			if err := w.Write(columns); err != nil {
				return err
			}
		}
		for _, row := range rows {
			record := make([]string, len(columns))
			for i, col := range columns {
				v := row[col]
				if v == nil {
					record[i] = ""
				} else if b, ok := v.([]byte); ok {
					record[i] = string(b)
				} else {
					record[i] = fmt.Sprintf("%v", v)
				}
			}
			if err := w.Write(record); err != nil {
				return err
			}
		}
		return nil
	}, progressFn)
	if err != nil {
		return err
	}

	w.Flush()
	return w.Error()
}
