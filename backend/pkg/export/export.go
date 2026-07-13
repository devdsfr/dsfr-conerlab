// Package export gera saídas em CSV e XLSX a partir de tabelas genéricas
// (cabeçalho + linhas de texto), usado pelos endpoints de exportação exigidos pela
// regra geral do MVP: "Todo dado utilizado em dashboards e análises deve estar
// disponível para exportação em CSV e Excel."
package export

import (
	"bytes"
	"encoding/csv"

	"github.com/xuri/excelize/v2"
)

// Table é uma representação genérica e já formatada (texto) de qualquer relatório do
// CornerLab, pronta para ser exportada.
type Table struct {
	SheetName string
	Headers   []string
	Rows      [][]string
}

func ToCSV(t Table) ([]byte, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	if err := w.Write(t.Headers); err != nil {
		return nil, err
	}
	for _, row := range t.Rows {
		if err := w.Write(row); err != nil {
			return nil, err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func ToXLSX(t Table) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()

	sheet := t.SheetName
	if sheet == "" {
		sheet = "Sheet1"
	}
	f.SetSheetName("Sheet1", sheet)

	for col, header := range t.Headers {
		cell, err := excelize.CoordinatesToCellName(col+1, 1)
		if err != nil {
			return nil, err
		}
		if err := f.SetCellValue(sheet, cell, header); err != nil {
			return nil, err
		}
	}
	for rowIdx, row := range t.Rows {
		for col, value := range row {
			cell, err := excelize.CoordinatesToCellName(col+1, rowIdx+2)
			if err != nil {
				return nil, err
			}
			if err := f.SetCellValue(sheet, cell, value); err != nil {
				return nil, err
			}
		}
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
