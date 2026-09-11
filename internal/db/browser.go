package db

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/lib/pq"
)

type TableBrowseRequest struct {
	Limit         int
	Offset        int
	SortColumn    string
	Descending    bool
	FilterColumn  string
	FilterValue   string
	ConnectionID  uint64
	Cursor        *TableCursor
	Timeout       time.Duration
	CellByteLimit int
	PageByteLimit int
}

type TableCursor struct {
	Values       []CellValue
	OrderColumns []string
	ConnectionID uint64
	Relation     Relation
	SortColumn   string
	Descending   bool
	FilterColumn string
	FilterValue  string
}

func (cursor *TableCursor) Clone() *TableCursor {
	if cursor == nil {
		return nil
	}
	cloned := *cursor
	cloned.Values = make([]CellValue, len(cursor.Values))
	for index := range cursor.Values {
		cloned.Values[index] = cursor.Values[index].Clone()
	}
	cloned.OrderColumns = slices.Clone(cursor.OrderColumns)
	return &cloned
}

type PaginationMode string

const (
	PaginationKeyset PaginationMode = "keyset"
	PaginationOffset PaginationMode = "offset"
)

type TableBrowsePage struct {
	Structure        []TableColumnInfo
	Columns          []table.Column
	Rows             []DataRow
	HasNext          bool
	NextCursor       *TableCursor
	KeysetEnabled    bool
	SortColumn       string
	PaginationMode   PaginationMode
	PaginationReason string
	StableOrdering   bool
	OrderColumns     []string
	RowKeys          []RowKey
}

type RowKey map[string]CellValue

// BrowseTable fetches one bounded, deterministically ordered page. It uses
// keyset pagination when every ordering value is non-null and safely falls
// back to OFFSET otherwise.
func BrowseTable(database *sql.DB, tableName string, request TableBrowseRequest) (TableBrowsePage, error) {
	return BrowseRelationContext(context.Background(), database, Relation{Schema: "public", Name: tableName}, request)
}

func BrowseTableContext(parent context.Context, database *sql.DB, tableName string, request TableBrowseRequest) (TableBrowsePage, error) {
	return BrowseRelationContext(parent, database, Relation{Schema: "public", Name: tableName}, request)
}

func BrowseRelation(database *sql.DB, relation Relation, request TableBrowseRequest) (TableBrowsePage, error) {
	return BrowseRelationContext(context.Background(), database, relation, request)
}

func BrowseRelationContext(parent context.Context, database *sql.DB, relation Relation, request TableBrowseRequest) (TableBrowsePage, error) {
	if err := relation.Validate(); err != nil {
		return TableBrowsePage{}, err
	}
	if request.Cursor != nil {
		preflight := request
		if preflight.SortColumn == "" {
			preflight.SortColumn = request.Cursor.SortColumn
		}
		if err := request.Cursor.validateRequestScope(relation, preflight); err != nil {
			return TableBrowsePage{}, err
		}
	}
	timeout := request.Timeout
	if timeout <= 0 {
		timeout = BrowseTimeout
	}
	if timeout > MaximumBrowseTimeout {
		timeout = MaximumBrowseTimeout
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	structure, err := GetRelationStructureContext(ctx, database, relation)
	if err != nil {
		return TableBrowsePage{}, err
	}
	if len(structure) == 0 {
		return TableBrowsePage{}, fmt.Errorf("table %q has no columns", relation.DisplayName())
	}

	columnsByName := make(map[string]TableColumnInfo, len(structure))
	primaryKeyColumns := make([]TableColumnInfo, 0, 1)
	for _, column := range structure {
		columnsByName[column.Name] = column
		if column.PrimaryKey {
			primaryKeyColumns = append(primaryKeyColumns, column)
		}
	}
	sort.Slice(primaryKeyColumns, func(i, j int) bool {
		return primaryKeyColumns[i].PrimaryKeyPosition < primaryKeyColumns[j].PrimaryKeyPosition
	})
	primaryKeys := make([]string, len(primaryKeyColumns))
	for index, column := range primaryKeyColumns {
		primaryKeys[index] = column.Name
	}
	if request.FilterColumn != "" {
		if _, ok := columnsByName[request.FilterColumn]; !ok {
			return TableBrowsePage{}, fmt.Errorf("unknown filter column %q", request.FilterColumn)
		}
	}
	if request.SortColumn == "" {
		if len(primaryKeys) > 0 {
			request.SortColumn = primaryKeys[0]
		} else {
			request.SortColumn = structure[0].Name
		}
	}
	if _, ok := columnsByName[request.SortColumn]; !ok {
		return TableBrowsePage{}, fmt.Errorf("unknown sort column %q", request.SortColumn)
	}
	if request.Limit <= 0 || request.Limit > MaximumPageSize {
		request.Limit = 100
	}
	if request.CellByteLimit <= 0 {
		request.CellByteLimit = DefaultCellByteLimit
	}
	if request.PageByteLimit <= 0 {
		request.PageByteLimit = DefaultPageByteLimit
	}
	if request.Offset < 0 {
		request.Offset = 0
	}

	orderColumns := []string{request.SortColumn}
	for _, primaryKey := range primaryKeys {
		if primaryKey != request.SortColumn {
			orderColumns = append(orderColumns, primaryKey)
		}
	}
	stableOrdering := len(primaryKeys) > 0
	keyset := stableOrdering
	for _, name := range orderColumns {
		if columnsByName[name].Nullable {
			keyset = false
			break
		}
	}
	paginationReason := "stable keyset pagination"
	if !keyset && stableOrdering {
		paginationReason = "offset pagination: the sort column can contain NULL; rows may shift while data changes"
	} else if !keyset {
		paginationReason = "offset pagination: this table has no primary key, so row order may not be unique"
	}
	if request.Cursor != nil {
		if !keyset {
			return TableBrowsePage{}, fmt.Errorf("cursor pagination is unavailable for the active sort")
		}
		if err := request.Cursor.validateScope(relation, request, orderColumns); err != nil {
			return TableBrowsePage{}, err
		}
	}
	arguments := make([]any, 0, 2)
	conditions := make([]string, 0, 2)
	if request.FilterColumn != "" && request.FilterValue != "" {
		arguments = append(arguments, request.FilterValue)
		conditions = append(conditions, fmt.Sprintf("%s = $%d", pq.QuoteIdentifier(request.FilterColumn), len(arguments)))
	}
	if keyset && request.Cursor != nil {
		operator := ">"
		if request.Descending {
			operator = "<"
		}
		placeholders := make([]string, len(orderColumns))
		quotedColumns := make([]string, len(orderColumns))
		for index, name := range orderColumns {
			arguments = append(arguments, request.Cursor.Values[index].Raw)
			placeholders[index] = fmt.Sprintf("$%d", len(arguments))
			quotedColumns[index] = pq.QuoteIdentifier(name)
		}
		conditions = append(conditions, fmt.Sprintf("(%s) %s (%s)", strings.Join(quotedColumns, ", "), operator, strings.Join(placeholders, ", ")))
	}

	var query strings.Builder
	query.WriteString("SELECT * FROM ")
	query.WriteString(relation.QualifiedName())
	if len(conditions) > 0 {
		query.WriteString(" WHERE ")
		query.WriteString(strings.Join(conditions, " AND "))
	}
	query.WriteString(" ORDER BY ")
	for index, name := range orderColumns {
		if index > 0 {
			query.WriteString(", ")
		}
		query.WriteString(pq.QuoteIdentifier(name))
		if request.Descending {
			query.WriteString(" DESC")
		} else {
			query.WriteString(" ASC")
		}
		query.WriteString(" NULLS LAST")
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
	columnTypes, err := rows.ColumnTypes()
	if err != nil {
		return TableBrowsePage{}, err
	}
	result := TableBrowsePage{
		Structure:        structure,
		Columns:          make([]table.Column, len(columnNames)),
		KeysetEnabled:    keyset,
		SortColumn:       request.SortColumn,
		StableOrdering:   stableOrdering,
		OrderColumns:     slices.Clone(orderColumns),
		PaginationReason: paginationReason,
	}
	if keyset {
		result.PaginationMode = PaginationKeyset
	} else {
		result.PaginationMode = PaginationOffset
	}
	columnIndexes := make(map[string]int, len(columnNames))
	for index, name := range columnNames {
		result.Columns[index] = table.Column{Title: name, Width: 20}
		columnIndexes[name] = index
	}
	pageBytes := 0
	var lastCursorValues []CellValue
	cursorSafe := true
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
		row := make(DataRow, len(values))
		fullRow := make(DataRow, len(values))
		effectiveCellLimit := request.CellByteLimit
		if len(values) > 0 {
			effectiveCellLimit = min(effectiveCellLimit, max(request.PageByteLimit/(len(values)+len(primaryKeys)), 1))
		}
		for index, value := range values {
			databaseTypeName := ""
			if index < len(columnTypes) {
				databaseTypeName = columnTypes[index].DatabaseTypeName()
			}
			fullRow[index] = newCellValue(value, databaseTypeName)
			row[index] = fullRow[index].bounded(effectiveCellLimit)
		}
		rowKey := make(RowKey, len(primaryKeys))
		rowKeySafe := true
		rowBytes := 0
		for _, cell := range row {
			rowBytes += cell.retainedBytes()
		}
		for _, name := range primaryKeys {
			value := fullRow[columnIndexes[name]]
			if value.retainedBytes() > request.CellByteLimit {
				rowKeySafe = false
				break
			}
			rowKey[name] = value.Clone()
			rowBytes += value.retainedBytes()
		}
		if len(result.Rows) > 0 && pageBytes+rowBytes > request.PageByteLimit {
			result.HasNext = true
			break
		}
		pageBytes += rowBytes
		result.Rows = append(result.Rows, row)
		if !rowKeySafe {
			rowKey = nil
		}
		result.RowKeys = append(result.RowKeys, rowKey)
		if keyset {
			lastCursorValues = make([]CellValue, len(orderColumns))
			for index, name := range orderColumns {
				lastCursorValues[index] = fullRow[columnIndexes[name]].Clone()
				if lastCursorValues[index].retainedBytes() > request.CellByteLimit {
					cursorSafe = false
				}
			}
		}
	}
	if err := rows.Err(); err != nil {
		return TableBrowsePage{}, err
	}
	if keyset && !cursorSafe {
		result.KeysetEnabled = false
		result.PaginationMode = PaginationOffset
		result.PaginationReason = "offset pagination: an ordering value exceeds the cursor byte limit; rows may shift while data changes"
	}
	if keyset && cursorSafe && len(result.Rows) > 0 {
		result.NextCursor = &TableCursor{
			Values:       lastCursorValues,
			OrderColumns: slices.Clone(orderColumns),
			ConnectionID: request.ConnectionID,
			Relation:     relation,
			SortColumn:   request.SortColumn,
			Descending:   request.Descending,
			FilterColumn: request.FilterColumn,
			FilterValue:  request.FilterValue,
		}
	}
	return result, nil
}

// FetchRelationRowContext retrieves one complete row after the user explicitly
// asks to inspect a truncated preview. It requires the table's full primary key.
func FetchRelationRowContext(parent context.Context, database *sql.DB, relation Relation, key RowKey, timeout time.Duration) (DataRow, error) {
	if err := relation.Validate(); err != nil {
		return nil, err
	}
	if timeout <= 0 {
		timeout = BrowseTimeout
	}
	if timeout > MaximumBrowseTimeout {
		timeout = MaximumBrowseTimeout
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	structure, err := GetRelationStructureContext(ctx, database, relation)
	if err != nil {
		return nil, err
	}
	primaryKeys := make([]TableColumnInfo, 0)
	for _, column := range structure {
		if column.PrimaryKey {
			primaryKeys = append(primaryKeys, column)
		}
	}
	sort.Slice(primaryKeys, func(i, j int) bool { return primaryKeys[i].PrimaryKeyPosition < primaryKeys[j].PrimaryKeyPosition })
	if len(primaryKeys) == 0 || len(key) != len(primaryKeys) {
		return nil, fmt.Errorf("full row retrieval requires a complete primary key")
	}
	conditions := make([]string, len(primaryKeys))
	arguments := make([]any, len(primaryKeys))
	for index, column := range primaryKeys {
		value, ok := key[column.Name]
		if !ok || value.IsNull || value.Raw == nil {
			return nil, fmt.Errorf("full row retrieval requires primary key column %q", column.Name)
		}
		conditions[index] = fmt.Sprintf("%s = $%d", pq.QuoteIdentifier(column.Name), index+1)
		arguments[index] = value.Raw
	}
	query := "SELECT * FROM " + relation.QualifiedName() + " WHERE " + strings.Join(conditions, " AND ") + " LIMIT 2"
	rows, err := database.QueryContext(ctx, query, arguments...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	columnTypes, err := rows.ColumnTypes()
	if err != nil {
		return nil, err
	}
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("row no longer exists")
	}
	values := make([]any, len(columnTypes))
	scanArguments := make([]any, len(values))
	for index := range values {
		scanArguments[index] = &values[index]
	}
	if err := rows.Scan(scanArguments...); err != nil {
		return nil, err
	}
	row := make(DataRow, len(values))
	for index, value := range values {
		row[index] = newCellValue(value, columnTypes[index].DatabaseTypeName())
	}
	return row, nil
}

func (cursor *TableCursor) validateRequestScope(relation Relation, request TableBrowseRequest) error {
	if cursor.ConnectionID != request.ConnectionID || cursor.Relation != relation ||
		cursor.SortColumn != request.SortColumn || cursor.Descending != request.Descending ||
		cursor.FilterColumn != request.FilterColumn || cursor.FilterValue != request.FilterValue {
		return fmt.Errorf("cursor does not belong to the active connection, relation, sort, and filter")
	}
	return nil
}

func (cursor *TableCursor) validateScope(relation Relation, request TableBrowseRequest, orderColumns []string) error {
	if err := cursor.validateRequestScope(relation, request); err != nil {
		return err
	}
	if !slices.Equal(cursor.OrderColumns, orderColumns) || len(cursor.Values) != len(orderColumns) {
		return fmt.Errorf("cursor does not match the active ordering")
	}
	for _, value := range cursor.Values {
		if value.IsNull {
			return fmt.Errorf("cursor value cannot be SQL NULL")
		}
		if value.Raw == nil {
			return fmt.Errorf("cursor raw value is missing")
		}
	}
	return nil
}
