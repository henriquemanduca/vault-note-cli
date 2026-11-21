package cmd

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
)

// cleanOneLine sanitizes text cells for TSV/table rows by replacing newlines
// and tabs with spaces so tabular alignment is never corrupted.
func cleanOneLine(s string) string {
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\t", " ")
	return strings.TrimSpace(s)
}

// writeTabular renders rows in TSV (default) or aligned table format.
// Per architectural invariants, zero rows produces zero output (no header).
func writeTabular(w io.Writer, format OutputFormat, headers []string, rows [][]string, noHeader bool) error {
	if len(rows) == 0 {
		return nil
	}

	if format == FormatTable {
		tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		if !noHeader {
			if _, err := fmt.Fprintln(tw, strings.Join(headers, "\t")); err != nil {
				return err
			}
		}
		for _, row := range rows {
			clean := make([]string, len(row))
			for i, cell := range row {
				clean[i] = cleanOneLine(cell)
			}
			if _, err := fmt.Fprintln(tw, strings.Join(clean, "\t")); err != nil {
				return err
			}
		}
		return tw.Flush()
	}

	if !noHeader {
		if _, err := fmt.Fprintln(w, strings.Join(headers, "\t")); err != nil {
			return err
		}
	}
	for _, row := range rows {
		clean := make([]string, len(row))
		for i, cell := range row {
			clean[i] = cleanOneLine(cell)
		}
		if _, err := fmt.Fprintln(w, strings.Join(clean, "\t")); err != nil {
			return err
		}
	}
	return nil
}
