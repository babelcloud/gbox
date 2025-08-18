package cmd

import (
	"fmt"
	"strings"
)

// TableColumn represents a column in a table
type TableColumn struct {
	Header string
	Key    string // key to extract from data map
	Width  int    // calculated width
}

// renderTable renders a table with dynamic column width calculation
func renderTable(columns []TableColumn, data []map[string]interface{}) {
	if len(data) == 0 {
		fmt.Println("No data to display")
		return
	}

	// Calculate column widths based on header and data
	for i := range columns {
		columns[i].Width = len(columns[i].Header)
		for _, row := range data {
			if value, exists := row[columns[i].Key]; exists {
				valueStr := fmt.Sprintf("%v", value)
				if len(valueStr) > columns[i].Width {
					columns[i].Width = len(valueStr)
				}
			}
		}
	}

	// Print header
	var headerParts []string
	for _, col := range columns {
		headerParts = append(headerParts, fmt.Sprintf("%-*s", col.Width, col.Header))
	}
	header := strings.Join(headerParts, " ")
	fmt.Println(header)

	// Print separator
	var separatorParts []string
	for _, col := range columns {
		separatorParts = append(separatorParts, strings.Repeat("-", col.Width))
	}
	separator := strings.Join(separatorParts, " ")
	fmt.Println(separator)

	// Print data rows
	for _, row := range data {
		var rowParts []string
		for _, col := range columns {
			value := ""
			if v, exists := row[col.Key]; exists {
				value = fmt.Sprintf("%v", v)
			}
			rowParts = append(rowParts, fmt.Sprintf("%-*s", col.Width, value))
		}
		fmt.Println(strings.Join(rowParts, " "))
	}
}
