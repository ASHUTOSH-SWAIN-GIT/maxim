package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/lib/pq"
)

// QueryResult represents the result of a SQL query execution
type QueryResult struct {
	Success  bool
	Data     string
	Error    string
	RowCount int
	Columns  []string
	Rows     []DataRow
}

// ExecuteQuery executes a SQL query and returns formatted results
func ExecuteQuery(db *sql.DB, query string) QueryResult {
	return ExecuteQueryContext(context.Background(), db, query)
}

func ExecuteQueryContext(parent context.Context, db *sql.DB, query string) QueryResult {
	ctx, cancel := context.WithTimeout(parent, QueryTimeout)
	defer cancel()

	// Validate input
	trimmedQuery := strings.TrimSpace(query)
	if trimmedQuery == "" {
		return QueryResult{
			Success: false,
			Error:   " No query to execute\n\nPlease enter a SQL query and try again.",
		}
	}

	// Execute the query
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return QueryResult{
			Success: false,
			Error:   formatQueryError(err),
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
	columnTypes, err := rows.ColumnTypes()
	if err != nil {
		return QueryResult{Success: false, Error: fmt.Sprintf("Error getting column types:\n%s", err.Error())}
	}

	// Build results table
	var result strings.Builder
	result.WriteString("Query executed successfully!\n\n")

	// First pass: collect all data to calculate column widths
	var allRows []DataRow
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
		rowData := make(DataRow, len(columns))
		for i, val := range values {
			databaseTypeName := ""
			if i < len(columnTypes) {
				databaseTypeName = columnTypes[i].DatabaseTypeName()
			}
			rowData[i] = newCellValue(val, databaseTypeName)
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
			cellValue := cell.DisplayText(false)
			if ansi.StringWidth(cellValue) > columnWidths[i] {
				columnWidths[i] = ansi.StringWidth(cellValue)
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
			displayValue := cell.DisplayText(false)
			if ansi.StringWidth(displayValue) > columnWidths[i] {
				displayValue = ansi.Truncate(displayValue, columnWidths[i], "...")
			}
			padding := strings.Repeat(" ", max(columnWidths[i]-ansi.StringWidth(displayValue), 0))
			rowStr += " " + displayValue + padding + " │"
		}
		result.WriteString(rowStr + "\n")
	}

	if rowCount >= 100 {
		result.WriteString("\n... (showing first 100 rows only)\n")
	}

	if err := rows.Err(); err != nil {
		return QueryResult{
			Success: false,
			Error:   formatQueryError(err),
		}
	}

	result.WriteString(fmt.Sprintf("\nTotal rows: %d", rowCount))

	return QueryResult{
		Success:  true,
		Data:     result.String(),
		RowCount: rowCount,
		Columns:  columns,
		Rows:     allRows,
	}
}

// formatQueryError formats database errors with better readability
func formatQueryError(err error) string {
	if errors.Is(err, context.Canceled) {
		return "Query cancelled."
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Sprintf("Query timed out after %s.", QueryTimeout)
	}
	// Handle PostgreSQL specific errors
	if pqErr, ok := err.(*pq.Error); ok {
		return pqErr.Message
	}

	// Generic error formatting
	return err.Error()
}
