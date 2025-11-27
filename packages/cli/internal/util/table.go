package util

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

// RenderTable renders a table with dynamic column width calculation
func RenderTable(columns []TableColumn, data []map[string]interface{}) {
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
				// Calculate display width accounting for ANSI codes
				displayWidth := getDisplayWidth(valueStr)
				if displayWidth > columns[i].Width {
					columns[i].Width = displayWidth
				}
			}
		}
		// Ensure minimum width for arrow column
		if columns[i].Header == " " && columns[i].Width < 2 {
			columns[i].Width = 2
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
			// Use custom padding function that handles ANSI codes correctly
			paddedValue := padStringToWidth(value, col.Width)
			rowParts = append(rowParts, paddedValue)
		}
		fmt.Println(strings.Join(rowParts, " "))
	}
}

// removeANSICodes removes ANSI escape codes from a string for width calculation
func removeANSICodes(s string) string {
	// Simple ANSI code removal - this could be more sophisticated
	// but should handle most common cases
	for {
		start := strings.Index(s, "\033[")
		if start == -1 {
			break
		}
		end := strings.Index(s[start:], "m")
		if end == -1 {
			break
		}
		s = s[:start] + s[start+end+1:]
	}
	return s
}

// getDisplayWidth calculates the display width of a string, accounting for ANSI codes and Unicode characters
func getDisplayWidth(s string) int {
	clean := removeANSICodes(s)
	// Count the number of runes (Unicode characters) instead of bytes
	return len([]rune(clean))
}

// padStringToWidth pads a string to a specific width, accounting for ANSI codes
func padStringToWidth(s string, width int) string {
	displayWidth := getDisplayWidth(s)
	if displayWidth >= width {
		return s
	}
	// Add spaces to reach the target width
	result := s + strings.Repeat(" ", width-displayWidth)
	return result
}
