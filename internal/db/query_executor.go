package db

import (
	"database/sql"
	"fmt"
	"strings"
)

// QueryResult represents the result of a SQL query execution
type QueryResult struct {
	Success  bool
	Data     string
	Error    string
	RowCount int
}

// ExecuteQuery executes a SQL query and returns formatted results
func ExecuteQuery(db *sql.DB, query string) QueryResult {
	// Execute the query
	rows, err := db.Query(query)
	if err != nil {
		return QueryResult{
			Success: false,
			Error:   fmt.Sprintf("Error executing query:\n%s", err.Error()),
		}
	}
	defer rows.Close()

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		return QueryResult{
			Success: false,
			Error:   fmt.Sprintf("Error getting columns:\n%s", err.Error()),
		}
	}

	// Build results table
	var result strings.Builder
	result.WriteString("Query executed successfully!\n\n")

	// First pass: collect all data to calculate column widths
	var allRows [][]string
	rowCount := 0
	for rows.Next() {
		// Create a slice of interface{} to hold the values
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range columns {
			valuePtrs[i] = &values[i]
		}

		// Scan the row
		if err := rows.Scan(valuePtrs...); err != nil {
			return QueryResult{
				Success: false,
				Error:   fmt.Sprintf("Error scanning row:\n%s", err.Error()),
			}
		}

		// Process row data
		rowData := make([]string, len(columns))
		for i, val := range values {
			cellValue := "NULL"
			if val != nil {
				// Handle different data types properly
				switch v := val.(type) {
				case []byte:
					// Convert byte array to string
					cellValue = string(v)
				case string:
					cellValue = v
				case int64:
					cellValue = fmt.Sprintf("%d", v)
				case int32:
					cellValue = fmt.Sprintf("%d", v)
				case int:
					cellValue = fmt.Sprintf("%d", v)
				case float64:
					cellValue = fmt.Sprintf("%.2f", v)
				case float32:
					cellValue = fmt.Sprintf("%.2f", v)
				case bool:
					cellValue = fmt.Sprintf("%t", v)
				default:
					// For other types, use string representation
					cellValue = fmt.Sprintf("%v", v)
				}
			}
			rowData[i] = cellValue
		}
		allRows = append(allRows, rowData)
		rowCount++

		// Limit results to prevent overwhelming output
		if rowCount >= 100 {
			break
		}
	}

	// Calculate column widths
	columnWidths := make([]int, len(columns))
	for i, col := range columns {
		columnWidths[i] = len(col) // Start with header width
	}

	// Find the maximum width for each column
	for _, row := range allRows {
		for i, cell := range row {
			if len(cell) > columnWidths[i] {
				columnWidths[i] = len(cell)
			}
		}
	}

	// Ensure minimum width of 8 and maximum width of 50
	for i := range columnWidths {
		if columnWidths[i] < 8 {
			columnWidths[i] = 8
		}
		if columnWidths[i] > 50 {
			columnWidths[i] = 50
		}
	}

	// Create header
	header := "│"
	for i, col := range columns {
		header += fmt.Sprintf(" %-*s │", columnWidths[i], col)
	}
	result.WriteString(header + "\n")

	// Create separator
	separator := "├"
	for i := 0; i < len(columns); i++ {
		separator += strings.Repeat("─", columnWidths[i]+2)
		if i < len(columns)-1 {
			separator += "┼"
		}
	}
	separator += "┤"
	result.WriteString(separator + "\n")

	// Process rows with dynamic widths
	for _, rowData := range allRows {
		rowStr := "│"
		for i, cell := range rowData {
			// Truncate if too long
			displayValue := cell
			if len(cell) > columnWidths[i] {
				displayValue = cell[:columnWidths[i]-3] + "..."
			}
			rowStr += fmt.Sprintf(" %-*s │", columnWidths[i], displayValue)
		}
		result.WriteString(rowStr + "\n")
	}

	if rowCount >= 100 {
		result.WriteString("\n... (showing first 100 rows only)\n")
	}

	if err := rows.Err(); err != nil {
		return QueryResult{
			Success: false,
			Error:   fmt.Sprintf("Error iterating rows:\n%s", err.Error()),
		}
	}

	result.WriteString(fmt.Sprintf("\nTotal rows: %d", rowCount))

	return QueryResult{
		Success:  true,
		Data:     result.String(),
		RowCount: rowCount,
	}
}
