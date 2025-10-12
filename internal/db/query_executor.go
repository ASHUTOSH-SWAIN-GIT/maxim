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

	// Create header
	header := "│"
	for _, col := range columns {
		header += fmt.Sprintf(" %-15s │", col)
	}
	result.WriteString(header + "\n")

	// Create separator
	separator := "├"
	for i := 0; i < len(columns); i++ {
		separator += strings.Repeat("─", 17)
		if i < len(columns)-1 {
			separator += "┼"
		}
	}
	separator += "┤"
	result.WriteString(separator + "\n")

	// Process rows
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

		// Build row string
		rowStr := "│"
		for _, val := range values {
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
				case float64:
					cellValue = fmt.Sprintf("%.2f", v)
				case bool:
					cellValue = fmt.Sprintf("%t", v)
				default:
					// For other types, use string representation
					cellValue = fmt.Sprintf("%v", v)
				}

				// Truncate long values
				if len(cellValue) > 15 {
					cellValue = cellValue[:12] + "..."
				}
			}
			rowStr += fmt.Sprintf(" %-15s │", cellValue)
		}
		result.WriteString(rowStr + "\n")
		rowCount++

		// Limit results to prevent overwhelming output
		if rowCount >= 100 {
			result.WriteString("\n... (showing first 100 rows only)\n")
			break
		}
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
