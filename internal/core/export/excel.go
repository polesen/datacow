package export

import (
	"context"
	"fmt"

	"github.com/polesen/datacow/internal/core/dataset"
	"github.com/xuri/excelize/v2"
)

func (e *Exporter) exportExcel(ctx context.Context, ds dataset.Dataset, opts dataset.QueryOptions, path string, progressFn func(int)) error {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()

	sheet := f.GetSheetName(0)
	opts.PageSize = exportPageSize
	rowNum := 1

	err := e.forEachPage(ctx, ds, opts, func(columns []string, rows []map[string]any, first bool) error {
		if first {
			for i, col := range columns {
				cell, _ := excelize.CoordinatesToCellName(i+1, rowNum)
				if err := f.SetCellValue(sheet, cell, col); err != nil {
					return fmt.Errorf("write header: %w", err)
				}
			}
			rowNum++
		}
		for _, row := range rows {
			for i, col := range columns {
				cell, _ := excelize.CoordinatesToCellName(i+1, rowNum)
				v := row[col]
				var val any
				if b, ok := v.([]byte); ok {
					val = string(b)
				} else {
					val = v
				}
				if err := f.SetCellValue(sheet, cell, val); err != nil {
					return fmt.Errorf("write cell: %w", err)
				}
			}
			rowNum++
		}
		return nil
	}, progressFn)
	if err != nil {
		return err
	}

	return f.SaveAs(path)
}
