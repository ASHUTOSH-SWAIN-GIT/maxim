package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/lib/pq"
)

type TableBrowseRequest struct {
	Limit        int
	Offset       int
	SortColumn   string
	Descending   bool
	FilterColumn string
	FilterValue  string
	Cursor       string
}

type TableBrowsePage struct {
	Columns       []table.Column
	Rows          []table.Row
	HasNext       bool
	NextCursor    string
	KeysetEnabled bool
	SortColumn    string
}

// BrowseTable fetches one bounded, deterministically ordered page. It uses
// keyset pagination when sorting by a single-column primary key and safely
// falls back to OFFSET for other orderings.
func BrowseTable(database *sql.DB, tableName string, request TableBrowseRequest) (TableBrowsePage, error) {
	return BrowseTableContext(context.Background(), database, tableName, request)
}

func BrowseTableContext(parent context.Context, database *sql.DB, tableName string, request TableBrowseRequest) (TableBrowsePage, error) {
	ctx, cancel := context.WithTimeout(parent, BrowseTimeout)
	defer cancel()

	structure, err := GetTableStructureContext(ctx, database, tableName)
	if err != nil {
		return TableBrowsePage{}, err
	}
	if len(structure) == 0 {
		return TableBrowsePage{}, fmt.Errorf("table %q has no columns", tableName)
	}

	columnsByName := make(map[string]TableColumnInfo, len(structure))
	primaryKeys := make([]string, 0, 1)
	for _, column := range structure {
		columnsByName[column.Name] = column
		if column.PrimaryKey {
			primaryKeys = append(primaryKeys, column.Name)
		}
	}
	if request.FilterColumn != "" {
		if _, ok := columnsByName[request.FilterColumn]; !ok {
			return TableBrowsePage{}, fmt.Errorf("unknown filter column %q", request.FilterColumn)
		}
	}
	if request.SortColumn == "" {
		if len(primaryKeys) == 1 {
			request.SortColumn = primaryKeys[0]
		} else {
			request.SortColumn = structure[0].Name
		}
	}
	if _, ok := columnsByName[request.SortColumn]; !ok {
		return TableBrowsePage{}, fmt.Errorf("unknown sort column %q", request.SortColumn)
	}
	if request.Limit <= 0 || request.Limit > 500 {
		request.Limit = 100
	}
	if request.Offset < 0 {
		request.Offset = 0
	}

	keyset := len(primaryKeys) == 1 && request.SortColumn == primaryKeys[0]
	arguments := make([]any, 0, 2)
	conditions := make([]string, 0, 2)
	if request.FilterColumn != "" && request.FilterValue != "" {
		arguments = append(arguments, request.FilterValue)
		conditions = append(conditions, fmt.Sprintf("%s = $%d", pq.QuoteIdentifier(request.FilterColumn), len(arguments)))
	}
	if keyset && request.Cursor != "" {
		operator := ">"
		if request.Descending {
			operator = "<"
		}
		arguments = append(arguments, request.Cursor)
		conditions = append(conditions, fmt.Sprintf("%s %s $%d", pq.QuoteIdentifier(request.SortColumn), operator, len(arguments)))
	}

	var query strings.Builder
	query.WriteString("SELECT * FROM ")
	query.WriteString(pq.QuoteIdentifier(tableName))
	if len(conditions) > 0 {
		query.WriteString(" WHERE ")
		query.WriteString(strings.Join(conditions, " AND "))
	}
	query.WriteString(" ORDER BY ")
	query.WriteString(pq.QuoteIdentifier(request.SortColumn))
	if request.Descending {
		query.WriteString(" DESC")
	} else {
		query.WriteString(" ASC")
	}
	if len(primaryKeys) == 1 && request.SortColumn != primaryKeys[0] {
		query.WriteString(", ")
		query.WriteString(pq.QuoteIdentifier(primaryKeys[0]))
		if request.Descending {
			query.WriteString(" DESC")
		} else {
			query.WriteString(" ASC")
		}
	}
	query.WriteString(fmt.Sprintf(" LIMIT %d", request.Limit+1))
	if !keyset {
		query.WriteString(fmt.Sprintf(" OFFSET %d", request.Offset))
	}

	rows, err := database.QueryContext(ctx, query.String(), arguments...)
	if err != nil {
		return TableBrowsePage{}, err
	}
	defer rows.Close()

	columnNames, err := rows.Columns()
	if err != nil {
		return TableBrowsePage{}, err
	}
	result := TableBrowsePage{
		Columns:       make([]table.Column, len(columnNames)),
		KeysetEnabled: keyset,
		SortColumn:    request.SortColumn,
	}
	sortIndex := 0
	for index, name := range columnNames {
		result.Columns[index] = table.Column{Title: name, Width: 20}
		if name == request.SortColumn {
			sortIndex = index
		}
	}
	for rows.Next() {
		values := make([]any, len(columnNames))
		scanArguments := make([]any, len(columnNames))
		for index := range values {
			scanArguments[index] = &values[index]
		}
		if err := rows.Scan(scanArguments...); err != nil {
			return TableBrowsePage{}, err
		}
		if len(result.Rows) == request.Limit {
			result.HasNext = true
			break
		}
		row := make(table.Row, len(values))
		for index, value := range values {
			row[index] = formatTableValue(value)
		}
		result.Rows = append(result.Rows, row)
	}
	if err := rows.Err(); err != nil {
		return TableBrowsePage{}, err
	}
	if keyset && len(result.Rows) > 0 {
		result.NextCursor = result.Rows[len(result.Rows)-1][sortIndex]
	}
	return result, nil
}

func formatTableValue(value any) string {
	if value == nil {
		return "NULL"
	}
	switch typed := value.(type) {
	case []byte:
		return string(typed)
	case float64:
		return fmt.Sprintf("%.2f", typed)
	case float32:
		return fmt.Sprintf("%.2f", typed)
	default:
		return fmt.Sprintf("%v", typed)
	}
}
