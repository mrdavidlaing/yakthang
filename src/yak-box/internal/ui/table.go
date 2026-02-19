// Package ui provides user interface utilities for yak-box.
package ui

import (
	"fmt"
	"io"
	"text/tabwriter"
)

// PrintTable prints a formatted table with headers and rows using text/tabwriter.
// Headers and rows are printed with tab separators for automatic column alignment.
func PrintTable(w io.Writer, headers []string, rows [][]string) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)

	// Print headers
	if len(headers) > 0 {
		for i, h := range headers {
			fmt.Fprint(tw, h)
			if i < len(headers)-1 {
				fmt.Fprint(tw, "\t")
			}
		}
		fmt.Fprintln(tw)
	}

	// Print rows
	for _, row := range rows {
		for i, cell := range row {
			fmt.Fprint(tw, cell)
			if i < len(row)-1 {
				fmt.Fprint(tw, "\t")
			}
		}
		fmt.Fprintln(tw)
	}

	return tw.Flush()
}
