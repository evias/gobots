package tui

import (
	"fmt"
)

// TableRow describes a wrapper around a string-slice of values.
type TableRow struct {
	Values []string
}

// PrintTable uses [fmt.Print] to print a tabulated number of rows. Prints a
// tab character ("\t") after each value from [TableRow#Values].
// Note that this method requires the console to be set in raw mode.
func PrintTable(rows []TableRow) error {
	for _, row := range rows {
		rowStr := ""
		for i, value := range row.Values {
			prefix := "\t"
			suffix := ""
			if i == 0 {
				prefix = ""
			}
			if i == len(row.Values)-1 {
				suffix = "\r\n" // terminal raw mode!
			}

			rowStr += fmt.Sprintf("%s%s%s", prefix, value, suffix)
		}

		fmt.Print(rowStr)
	}

	return nil
}

// PrintTableWithHeader uses [fmt.Print] to print a tabulated number of rows,
// preceded by a row of headers. Prints a tab character ("\t") after each value
// from [TableRow#Values].
// Note that this method requires the console to be set in raw mode.
func PrintTableWithHeader(headers []string, rows []TableRow) error {
	headerStr := ""
	for i, header := range headers {
		prefix := "\t"
		suffix := ""
		if i == 0 {
			prefix = ""
		}

		headerStr += fmt.Sprintf("%s%s%s", prefix, header, suffix)
	}
	fmt.Print(headerStr)

	for _, row := range rows {
		rowStr := ""
		for i, value := range row.Values {
			prefix := "\t"
			suffix := ""
			if i == 0 {
				prefix = ""
			}
			if i == len(row.Values)-1 {
				suffix = "\r\n" // terminal raw mode!
			}

			rowStr += fmt.Sprintf("%s%s%s", prefix, value, suffix)
		}
		fmt.Print(rowStr)
	}

	return nil
}
