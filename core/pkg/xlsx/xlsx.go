// Package xlsx wraps xuri/excelize with a tiny append-rows builder, enough to
// stream a single-sheet spreadsheet export with a styled header.
package xlsx

import (
	"fmt"
	"io"

	"github.com/xuri/excelize/v2"
)

// Workbook is a single-sheet spreadsheet being built row by row.
type Workbook struct {
	f     *excelize.File
	sheet string
	row   int // last written row (1-based); 0 before any row
	cols  int // column count, set by Header
}

type Option func(*Workbook)

// WithSheet names the (single) sheet.
func WithSheet(name string) Option {
	return func(w *Workbook) { w.sheet = name }
}

const _defaultSheet = "Sheet1"

// New builds an empty workbook with one sheet.
func New(opts ...Option) *Workbook {
	w := &Workbook{f: excelize.NewFile(), sheet: _defaultSheet}
	for _, opt := range opts {
		opt(w)
	}
	if w.sheet != _defaultSheet {
		_ = w.f.SetSheetName(_defaultSheet, w.sheet)
	}

	return w
}

// Header writes the first row and styles it (bold on a light fill with a bottom
// border), then freezes it so it stays visible while scrolling.
func (w *Workbook) Header(cols ...string) error {
	if err := w.AppendRow(toAny(cols)...); err != nil {
		return err
	}
	w.cols = len(cols)

	style, err := w.f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "1E293B"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"F1F5F9"}, Pattern: 1},
		Border:    []excelize.Border{{Type: "bottom", Color: "CBD5E1", Style: 1}},
		Alignment: &excelize.Alignment{Vertical: "center"},
	})
	if err != nil {
		return fmt.Errorf("xlsx: header style: %w", err)
	}

	last, err := excelize.CoordinatesToCellName(len(cols), 1)
	if err != nil {
		return fmt.Errorf("xlsx: header range: %w", err)
	}
	if err = w.f.SetCellStyle(w.sheet, "A1", last, style); err != nil {
		return fmt.Errorf("xlsx: apply header style: %w", err)
	}

	if err = w.f.SetPanes(w.sheet, &excelize.Panes{
		Freeze:      true,
		YSplit:      1,
		TopLeftCell: "A2",
		ActivePane:  "bottomLeft",
	}); err != nil {
		return fmt.Errorf("xlsx: freeze header: %w", err)
	}

	return nil
}

// AppendRow writes one row of cells (left to right) and advances the cursor.
func (w *Workbook) AppendRow(values ...any) error {
	w.row++
	for i, v := range values {
		cell, err := excelize.CoordinatesToCellName(i+1, w.row)
		if err != nil {
			return fmt.Errorf("xlsx: cell name: %w", err)
		}
		if err := w.f.SetCellValue(w.sheet, cell, v); err != nil {
			return fmt.Errorf("xlsx: set cell %s: %w", cell, err)
		}
	}

	return nil
}

// SetColWidth sets the width (in character units) of a 1-based column.
func (w *Workbook) SetColWidth(col int, width float64) error {
	name, err := excelize.ColumnNumberToName(col)
	if err != nil {
		return fmt.Errorf("xlsx: column name: %w", err)
	}
	if err = w.f.SetColWidth(w.sheet, name, name, width); err != nil {
		return fmt.Errorf("xlsx: set width: %w", err)
	}

	return nil
}

// SetColNumberFormat applies a custom number format to a 1-based column's data
// cells (every row written after the header). Call after all rows are appended.
func (w *Workbook) SetColNumberFormat(col int, format string) error {
	if w.row <= 1 {
		return nil // header only, no data rows
	}

	style, err := w.f.NewStyle(&excelize.Style{CustomNumFmt: &format})
	if err != nil {
		return fmt.Errorf("xlsx: number format: %w", err)
	}

	top, err := excelize.CoordinatesToCellName(col, 2)
	if err != nil {
		return fmt.Errorf("xlsx: format range: %w", err)
	}
	bottom, err := excelize.CoordinatesToCellName(col, w.row)
	if err != nil {
		return fmt.Errorf("xlsx: format range: %w", err)
	}
	if err = w.f.SetCellStyle(w.sheet, top, bottom, style); err != nil {
		return fmt.Errorf("xlsx: apply number format: %w", err)
	}

	return nil
}

// AutoFilter turns the header into a filterable/sortable range over all rows.
func (w *Workbook) AutoFilter() error {
	if w.cols == 0 || w.row == 0 {
		return nil
	}

	last, err := excelize.CoordinatesToCellName(w.cols, w.row)
	if err != nil {
		return fmt.Errorf("xlsx: autofilter range: %w", err)
	}
	if err = w.f.AutoFilter(w.sheet, "A1:"+last, nil); err != nil {
		return fmt.Errorf("xlsx: autofilter: %w", err)
	}

	return nil
}

// WriteTo streams the workbook bytes to dst.
func (w *Workbook) WriteTo(dst io.Writer) (int64, error) {
	n, err := w.f.WriteTo(dst)
	if err != nil {
		return n, fmt.Errorf("xlsx: write: %w", err)
	}

	return n, nil
}

// Close releases the workbook's resources.
func (w *Workbook) Close() error {
	return w.f.Close()
}

func toAny(s []string) []any {
	out := make([]any, len(s))
	for i, v := range s {
		out[i] = v
	}

	return out
}
