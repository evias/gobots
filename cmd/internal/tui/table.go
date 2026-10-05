package tui

import (
	"fmt"
)

type TableRow struct {
	Values []string
}

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
				suffix = "\r\n"
			}

			rowStr += fmt.Sprintf("%s%s%s", prefix, value, suffix)
		}

		fmt.Print(rowStr)
	}

	return nil
}

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
				suffix = "\r\n"
			}

			rowStr += fmt.Sprintf("%s%s%s", prefix, value, suffix)
		}
		fmt.Print(rowStr)
	}

	return nil
}
